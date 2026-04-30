package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceRepository_List_PaginationValidation(t *testing.T) {
	db := testutil.ConnectDB(t)
	testutil.EnsureTestEnterprise(t, db.Writer)
	t.Cleanup(func() { testutil.CleanupTestData(t, db.Writer) })
	enterpriseID := testutil.TestEnterpriseID

	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("excessive limit rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterpriseID, 10000, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("negative offset rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterpriseID, 100, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be non-negative")
	})

	t.Run("zero limit defaults to 100", func(t *testing.T) {
		devices, _, err := repo.List(ctx, enterpriseID, 0, 0)
		require.NoError(t, err)
		assert.NotNil(t, devices)
	})

	t.Run("maximum limit allowed", func(t *testing.T) {
		devices, _, err := repo.List(ctx, enterpriseID, MaxPageSize, 0)
		require.NoError(t, err)
		assert.NotNil(t, devices)
	})
}

func TestEnterpriseRepository_List_PaginationValidation(t *testing.T) {
	db := testutil.ConnectDB(t)

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
	testutil.EnsureTestEnterprise(t, db.Writer)
	t.Cleanup(func() { testutil.CleanupTestData(t, db.Writer) })
	enterpriseID := testutil.TestEnterpriseID

	repo, err := NewPolicyRepository(db.Writer, db.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("excessive limit rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterpriseID, 10000, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("negative offset rejected", func(t *testing.T) {
		_, _, err := repo.List(ctx, enterpriseID, 100, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be non-negative")
	})

	t.Run("zero limit defaults to 100", func(t *testing.T) {
		policies, _, err := repo.List(ctx, enterpriseID, 0, 0)
		require.NoError(t, err)
		assert.NotNil(t, policies)
	})
}

func TestPaginationValidation_DoSPrevention(t *testing.T) {
	db := testutil.ConnectDB(t)
	testutil.EnsureTestEnterprise(t, db.Writer)
	t.Cleanup(func() { testutil.CleanupTestData(t, db.Writer) })
	enterpriseID := testutil.TestEnterpriseID

	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	require.NoError(t, err)
	ctx := context.Background()

	// Create some test devices
	for i := 0; i < 10; i++ {
		device := &models.Device{
			EnterpriseID: enterpriseID,
			SerialNumber: uuid.New().String(),
			DeviceID:     uuid.New().String(),
			Platform:     "windows",
			PlatformData: models.JSONB{},
		}
		err := deviceRepo.Create(ctx, device)
		require.NoError(t, err)
	}

	t.Run("attacker cannot request millions of records", func(t *testing.T) {
		// Attempt to request 1 million records
		_, _, err := deviceRepo.List(ctx, enterpriseID, 1000000, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("legitimate large request within limits works", func(t *testing.T) {
		devices, total, err := deviceRepo.List(ctx, enterpriseID, MaxPageSize, 0)
		require.NoError(t, err)
		assert.Equal(t, 10, total)
		assert.Len(t, devices, 10)
	})

	t.Run("pagination works correctly with limits", func(t *testing.T) {
		// Get first 5
		devices1, total, err := deviceRepo.List(ctx, enterpriseID, 5, 0)
		require.NoError(t, err)
		assert.Equal(t, 10, total)
		assert.Len(t, devices1, 5)

		// Get next 5
		devices2, total, err := deviceRepo.List(ctx, enterpriseID, 5, 5)
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
