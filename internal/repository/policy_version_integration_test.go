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

func TestPolicyVersionRepository(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{
		Name: "pv-test-" + uuid.New().String()[:8],
		Slug: "pv-test-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { entRepo.Delete(context.Background(), enterprise.ID) })

	policyRepo, err := repository.NewPolicyRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	policy := &models.Policy{
		EnterpriseID: enterprise.ID,
		Name:         "PV Policy",
		Platform:     models.PlatformMacOS,
		PolicyType:   "wifi",
		PolicyConfig: models.JSONB{"ssid": "Corp"},
		IsActive:     true,
	}
	require.NoError(t, policyRepo.Create(context.Background(), policy))
	t.Cleanup(func() { policyRepo.Delete(context.Background(), policy.ID) })

	repo, err := repository.NewPolicyVersionRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("create version", func(t *testing.T) {
		v := &models.PolicyVersion{
			PolicyID:     policy.ID,
			Version:      1,
			PolicyConfig: models.JSONB{"ssid": "Corp", "security": "WPA2"},
			Name:         "PV Policy",
			Description:  "Initial version",
			Platform:     models.PlatformMacOS,
			PolicyType:   "wifi",
			IsActive:     true,
			CreatedBy:    "admin@test.com",
		}
		err := repo.Create(ctx, v)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, v.ID)
	})

	t.Run("create second version", func(t *testing.T) {
		v := &models.PolicyVersion{
			PolicyID:     policy.ID,
			Version:      2,
			PolicyConfig: models.JSONB{"ssid": "Corp-5G", "security": "WPA3"},
			Name:         "PV Policy Updated",
			Description:  "Changed SSID",
			Platform:     models.PlatformMacOS,
			PolicyType:   "wifi",
			IsActive:     true,
			CreatedBy:    "admin@test.com",
		}
		require.NoError(t, repo.Create(ctx, v))
	})

	t.Run("list by policy returns versions newest first", func(t *testing.T) {
		versions, total, err := repo.ListByPolicy(ctx, policy.ID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Equal(t, 2, len(versions))
		assert.Equal(t, 2, versions[0].Version) // newest first (ORDER BY version DESC)
		assert.Equal(t, 1, versions[1].Version)
	})

	t.Run("get by version", func(t *testing.T) {
		v, err := repo.GetByVersion(ctx, policy.ID, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, v.Version)
		assert.Equal(t, "PV Policy", v.Name)
		assert.Equal(t, "Initial version", v.Description)
	})

	t.Run("get by version not found", func(t *testing.T) {
		_, err := repo.GetByVersion(ctx, policy.ID, 999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("latest version", func(t *testing.T) {
		latest, err := repo.LatestVersion(ctx, policy.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, latest)
	})

	t.Run("latest version for policy with no versions", func(t *testing.T) {
		latest, err := repo.LatestVersion(ctx, uuid.New())
		require.NoError(t, err)
		assert.Equal(t, 0, latest) // COALESCE(MAX(version), 0)
	})

	t.Run("JSONB round-trips correctly", func(t *testing.T) {
		config := models.JSONB{
			"ssid":     "TestNet",
			"security": "WPA3",
			"nested":   map[string]interface{}{"key": "value"},
		}
		v := &models.PolicyVersion{
			PolicyID:     policy.ID,
			Version:      3,
			PolicyConfig: config,
			Name:         "JSONB Test",
			Platform:     models.PlatformMacOS,
			PolicyType:   "wifi",
			IsActive:     true,
			CreatedBy:    "test",
		}
		require.NoError(t, repo.Create(ctx, v))

		fetched, err := repo.GetByVersion(ctx, policy.ID, 3)
		require.NoError(t, err)
		assert.Equal(t, "TestNet", fetched.PolicyConfig["ssid"])
		nested, ok := fetched.PolicyConfig["nested"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", nested["key"])
	})

	t.Run("list with pagination", func(t *testing.T) {
		versions, total, err := repo.ListByPolicy(ctx, policy.ID, 2, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, len(versions))
		assert.Equal(t, 3, total)
	})
}

func TestPolicyVersionRepository_Constructor(t *testing.T) {
	t.Run("nil returns error", func(t *testing.T) {
		_, err := repository.NewPolicyVersionRepository(nil, nil)
		assert.Error(t, err)
	})
	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewPolicyVersionRepository("bad", "bad")
		assert.Error(t, err)
	})
}
