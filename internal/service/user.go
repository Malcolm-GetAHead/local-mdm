package service

import (
	"context"
	"fmt"
	"log/slog"

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

type UserService struct {
	userRepo UserRepository
	logger   *slog.Logger
}

func NewUserService(userRepo UserRepository, logger *slog.Logger) *UserService {
	return &UserService{userRepo: userRepo, logger: logger}
}

var validRoles = map[string]bool{
	models.RoleSuperAdmin: true,
	models.RoleAdmin:      true,
	models.RoleOperator:   true,
	models.RoleViewer:     true,
}

func (s *UserService) Create(ctx context.Context, user *models.User) error {
	if user.Email == "" {
		return fmt.Errorf("email is required")
	}
	if !validRoles[user.Role] {
		return fmt.Errorf("invalid role: %s", user.Role)
	}
	if user.PasswordHash == "" {
		user.PasswordHash = "oidc-managed" // No password — auth via Keycloak or API token
	}
	user.IsActive = true
	return s.userRepo.Create(ctx, user)
}

func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

func (s *UserService) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	return s.userRepo.List(ctx, enterpriseID, limit, offset)
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, fullName, role *string, isActive *bool) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if fullName != nil {
		user.FullName = *fullName
	}
	if role != nil {
		if !validRoles[*role] {
			return nil, fmt.Errorf("invalid role: %s", *role)
		}
		user.Role = *role
	}
	if isActive != nil {
		user.IsActive = *isActive
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Deactivate(ctx context.Context, id uuid.UUID) error {
	return s.userRepo.Deactivate(ctx, id)
}
