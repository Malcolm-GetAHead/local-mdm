package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/validation"
)

// EnterpriseRepository provides data access operations for enterprise management.
// It handles multi-tenant organization data and supports enterprise isolation.
type EnterpriseRepository interface {
	Create(ctx context.Context, enterprise *models.Enterprise) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Enterprise, error)
	GetBySlug(ctx context.Context, slug string) (*models.Enterprise, error)
	List(ctx context.Context, limit, offset int) ([]*models.Enterprise, int, error)
	Update(ctx context.Context, enterprise *models.Enterprise) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type enterpriseRepository struct {
	writer executor
	reader executor
}

// NewEnterpriseRepository creates a new enterprise repository instance.
// The writer and reader parameters must be either *sql.DB or an executor interface.
// Returns an error if either is nil or of an unsupported type.
func NewEnterpriseRepository(writer, reader interface{}) (EnterpriseRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &enterpriseRepository{writer: w, reader: r}, nil
}

func (r *enterpriseRepository) Create(ctx context.Context, enterprise *models.Enterprise) error {
	if err := validation.ValidateJSONB(enterprise.Settings, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	query := `
		INSERT INTO enterprises (id, name, slug, settings)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`
	
	if enterprise.ID == uuid.Nil {
		enterprise.ID = uuid.New()
	}
	
	return getExecutor(ctx, r.writer).QueryRowContext(ctx, query,
		enterprise.ID, enterprise.Name, enterprise.Slug, enterprise.Settings,
	).Scan(&enterprise.CreatedAt, &enterprise.UpdatedAt)
}

func (r *enterpriseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Enterprise, error) {
	query := `
		SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		FROM enterprises
		WHERE id = $1 AND deleted_at IS NULL`
	
	enterprise := &models.Enterprise{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx, query, id).Scan(
		&enterprise.ID, &enterprise.Name, &enterprise.Slug, &enterprise.Settings,
		&enterprise.CreatedAt, &enterprise.UpdatedAt, &enterprise.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("enterprise not found")
	}
	return enterprise, err
}

func (r *enterpriseRepository) GetBySlug(ctx context.Context, slug string) (*models.Enterprise, error) {
	query := `
		SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		FROM enterprises
		WHERE slug = $1 AND deleted_at IS NULL`
	
	enterprise := &models.Enterprise{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx, query, slug).Scan(
		&enterprise.ID, &enterprise.Name, &enterprise.Slug, &enterprise.Settings,
		&enterprise.CreatedAt, &enterprise.UpdatedAt, &enterprise.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("enterprise not found")
	}
	return enterprise, err
}

func (r *enterpriseRepository) List(ctx context.Context, limit, offset int) ([]*models.Enterprise, int, error) {
	// Validate pagination parameters
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pagination: %w", err)
	}

	countQuery := `SELECT COUNT(*) FROM enterprises WHERE deleted_at IS NULL`
	dataQuery := `
		SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		FROM enterprises
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	scanFn := func(rows *sql.Rows) (*models.Enterprise, error) {
		enterprise := &models.Enterprise{}
		err := rows.Scan(
			&enterprise.ID, &enterprise.Name, &enterprise.Slug, &enterprise.Settings,
			&enterprise.CreatedAt, &enterprise.UpdatedAt, &enterprise.DeletedAt,
		)
		return enterprise, err
	}

	return ExecutePaginatedQuery(
		ctx,
		getReadExecutor(ctx, r.reader),
		countQuery,
		nil, // no count args
		dataQuery,
		[]interface{}{limit, offset},
		scanFn,
	)
}

func (r *enterpriseRepository) Update(ctx context.Context, enterprise *models.Enterprise) error {
	if err := validation.ValidateJSONB(enterprise.Settings, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	query := `
		UPDATE enterprises
		SET name = $1, settings = $2
		WHERE id = $3 AND deleted_at IS NULL`
	
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx, query,
		enterprise.Name, enterprise.Settings, enterprise.ID,
	)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("enterprise not found")
	}
	
	return nil
}

func (r *enterpriseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE enterprises SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("enterprise not found")
	}
	
	return nil
}
