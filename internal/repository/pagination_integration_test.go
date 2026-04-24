package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceRepository_List_PaginationValidation(t *testing.T) {
	db := testutil.ConnectDB(t)
	defer db.Close()

	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	ctx := context.Background()

	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: fmt.Sprintf("test-enterprise-%d", time.Now().UnixNano()),
	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	err = enterpriseRepo.Create(ctx, enterprise)
	require.NoError(t, err)
	t.Cleanup(func() { enterpriseRepo.Delete(ctx, enterprise.ID) })

	t.Run("excessive limit rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterprise.ID, 10000, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("negative offset rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterprise.ID, 100, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be non-negative")
	})

	t.Run("zero limit defaults to 100", func(t *testing.T) {
		devices, total, err := repo.List(ctx, enterprise.ID, 0, 0)
		require.NoError(t, err)
		assert.NotNil(t, devices)
		assert.Equal(t, 0, total) // No devices created yet
	})

	t.Run("maximum limit allowed", func(t *testing.T) {
		devices, total, err := repo.List(ctx, enterprise.ID, MaxPageSize, 0)
		require.NoError(t, err)
		assert.NotNil(t, devices)
		assert.Equal(t, 0, total)
	})
}

func TestEnterpriseRepository_List_PaginationValidation(t *testing.T) {
	db := testutil.ConnectDB(t)
	defer db.Close()

	repo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("excessive limit rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, 10000, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("negative offset rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, 100, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be non-negative")
	})

	t.Run("zero limit defaults to 100", func(t *testing.T) {
		enterprises, total, err := repo.List(ctx, 0, 0)
		require.NoError(t, err)
		assert.NotNil(t, enterprises)
		assert.GreaterOrEqual(t, total, 0)
	})
}

func TestPolicyRepository_List_PaginationValidation(t *testing.T) {
	db := testutil.ConnectDB(t)
	defer db.Close()

	repo, err := NewPolicyRepository(db.Writer, db.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	enterprise := &models.Enterprise{
		Name: "Test Enterprise Policy",
		Slug: fmt.Sprintf("test-enterprise-policy-%d", time.Now().UnixNano()),
	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	err = enterpriseRepo.Create(ctx, enterprise)
	require.NoError(t, err)
	t.Cleanup(func() { enterpriseRepo.Delete(ctx, enterprise.ID) })

	t.Run("excessive limit rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterprise.ID, 10000, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("negative offset rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterprise.ID, 100, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be non-negative")
	})

	t.Run("zero limit defaults to 100", func(t *testing.T) {
		policies, total, err := repo.List(ctx, enterprise.ID, 0, 0)
		require.NoError(t, err)
		assert.NotNil(t, policies)
		assert.Equal(t, 0, total)
	})
}

func TestPaginationValidation_DoSPrevention(t *testing.T) {
	db := testutil.ConnectDB(t)
	defer db.Close()

	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	ctx := context.Background()

	// Create test enterprise
	enterprise := &models.Enterprise{
		Name: "Test Enterprise DoS",
		Slug: fmt.Sprintf("test-enterprise-dos-%d", time.Now().UnixNano()),
	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	err = enterpriseRepo.Create(ctx, enterprise)
	require.NoError(t, err)
	t.Cleanup(func() { enterpriseRepo.Delete(ctx, enterprise.ID) })

	// Create some test devices
	var deviceIDs []uuid.UUID
	for i := 0; i < 10; i++ {
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			SerialNumber: uuid.New().String(),
			DeviceID:     uuid.New().String(),
			Platform:     "windows",
			PlatformData: models.JSONB{},
		}
		err := deviceRepo.Create(ctx, device)
		require.NoError(t, err)
		deviceIDs = append(deviceIDs, device.ID)
	}
	t.Cleanup(func() {
		for _, id := range deviceIDs {
			deviceRepo.Delete(ctx, id)
		}
	})

	t.Run("attacker cannot request millions of records", func(t *testing.T) {
		// Attempt to request 1 million records
		_, _, err := deviceRepo.List(ctx, enterprise.ID, 1000000, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("legitimate large request within limits works", func(t *testing.T) {
		devices, total, err := deviceRepo.List(ctx, enterprise.ID, MaxPageSize, 0)
		require.NoError(t, err)
		assert.Equal(t, 10, total)
		assert.Len(t, devices, 10)
	})

	t.Run("pagination works correctly with limits", func(t *testing.T) {
		// Get first 5
		devices1, total, err := deviceRepo.List(ctx, enterprise.ID, 5, 0)
		require.NoError(t, err)
		assert.Equal(t, 10, total)
		assert.Len(t, devices1, 5)

		// Get next 5
		devices2, total, err := deviceRepo.List(ctx, enterprise.ID, 5, 5)
		require.NoError(t, err)
		assert.Equal(t, 10, total)
		assert.Len(t, devices2, 5)

		// Verify no overlap
		ids1 := make(map[uuid.UUID]bool)
		for _, d := range devices1 {
			ids1[d.ID] = true
		}
		for _, d := range devices2 {
			assert.False(t, ids1[d.ID], "devices should not overlap")
		}
	})
}
