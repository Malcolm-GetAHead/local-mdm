package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceRepository_GetByPlatformID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{
		Name: "plat-test-" + uuid.New().String()[:8],
		Slug: "plat-test-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { entRepo.Delete(context.Background(), enterprise.ID) })

	repo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	platformDeviceID := "WIN-PLAT-" + uuid.New().String()[:8]
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformWindows,
		DeviceID:     platformDeviceID,
		SerialNumber: "SN" + uuid.New().String()[:8],
		Name:         "Platform Test Device",
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, repo.Create(ctx, device))
	t.Cleanup(func() { repo.Delete(ctx, device.ID) })

	t.Run("returns device by platform and device_id", func(t *testing.T) {
		fetched, err := repo.GetByPlatformID(ctx, models.PlatformWindows, platformDeviceID)
		require.NoError(t, err)
		assert.Equal(t, device.ID, fetched.ID)
		assert.Equal(t, "Platform Test Device", fetched.Name)
	})

	t.Run("not found returns error", func(t *testing.T) {
		_, err := repo.GetByPlatformID(ctx, models.PlatformWindows, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("wrong platform returns not found", func(t *testing.T) {
		_, err := repo.GetByPlatformID(ctx, models.PlatformMacOS, platformDeviceID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestCommandRepository_GetByID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{
		Name: "cmd-getid-" + uuid.New().String()[:8],
		Slug: "cmd-getid-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { entRepo.Delete(context.Background(), enterprise.ID) })

	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformWindows,
		DeviceID:     "cmd-getid-" + uuid.New().String()[:8],
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(context.Background(), device))
	t.Cleanup(func() { deviceRepo.Delete(context.Background(), device.ID) })

	repo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	// Fixed: GetByID now uses COALESCE(error_message, '') to handle NULL.
	t.Run("pending command with NULL error_message succeeds", func(t *testing.T) {
		cmd := &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: models.CommandTypeLock,
			CommandData: models.JSONB{"reason": "test"},
		}
		require.NoError(t, repo.Create(ctx, cmd))

		fetched, err := repo.GetByID(ctx, cmd.ID)
		require.NoError(t, err)
		assert.Equal(t, models.CommandStatusPending, fetched.Status)
		assert.Equal(t, "", fetched.ErrorMessage)
	})

	t.Run("returns failed command with error message", func(t *testing.T) {
		cmd := &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: models.CommandTypeWipe,
		}
		require.NoError(t, repo.Create(ctx, cmd))
		require.NoError(t, repo.MarkFailed(ctx, cmd.ID, "device offline"))

		fetched, err := repo.GetByID(ctx, cmd.ID)
		require.NoError(t, err)
		assert.Equal(t, models.CommandStatusFailed, fetched.Status)
		assert.Equal(t, "device offline", fetched.ErrorMessage)
	})

	t.Run("not found returns error", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestCommandRepository_ListByDevice(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{
		Name: "cmd-list-" + uuid.New().String()[:8],
		Slug: "cmd-list-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { entRepo.Delete(context.Background(), enterprise.ID) })

	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformMacOS,
		DeviceID:     "cmd-list-" + uuid.New().String()[:8],
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(context.Background(), device))
	t.Cleanup(func() { deviceRepo.Delete(context.Background(), device.ID) })

	repo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	// Create several commands
	for _, ct := range []string{models.CommandTypeLock, models.CommandTypeRestart, models.CommandTypeDeviceInfo} {
		require.NoError(t, repo.Create(ctx, &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: ct,
		}))
	}

	// Fixed: ListByDevice now uses COALESCE(error_message, '') to handle NULL.
	t.Run("lists commands with NULL error_message", func(t *testing.T) {
		cmds, total, err := repo.ListByDevice(ctx, device.ID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, cmds, 3)
	})

	t.Run("empty device returns zero", func(t *testing.T) {
		cmds, total, err := repo.ListByDevice(ctx, uuid.New(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, cmds)
	})
}

func TestCertificateRepository_GetBySerial(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	repo, err := repository.NewCertificateRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	t.Run("not found returns error", func(t *testing.T) {
		_, err := repo.GetBySerial(context.Background(), "NONEXISTENT-SERIAL")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestAuditLogRepository_ListEdgeCases(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	repo, err := repository.NewAuditLogRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	t.Run("nonexistent enterprise returns empty", func(t *testing.T) {
		logs, total, err := repo.List(context.Background(), uuid.New(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, logs)
	})

	t.Run("zero limit uses default", func(t *testing.T) {
		// ValidatePagination should handle 0 limit gracefully
		logs, total, err := repo.List(context.Background(), uuid.New(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, logs)
	})
}
