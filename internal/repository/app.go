package repository

import (
	"context"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// AppRepository provides data access for the app catalog.
type AppRepository interface {
	Create(ctx context.Context, app *models.App) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.App, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.App, int, error)
	Update(ctx context.Context, app *models.App) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type appRepository struct {
	writer executor
	reader executor
}

// NewAppRepository creates a new app repository instance.
// writer is used for Create/Update/Delete, reader for Get/List queries.
func NewAppRepository(writer, reader interface{}) (AppRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &appRepository{writer: w, reader: r}, nil
}

func (r *appRepository) Create(ctx context.Context, app *models.App) error {
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}

	query := `
		INSERT INTO apps (id, enterprise_id, name, platform, identifier, version, install_type, app_config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`

	exec := getExecutor(ctx, r.writer)
	return exec.QueryRowContext(ctx, query,
		app.ID, app.EnterpriseID, app.Name, app.Platform,
		app.Identifier, app.Version, app.InstallType, app.AppConfig,
	).Scan(&app.CreatedAt, &app.UpdatedAt)
}

func (r *appRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.App, error) {
	query := `
		SELECT id, enterprise_id, name, platform, identifier, version, install_type, app_config, created_at, updated_at
		FROM apps WHERE id = $1 AND deleted_at IS NULL`

	exec := getReadExecutor(ctx, r.reader)
	app := &models.App{}
	err := exec.QueryRowContext(ctx, query, id).Scan(
		&app.ID, &app.EnterpriseID, &app.Name, &app.Platform,
		&app.Identifier, &app.Version, &app.InstallType, &app.AppConfig,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("app not found: %w", apperrors.ErrNotFound)
	}
	return app, nil
}

func (r *appRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.App, int, error) {
	vLimit, vOffset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	countQuery := `SELECT COUNT(*) FROM apps WHERE enterprise_id = $1 AND deleted_at IS NULL`
	exec := getReadExecutor(ctx, r.reader)
	var total int
	if err := exec.QueryRowContext(ctx, countQuery, enterpriseID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count apps: %w", err)
	}

	query := `
		SELECT id, enterprise_id, name, platform, identifier, version, install_type, app_config, created_at, updated_at
		FROM apps WHERE enterprise_id = $1 AND deleted_at IS NULL
		ORDER BY name ASC LIMIT $2 OFFSET $3`

	rows, err := exec.QueryContext(ctx, query, enterpriseID, vLimit, vOffset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list apps: %w", err)
	}
	defer rows.Close()

	var apps []*models.App
	for rows.Next() {
		app := &models.App{}
		if err := rows.Scan(
			&app.ID, &app.EnterpriseID, &app.Name, &app.Platform,
			&app.Identifier, &app.Version, &app.InstallType, &app.AppConfig,
			&app.CreatedAt, &app.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan app: %w", err)
		}
		apps = append(apps, app)
	}
	return apps, total, rows.Err()
}

func (r *appRepository) Update(ctx context.Context, app *models.App) error {
	query := `
		UPDATE apps SET name = $1, version = $2, install_type = $3, app_config = $4
		WHERE id = $5 AND deleted_at IS NULL`

	exec := getExecutor(ctx, r.writer)
	result, err := exec.ExecContext(ctx, query, app.Name, app.Version, app.InstallType, app.AppConfig, app.ID)
	if err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("app not found: %w", apperrors.ErrNotFound)
	}
	return nil
}

func (r *appRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE apps SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	exec := getExecutor(ctx, r.writer)
	result, err := exec.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("app not found: %w", apperrors.ErrNotFound)
	}
	return nil
}
