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

func TestAppRepository(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	enterprise := &models.Enterprise{
		Name: "app-test-ent-" + uuid.New().String()[:8],
		Slug: "app-test-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { entRepo.Delete(context.Background(), enterprise.ID) })

	repo, err := repository.NewAppRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("create and get by ID", func(t *testing.T) {
		app := &models.App{
			EnterpriseID: enterprise.ID,
			Name:         "Test App",
			Platform:     models.PlatformWindows,
			Identifier:   "com.test.app-" + uuid.New().String()[:8],
			Version:      "1.0",
			InstallType:  models.AppInstallRequired,
			AppConfig:    models.JSONB{"key": "value"},
		}
		err := repo.Create(ctx, app)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, app.ID)
		assert.False(t, app.CreatedAt.IsZero())

		fetched, err := repo.GetByID(ctx, app.ID)
		require.NoError(t, err)
		assert.Equal(t, app.Name, fetched.Name)
		assert.Equal(t, app.Platform, fetched.Platform)
		assert.Equal(t, app.Identifier, fetched.Identifier)
		assert.Equal(t, "value", fetched.AppConfig["key"])
	})

	t.Run("list with pagination", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			require.NoError(t, repo.Create(ctx, &models.App{
				EnterpriseID: enterprise.ID,
				Name:         "List App " + uuid.New().String()[:4],
				Platform:     models.PlatformMacOS,
				Identifier:   "com.list." + uuid.New().String()[:8],
				InstallType:  models.AppInstallAvailable,
				AppConfig:    models.JSONB{},
			}))
		}

		apps, total, err := repo.List(ctx, enterprise.ID, 2, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, len(apps))
		assert.GreaterOrEqual(t, total, 3)
	})

	t.Run("update", func(t *testing.T) {
		app := &models.App{
			EnterpriseID: enterprise.ID,
			Name:         "Update Me",
			Platform:     models.PlatformAndroid,
			Identifier:   "com.update." + uuid.New().String()[:8],
			Version:      "1.0",
			InstallType:  models.AppInstallRequired,
			AppConfig:    models.JSONB{},
		}
		require.NoError(t, repo.Create(ctx, app))

		app.Name = "Updated Name"
		app.Version = "2.0"
		err := repo.Update(ctx, app)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, app.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", fetched.Name)
		assert.Equal(t, "2.0", fetched.Version)
	})

	t.Run("delete soft-deletes", func(t *testing.T) {
		app := &models.App{
			EnterpriseID: enterprise.ID,
			Name:         "Delete Me",
			Platform:     models.PlatformWindows,
			Identifier:   "com.delete." + uuid.New().String()[:8],
			InstallType:  models.AppInstallRequired,
			AppConfig:    models.JSONB{},
		}
		require.NoError(t, repo.Create(ctx, app))

		err := repo.Delete(ctx, app.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, app.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("get nonexistent returns error", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("update nonexistent returns error", func(t *testing.T) {
		err := repo.Update(ctx, &models.App{BaseModel: models.BaseModel{ID: uuid.New()}})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("delete nonexistent returns error", func(t *testing.T) {
		err := repo.Delete(ctx, uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestAppRepository_Constructor(t *testing.T) {
	t.Run("nil returns error", func(t *testing.T) {
		_, err := repository.NewAppRepository(nil, nil)
		assert.Error(t, err)
	})
	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewAppRepository("bad", "bad")
		assert.Error(t, err)
	})
}
