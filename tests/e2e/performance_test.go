package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestPerformance_DeviceListLatency(t *testing.T) {
	database := setupDB(t)
	defer database.Close()
	ctx := context.Background()

	entRepo, _ := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	deviceRepo, _ := repository.NewDeviceRepository(database.Writer, database.Reader)

	enterprise := &models.Enterprise{Name: "perf-" + uuid.New().String()[:8], Slug: "perf-" + uuid.New().String()[:8]}
	require.NoError(t, entRepo.Create(ctx, enterprise))
	t.Cleanup(func() { entRepo.Delete(ctx, enterprise.ID) })

	// Create 100 devices
	for i := 0; i < 100; i++ {
		d := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     []string{models.PlatformWindows, models.PlatformMacOS, models.PlatformAndroid}[i%3],
			DeviceID:     fmt.Sprintf("PERF-%d-%s", i, uuid.New().String()[:8]),
			Name:         fmt.Sprintf("Perf Device %d", i),
			Status:       models.DeviceStatusEnrolled,
			PlatformData: models.JSONB{},
		}
		require.NoError(t, deviceRepo.Create(ctx, d))
		t.Cleanup(func() { deviceRepo.Delete(ctx, d.ID) })
	}

	// Measure list latency
	start := time.Now()
	devices, total, err := deviceRepo.List(ctx, enterprise.ID, 50, 0)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, 100, total)
	require.Len(t, devices, 50)

	t.Logf("Device list (50 of 100): %v", elapsed)
	if elapsed > 200*time.Millisecond {
		t.Errorf("Device list p95 target exceeded: %v > 200ms", elapsed)
	}
}

func TestPerformance_ConcurrentEnrollments(t *testing.T) {
	database := setupDB(t)
	defer database.Close()
	ctx := context.Background()

	entRepo, _ := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	deviceRepo, _ := repository.NewDeviceRepository(database.Writer, database.Reader)

	enterprise := &models.Enterprise{Name: "conc-" + uuid.New().String()[:8], Slug: "conc-" + uuid.New().String()[:8]}
	require.NoError(t, entRepo.Create(ctx, enterprise))
	t.Cleanup(func() { entRepo.Delete(ctx, enterprise.ID) })

	// Simulate 50 concurrent enrollments
	var wg sync.WaitGroup
	errors := make(chan error, 50)
	start := time.Now()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			d := &models.Device{
				EnterpriseID: enterprise.ID,
				Platform:     models.PlatformWindows,
				DeviceID:     fmt.Sprintf("CONC-%d-%s", idx, uuid.New().String()[:8]),
				Status:       models.DeviceStatusEnrolled,
				PlatformData: models.JSONB{},
			}
			if err := deviceRepo.Create(ctx, d); err != nil {
				errors <- err
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	elapsed := time.Since(start)

	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "concurrent enrollments should not fail")

	t.Logf("50 concurrent enrollments: %v", elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("Concurrent enrollment target exceeded: %v > 5s", elapsed)
	}

	// Verify all created
	_, total, err := deviceRepo.List(ctx, enterprise.ID, 1, 0)
	require.NoError(t, err)
	require.Equal(t, 50, total)

	// Cleanup
	devices, _, _ := deviceRepo.List(ctx, enterprise.ID, 100, 0)
	for _, d := range devices {
		deviceRepo.Delete(ctx, d.ID)
	}
}
