package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// EnrollmentTokenRepository provides data access for enrollment tokens.
type EnrollmentTokenRepository interface {
	Create(ctx context.Context, token *models.EnrollmentToken) error
	GetByToken(ctx context.Context, token string) (*models.EnrollmentToken, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.EnrollmentToken, int, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	DecrementUses(ctx context.Context, id uuid.UUID) error
}

type enrollmentTokenRepository struct {
	writer executor
	reader executor
}

func NewEnrollmentTokenRepository(writer, reader interface{}) (EnrollmentTokenRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &enrollmentTokenRepository{writer: w, reader: r}, nil
}

func (r *enrollmentTokenRepository) Create(ctx context.Context, token *models.EnrollmentToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	return getExecutor(ctx, r.writer).QueryRowContext(ctx,
		`INSERT INTO enrollment_tokens (id, enterprise_id, token, description, max_uses, uses_remaining, expires_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING created_at`,
		token.ID, token.EnterpriseID, token.Token, token.Description,
		token.MaxUses, token.UsesRemaining, token.ExpiresAt, token.CreatedBy,
	).Scan(&token.CreatedAt)
}

func (r *enrollmentTokenRepository) GetByToken(ctx context.Context, token string) (*models.EnrollmentToken, error) {
	t := &models.EnrollmentToken{}
	var description sql.NullString
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT id, enterprise_id, token, description, max_uses, uses_remaining, expires_at, created_by, created_at, revoked_at
		 FROM enrollment_tokens WHERE token = $1`, token,
	).Scan(&t.ID, &t.EnterpriseID, &t.Token, &description, &t.MaxUses, &t.UsesRemaining, &t.ExpiresAt, &t.CreatedBy, &t.CreatedAt, &t.RevokedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("enrollment token not found: %w", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get enrollment token: %w", err)
	}
	t.Description = description.String
	return t, nil
}

func (r *enrollmentTokenRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.EnrollmentToken, int, error) {
	var total int
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enrollment_tokens WHERE enterprise_id = $1`, enterpriseID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count enrollment tokens: %w", err)
	}

	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT id, enterprise_id, token, description, max_uses, uses_remaining, expires_at, created_by, created_at, revoked_at
		 FROM enrollment_tokens WHERE enterprise_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		enterpriseID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list enrollment tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*models.EnrollmentToken
	for rows.Next() {
		t := &models.EnrollmentToken{}
		var description sql.NullString
		if err := rows.Scan(&t.ID, &t.EnterpriseID, &t.Token, &description, &t.MaxUses, &t.UsesRemaining, &t.ExpiresAt, &t.CreatedBy, &t.CreatedAt, &t.RevokedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan enrollment token: %w", err)
		}
		t.Description = description.String
		tokens = append(tokens, t)
	}
	return tokens, total, rows.Err()
}

func (r *enrollmentTokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE enrollment_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("failed to revoke enrollment token: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("enrollment token not found or already revoked: %w", apperrors.ErrNotFound)
	}
	return nil
}

func (r *enrollmentTokenRepository) DecrementUses(ctx context.Context, id uuid.UUID) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE enrollment_tokens SET uses_remaining = uses_remaining - 1
		 WHERE id = $1 AND revoked_at IS NULL AND (uses_remaining IS NULL OR uses_remaining > 0)`, id)
	if err != nil {
		return fmt.Errorf("failed to decrement enrollment token uses: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("enrollment token exhausted or revoked: %w", apperrors.ErrNotFound)
	}
	return nil
}
