package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupRepository(t *testing.T) {
	database := testutil.ConnectDB(t)

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{
		Name: "grp-test-" + uuid.New().String()[:8],
		Slug: "grp-test-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformMacOS,
		DeviceID:     "grp-dev-" + uuid.New().String()[:8],
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(context.Background(), device))
	t.Cleanup(func() { deviceRepo.Delete(context.Background(), device.ID) })

	repo, err := repository.NewGroupRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("create and get by ID", func(t *testing.T) {
		group := &models.DeviceGroup{
			EnterpriseID: enterprise.ID,
			Name:         "Engineering",
			Description:  "Engineering team devices",
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, group.ID)
		assert.False(t, group.CreatedAt.IsZero())

		fetched, err := repo.GetByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, "Engineering", fetched.Name)
		assert.Equal(t, "Engineering team devices", fetched.Description)
	})

	t.Run("list with pagination", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			require.NoError(t, repo.Create(ctx, &models.DeviceGroup{
				EnterpriseID: enterprise.ID,
				Name:         "List Group " + uuid.New().String()[:4],
			}))
		}

		groups, total, err := repo.List(ctx, enterprise.ID, 2, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, len(groups))
		assert.GreaterOrEqual(t, total, 3)
	})

	t.Run("update", func(t *testing.T) {
		group := &models.DeviceGroup{
			EnterpriseID: enterprise.ID,
			Name:         "Old Name",
		}
		require.NoError(t, repo.Create(ctx, group))

		group.Name = "New Name"
		group.Description = "Updated desc"
		err := repo.Update(ctx, group)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, "New Name", fetched.Name)
		assert.Equal(t, "Updated desc", fetched.Description)
	})

	t.Run("delete soft-deletes", func(t *testing.T) {
		group := &models.DeviceGroup{
			EnterpriseID: enterprise.ID,
			Name:         "Delete Me",
		}
		require.NoError(t, repo.Create(ctx, group))

		err := repo.Delete(ctx, group.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, group.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("add and list members", func(t *testing.T) {
		group := &models.DeviceGroup{
			EnterpriseID: enterprise.ID,
			Name:         "Members Group",
		}
		require.NoError(t, repo.Create(ctx, group))

		err := repo.AddMember(ctx, group.ID, device.ID)
		require.NoError(t, err)

		members, total, err := repo.ListMembers(ctx, group.ID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, 1, len(members))
		assert.Equal(t, device.ID, members[0].ID)
	})

	t.Run("add member duplicate is idempotent", func(t *testing.T) {
		group := &models.DeviceGroup{
			EnterpriseID: enterprise.ID,
			Name:         "Dup Group",
		}
		require.NoError(t, repo.Create(ctx, group))
		require.NoError(t, repo.AddMember(ctx, group.ID, device.ID))

		// Second add should not error (ON CONFLICT DO NOTHING)
		err := repo.AddMember(ctx, group.ID, device.ID)
		assert.NoError(t, err)

		_, total, err := repo.ListMembers(ctx, group.ID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total) // still just one
	})

	t.Run("remove member", func(t *testing.T) {
		group := &models.DeviceGroup{
			EnterpriseID: enterprise.ID,
			Name:         "Remove Group",
		}
		require.NoError(t, repo.Create(ctx, group))
		require.NoError(t, repo.AddMember(ctx, group.ID, device.ID))

		err := repo.RemoveMember(ctx, group.ID, device.ID)
		require.NoError(t, err)

		members, total, err := repo.ListMembers(ctx, group.ID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, members)
	})

	t.Run("list groups for device", func(t *testing.T) {
		g1 := &models.DeviceGroup{EnterpriseID: enterprise.ID, Name: "DevGroup A"}
		g2 := &models.DeviceGroup{EnterpriseID: enterprise.ID, Name: "DevGroup B"}
		require.NoError(t, repo.Create(ctx, g1))
		require.NoError(t, repo.Create(ctx, g2))
		require.NoError(t, repo.AddMember(ctx, g1.ID, device.ID))
		require.NoError(t, repo.AddMember(ctx, g2.ID, device.ID))

		groups, err := repo.ListGroupsForDevice(ctx, device.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 2)
	})

	t.Run("get nonexistent returns error", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("update nonexistent returns error", func(t *testing.T) {
		err := repo.Update(ctx, &models.DeviceGroup{BaseModel: models.BaseModel{ID: uuid.New()}})
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("delete nonexistent returns error", func(t *testing.T) {
		err := repo.Delete(ctx, uuid.New())
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})
}

func TestGroupRepository_Constructor(t *testing.T) {
	t.Run("nil returns error", func(t *testing.T) {
		_, err := repository.NewGroupRepository(nil, nil)
		assert.Error(t, err)
	})
	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewGroupRepository("bad", "bad")
		assert.Error(t, err)
	})
}
