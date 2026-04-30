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

func TestUserRepository(t *testing.T) {
	database := testutil.ConnectDB(t)

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{Name: "user-test-" + uuid.New().String()[:8], Slug: "user-test-" + uuid.New().String()[:8]}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	repo, err := repository.NewUserRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("create and get by ID", func(t *testing.T) {
		user := &models.User{
			EnterpriseID: enterprise.ID,
			Email:        "test-" + uuid.New().String()[:8] + "@example.com",
			PasswordHash: "oidc-managed",
			Role:         models.RoleAdmin,
			IsActive:     true,
		}
		require.NoError(t, repo.Create(ctx, user))
		assert.NotEqual(t, uuid.Nil, user.ID)

		fetched, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Email, fetched.Email)
		assert.Equal(t, models.RoleAdmin, fetched.Role)
		assert.True(t, fetched.IsActive)
	})

	t.Run("get by email", func(t *testing.T) {
		email := "byemail-" + uuid.New().String()[:8] + "@example.com"
		user := &models.User{EnterpriseID: enterprise.ID, Email: email, PasswordHash: "x", Role: models.RoleViewer, IsActive: true}
		require.NoError(t, repo.Create(ctx, user))

		fetched, err := repo.GetByEmail(ctx, enterprise.ID, email)
		require.NoError(t, err)
		assert.Equal(t, email, fetched.Email)
	})

	t.Run("list by enterprise", func(t *testing.T) {
		users, total, err := repo.List(ctx, enterprise.ID, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(users), 2)
	})

	t.Run("update role", func(t *testing.T) {
		user := &models.User{EnterpriseID: enterprise.ID, Email: "upd-" + uuid.New().String()[:8] + "@example.com", PasswordHash: "x", Role: models.RoleViewer, IsActive: true}
		require.NoError(t, repo.Create(ctx, user))

		user.Role = models.RoleOperator
		require.NoError(t, repo.Update(ctx, user))

		fetched, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, models.RoleOperator, fetched.Role)
	})

	t.Run("deactivate", func(t *testing.T) {
		user := &models.User{EnterpriseID: enterprise.ID, Email: "deact-" + uuid.New().String()[:8] + "@example.com", PasswordHash: "x", Role: models.RoleViewer, IsActive: true}
		require.NoError(t, repo.Create(ctx, user))

		require.NoError(t, repo.Deactivate(ctx, user.ID))

		_, err := repo.GetByID(ctx, user.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestTokenRepository(t *testing.T) {
	database := testutil.ConnectDB(t)

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{Name: "token-test-" + uuid.New().String()[:8], Slug: "token-test-" + uuid.New().String()[:8]}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	userRepo, err := repository.NewUserRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	user := &models.User{EnterpriseID: enterprise.ID, Email: "tokenuser-" + uuid.New().String()[:8] + "@example.com", PasswordHash: "x", Role: models.RoleAdmin, IsActive: true}
	require.NoError(t, userRepo.Create(context.Background(), user))

	repo, err := repository.NewTokenRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("create and get by hash", func(t *testing.T) {
		token := &models.APIToken{
			UserID:    user.ID,
			Name:      "test-token",
			TokenHash: "hash-" + uuid.New().String(),
		}
		require.NoError(t, repo.Create(ctx, token))
		assert.NotEqual(t, uuid.Nil, token.ID)

		fetched, err := repo.GetByHash(ctx, token.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, token.Name, fetched.Name)
		assert.Equal(t, user.ID, fetched.UserID)
	})

	t.Run("list by user", func(t *testing.T) {
		tokens, err := repo.List(ctx, user.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(tokens), 1)
	})

	t.Run("revoke", func(t *testing.T) {
		token := &models.APIToken{UserID: user.ID, Name: "revoke-me", TokenHash: "revoke-" + uuid.New().String()}
		require.NoError(t, repo.Create(ctx, token))

		require.NoError(t, repo.Revoke(ctx, token.ID))

		// Revoked token should not be found
		_, err := repo.GetByHash(ctx, token.TokenHash)
		assert.Error(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.GetByHash(ctx, "nonexistent-hash")
		assert.Error(t, err)
	})
}

func TestAuditLogRepository_Search(t *testing.T) {
	database := testutil.ConnectDB(t)

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	enterprise := &models.Enterprise{Name: "audit-search-" + uuid.New().String()[:8], Slug: "audit-search-" + uuid.New().String()[:8]}
	require.NoError(t, entRepo.Create(context.Background(), enterprise))
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	// Insert audit logs directly
	ctx := context.Background()
	for _, action := range []string{"device.create", "device.lock", "policy.update", "user.create"} {
		_, err := database.Writer.ExecContext(ctx,
			`INSERT INTO audit_logs (id, enterprise_id, action, resource_type, created_at) VALUES ($1, $2, $3, 'test', NOW())`,
			uuid.New(), enterprise.ID, action)
		require.NoError(t, err)
	}

	repo, err := repository.NewAuditLogRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	t.Run("search by action", func(t *testing.T) {
		logs, total, err := repo.Search(ctx, enterprise.ID, "device", "", "", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, total) // device.create + device.lock
		assert.Len(t, logs, 2)
	})

	t.Run("search with no filter returns all", func(t *testing.T) {
		logs, total, err := repo.Search(ctx, enterprise.ID, "", "", "", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Len(t, logs, 4)
	})

	t.Run("search with date range", func(t *testing.T) {
		logs, total, err := repo.Search(ctx, enterprise.ID, "", "2020-01-01", "2099-12-31", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Len(t, logs, 4)
	})

	t.Run("search with future start date returns empty", func(t *testing.T) {
		logs, total, err := repo.Search(ctx, enterprise.ID, "", "2099-01-01", "", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, logs)
	})
}
