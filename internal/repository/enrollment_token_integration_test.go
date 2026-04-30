package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentTokenRepository(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()
	testutil.EnsureTestEnterprise(t, database.Writer)
	t.Cleanup(func() { testutil.CleanupTestData(t, database.Writer) })
	enterpriseID := testutil.TestEnterpriseID

	repo, err := repository.NewEnrollmentTokenRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	t.Run("create and get by token", func(t *testing.T) {
		maxUses := 5
		tok := &models.EnrollmentToken{
			EnterpriseID:  enterpriseID,
			Token:         "inttest-" + uuid.New().String()[:8],
			Description:   "integration test token",
			MaxUses:       &maxUses,
			UsesRemaining: &maxUses,
			ExpiresAt:     time.Now().Add(time.Hour),
		}
		err := repo.Create(ctx, tok)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, tok.ID)
		assert.False(t, tok.CreatedAt.IsZero())

		fetched, err := repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		assert.Equal(t, tok.ID, fetched.ID)
		assert.Equal(t, enterpriseID, fetched.EnterpriseID)
		assert.Equal(t, "integration test token", fetched.Description)
		assert.Equal(t, 5, *fetched.MaxUses)
		assert.Equal(t, 5, *fetched.UsesRemaining)
		assert.Equal(t, models.EnrollmentTokenStatusActive, fetched.Status)
		assert.Nil(t, fetched.RevokedAt)
	})

	t.Run("get by token not found", func(t *testing.T) {
		_, err := repo.GetByToken(ctx, "nonexistent-token-xyz")
		require.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("list by enterprise", func(t *testing.T) {
		// Create a second token
		tok2 := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-list-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, tok2))

		tokens, total, err := repo.List(ctx, enterpriseID, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(tokens), 2)
	})

	t.Run("list pagination", func(t *testing.T) {
		tokens, total, err := repo.List(ctx, enterpriseID, 1, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.Len(t, tokens, 1) // limit=1
	})

	t.Run("list empty enterprise", func(t *testing.T) {
		tokens, total, err := repo.List(ctx, uuid.New(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, tokens)
	})

	t.Run("revoke", func(t *testing.T) {
		tok := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-revoke-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, tok))

		err := repo.Revoke(ctx, tok.ID)
		require.NoError(t, err)

		fetched, err := repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		assert.NotNil(t, fetched.RevokedAt)
		assert.Equal(t, models.EnrollmentTokenStatusRevoked, fetched.Status)
	})

	t.Run("revoke not found", func(t *testing.T) {
		err := repo.Revoke(ctx, uuid.New())
		require.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("revoke already revoked", func(t *testing.T) {
		tok := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-double-revoke-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, tok))
		require.NoError(t, repo.Revoke(ctx, tok.ID))

		err := repo.Revoke(ctx, tok.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("decrement uses", func(t *testing.T) {
		maxUses := 3
		tok := &models.EnrollmentToken{
			EnterpriseID:  enterpriseID,
			Token:         "inttest-dec-" + uuid.New().String()[:8],
			MaxUses:       &maxUses,
			UsesRemaining: &maxUses,
			ExpiresAt:     time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, tok))

		require.NoError(t, repo.DecrementUses(ctx, tok.ID))

		fetched, err := repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		assert.Equal(t, 2, *fetched.UsesRemaining)

		// Decrement to 0
		require.NoError(t, repo.DecrementUses(ctx, tok.ID))
		require.NoError(t, repo.DecrementUses(ctx, tok.ID))

		fetched, err = repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		assert.Equal(t, 0, *fetched.UsesRemaining)

		// Can't decrement below 0
		err = repo.DecrementUses(ctx, tok.ID)
		require.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("decrement unlimited uses", func(t *testing.T) {
		tok := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-unlimited-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
			// MaxUses and UsesRemaining are nil = unlimited
		}
		require.NoError(t, repo.Create(ctx, tok))

		// Decrement should succeed (NULL uses_remaining matches the WHERE clause)
		require.NoError(t, repo.DecrementUses(ctx, tok.ID))
	})

	t.Run("create with created_by", func(t *testing.T) {
		// created_by references users table, so we need a real user
		userID := uuid.New()
		_, err := database.Writer.Exec(
			"INSERT INTO users (id, enterprise_id, email, full_name, role, is_active) VALUES ($1, $2, $3, $4, $5, true)",
			userID, enterpriseID, "test-"+uuid.New().String()[:8]+"@test.com", "Test User", "admin",
		)
		require.NoError(t, err)
		t.Cleanup(func() { database.Writer.Exec("DELETE FROM users WHERE id = $1", userID) })

		tok := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-createdby-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
			CreatedBy:    &userID,
		}
		require.NoError(t, repo.Create(ctx, tok))

		fetched, err := repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		require.NotNil(t, fetched.CreatedBy)
		assert.Equal(t, userID, *fetched.CreatedBy)
	})

	t.Run("null description scans correctly", func(t *testing.T) {
		tok := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-nulldesc-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
			// Description is empty string, stored as NULL
		}
		require.NoError(t, repo.Create(ctx, tok))

		fetched, err := repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		// Should not panic on NULL description scan
		assert.Equal(t, "", fetched.Description)
	})

	t.Run("set status", func(t *testing.T) {
		tok := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-setstatus-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, tok))
		assert.Equal(t, models.EnrollmentTokenStatusActive, tok.Status)

		require.NoError(t, repo.SetStatus(ctx, tok.ID, models.EnrollmentTokenStatusExpired))

		fetched, err := repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		assert.Equal(t, models.EnrollmentTokenStatusExpired, fetched.Status)
	})

	t.Run("expire tokens bulk", func(t *testing.T) {
		// Create an active token that's already past expiry
		tok := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-bulkexpire-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(-time.Hour),
		}
		require.NoError(t, repo.Create(ctx, tok))

		// Create an active token that's NOT expired
		tok2 := &models.EnrollmentToken{
			EnterpriseID: enterpriseID,
			Token:        "inttest-notexpired-" + uuid.New().String()[:8],
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, tok2))

		n, err := repo.ExpireTokens(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(1))

		// Expired token should have status=expired
		fetched, err := repo.GetByToken(ctx, tok.Token)
		require.NoError(t, err)
		assert.Equal(t, models.EnrollmentTokenStatusExpired, fetched.Status)

		// Non-expired token should still be active
		fetched2, err := repo.GetByToken(ctx, tok2.Token)
		require.NoError(t, err)
		assert.Equal(t, models.EnrollmentTokenStatusActive, fetched2.Status)
	})
}
