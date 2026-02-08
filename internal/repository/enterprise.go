package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/validation"
)

type EnterpriseRepository interface {
	Create(ctx context.Context, enterprise *models.Enterprise) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Enterprise, error)
	GetBySlug(ctx context.Context, slug string) (*models.Enterprise, error)
	List(ctx context.Context, limit, offset int) ([]*models.Enterprise, int, error)
	Update(ctx context.Context, enterprise *models.Enterprise) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type enterpriseRepository struct {
	db executor
}

func NewEnterpriseRepository(db interface{}) (EnterpriseRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	
	switch v := db.(type) {
	case *sql.DB:
		return &enterpriseRepository{db: v}, nil
	case executor:
		return &enterpriseRepository{db: v}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %T", db)
	}
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
	
	return getExecutor(ctx, r.db).QueryRowContext(ctx, query,
		enterprise.ID, enterprise.Name, enterprise.Slug, enterprise.Settings,
	).Scan(&enterprise.CreatedAt, &enterprise.UpdatedAt)
}

func (r *enterpriseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Enterprise, error) {
	query := `
		SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		FROM enterprises
		WHERE id = $1 AND deleted_at IS NULL`
	
	enterprise := &models.Enterprise{}
	err := getExecutor(ctx, r.db).QueryRowContext(ctx, query, id).Scan(
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
	err := getExecutor(ctx, r.db).QueryRowContext(ctx, query, slug).Scan(
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

	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM enterprises WHERE deleted_at IS NULL`
	if err := getExecutor(ctx, r.db).QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}
	
	query := `
		SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		FROM enterprises
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
	
	rows, err := getExecutor(ctx, r.db).QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	enterprises := []*models.Enterprise{}
	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		default:
		}
		
		enterprise := &models.Enterprise{}
		if err := rows.Scan(
			&enterprise.ID, &enterprise.Name, &enterprise.Slug, &enterprise.Settings,
			&enterprise.CreatedAt, &enterprise.UpdatedAt, &enterprise.DeletedAt,
		); err != nil {
			return nil, 0, err
		}
		enterprises = append(enterprises, enterprise)
	}
	
	return enterprises, total, rows.Err()
}

func (r *enterpriseRepository) Update(ctx context.Context, enterprise *models.Enterprise) error {
	if err := validation.ValidateJSONB(enterprise.Settings, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	query := `
		UPDATE enterprises
		SET name = $1, settings = $2
		WHERE id = $3 AND deleted_at IS NULL`
	
	result, err := getExecutor(ctx, r.db).ExecContext(ctx, query,
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
	result, err := getExecutor(ctx, r.db).ExecContext(ctx, query, id)
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
