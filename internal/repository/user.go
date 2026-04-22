package repository

import (
	"context"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, enterpriseID uuid.UUID, email string) (*models.User, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.User, int, error)
	Update(ctx context.Context, user *models.User) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type userRepository struct {
	writer executor
	reader executor
}

func NewUserRepository(writer, reader interface{}) (UserRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &userRepository{writer: w, reader: r}, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	return getExecutor(ctx, r.writer).QueryRowContext(ctx,
		`INSERT INTO users (id, enterprise_id, email, password_hash, full_name, role, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING created_at, updated_at`,
		user.ID, user.EnterpriseID, user.Email, user.PasswordHash, user.FullName, user.Role, user.IsActive,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u := &models.User{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT id, enterprise_id, email, password_hash, COALESCE(full_name, ''), role, is_active, last_login_at, created_at, updated_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&u.ID, &u.EnterpriseID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", apperrors.ErrNotFound)
	}
	return u, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, enterpriseID uuid.UUID, email string) (*models.User, error) {
	u := &models.User{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT id, enterprise_id, email, password_hash, COALESCE(full_name, ''), role, is_active, last_login_at, created_at, updated_at
		 FROM users WHERE enterprise_id = $1 AND email = $2 AND deleted_at IS NULL`, enterpriseID, email,
	).Scan(&u.ID, &u.EnterpriseID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", apperrors.ErrNotFound)
	}
	return u, nil
}

func (r *userRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	vLimit, vOffset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	exec := getReadExecutor(ctx, r.reader)
	var total int
	if err := exec.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE enterprise_id = $1 AND deleted_at IS NULL`, enterpriseID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := exec.QueryContext(ctx,
		`SELECT id, enterprise_id, email, password_hash, COALESCE(full_name, ''), role, is_active, last_login_at, created_at, updated_at
		 FROM users WHERE enterprise_id = $1 AND deleted_at IS NULL ORDER BY email ASC LIMIT $2 OFFSET $3`,
		enterpriseID, vLimit, vOffset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.EnterpriseID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE users SET full_name = $1, role = $2, is_active = $3 WHERE id = $4 AND deleted_at IS NULL`,
		user.FullName, user.Role, user.IsActive, user.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found: %w", apperrors.ErrNotFound)
	}
	return nil
}

func (r *userRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE users SET deleted_at = NOW(), is_active = false WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found: %w", apperrors.ErrNotFound)
	}
	return nil
}
