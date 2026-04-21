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

func TestCommandRepository(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	repo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	// Create a device to reference
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	enterprise := &models.Enterprise{
		Name: "cmd-test-ent-" + uuid.New().String()[:8],
		Slug: "cmd-test-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { entRepo.Delete(context.Background(), enterprise.ID) })

	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformWindows,
		DeviceID:     "cmd-test-" + uuid.New().String()[:8],
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(context.Background(), device))
	t.Cleanup(func() { deviceRepo.Delete(context.Background(), device.ID) })

	ctx := context.Background()

	t.Run("create and list pending", func(t *testing.T) {
		cmd := &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: models.CommandTypeLock,
			CommandData: models.JSONB{"reason": "test"},
		}
		err := repo.Create(ctx, cmd)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, cmd.ID)
		assert.Equal(t, models.CommandStatusPending, cmd.Status)

		pending, err := repo.ListPending(ctx, device.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(pending), 1)

		found := false
		for _, p := range pending {
			if p.ID == cmd.ID {
				found = true
				assert.Equal(t, models.CommandTypeLock, p.CommandType)
			}
		}
		assert.True(t, found, "created command should be in pending list")
	})

	t.Run("mark sent", func(t *testing.T) {
		cmd := &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: models.CommandTypeDeviceInfo,
		}
		require.NoError(t, repo.Create(ctx, cmd))

		err := repo.MarkSent(ctx, cmd.ID)
		require.NoError(t, err)

		// Should no longer appear in pending
		pending, err := repo.ListPending(ctx, device.ID)
		require.NoError(t, err)
		for _, p := range pending {
			assert.NotEqual(t, cmd.ID, p.ID, "sent command should not be in pending list")
		}
	})

	t.Run("mark completed", func(t *testing.T) {
		cmd := &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: models.CommandTypeWipe,
		}
		require.NoError(t, repo.Create(ctx, cmd))

		err := repo.MarkCompleted(ctx, cmd.ID)
		require.NoError(t, err)
	})

	t.Run("mark failed", func(t *testing.T) {
		cmd := &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: models.CommandTypeLock,
		}
		require.NoError(t, repo.Create(ctx, cmd))

		err := repo.MarkFailed(ctx, cmd.ID, "device unreachable")
		require.NoError(t, err)
	})

	t.Run("mark nonexistent command fails", func(t *testing.T) {
		err := repo.MarkSent(ctx, uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestCommandRepository_Constructor(t *testing.T) {
	t.Run("nil db returns error", func(t *testing.T) {
		_, err := repository.NewCommandRepository(nil, nil)
		assert.Error(t, err)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewCommandRepository("not a db", "not a db")
		assert.Error(t, err)
	})
}

func TestCertificateRepository_Constructor(t *testing.T) {
	t.Run("nil db returns error", func(t *testing.T) {
		_, err := repository.NewCertificateRepository(nil, nil)
		assert.Error(t, err)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewCertificateRepository("not a db", "not a db")
		assert.Error(t, err)
	})
}

func TestAuditLogRepository_Constructor(t *testing.T) {
	t.Run("nil db returns error", func(t *testing.T) {
		_, err := repository.NewAuditLogRepository(nil, nil)
		assert.Error(t, err)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewAuditLogRepository("not a db", "not a db")
		assert.Error(t, err)
	})
}

func TestCertificateRepository_List(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	repo, err := repository.NewCertificateRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	t.Run("empty list returns no error", func(t *testing.T) {
		certs, total, err := repo.List(context.Background(), nil, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 0)
		assert.NotNil(t, certs)
	})

	t.Run("filter by nonexistent device returns empty", func(t *testing.T) {
		fakeID := uuid.New()
		certs, total, err := repo.List(context.Background(), &fakeID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, certs)
	})
}

func TestAuditLogRepository_List(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	repo, err := repository.NewAuditLogRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	t.Run("empty list returns no error", func(t *testing.T) {
		logs, total, err := repo.List(context.Background(), uuid.New(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.NotNil(t, logs)
	})
}
