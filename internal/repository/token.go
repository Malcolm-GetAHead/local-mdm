package repository

import (
	"context"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

type TokenRepository interface {
	Create(ctx context.Context, token *models.APIToken) error
	GetByHash(ctx context.Context, tokenHash string) (*models.APIToken, error)
	List(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}

type tokenRepository struct {
	writer executor
	reader executor
}

func NewTokenRepository(writer, reader interface{}) (TokenRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &tokenRepository{writer: w, reader: r}, nil
}

func (r *tokenRepository) Create(ctx context.Context, token *models.APIToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	return getExecutor(ctx, r.writer).QueryRowContext(ctx,
		`INSERT INTO api_tokens (id, user_id, name, token_hash, scopes, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at, updated_at`,
		token.ID, token.UserID, token.Name, token.TokenHash, token.Scopes, token.ExpiresAt,
	).Scan(&token.CreatedAt, &token.UpdatedAt)
}

func (r *tokenRepository) GetByHash(ctx context.Context, tokenHash string) (*models.APIToken, error) {
	t := &models.APIToken{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT id, user_id, name, token_hash, scopes, last_used_at, expires_at, created_at, updated_at, revoked_at
		 FROM api_tokens WHERE token_hash = $1 AND revoked_at IS NULL
		 AND (expires_at IS NULL OR expires_at > NOW())`, tokenHash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.Scopes, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt, &t.UpdatedAt, &t.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("token not found: %w", apperrors.ErrNotFound)
	}
	return t, nil
}

func (r *tokenRepository) List(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error) {
	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT id, user_id, name, scopes, last_used_at, expires_at, created_at, revoked_at
		 FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []*models.APIToken
	for rows.Next() {
		t := &models.APIToken{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Scopes, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *tokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE api_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("token not found: %w", apperrors.ErrNotFound)
	}
	return nil
}

func (r *tokenRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}
