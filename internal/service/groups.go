package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// GroupRepository is the interface for group data access.
type GroupRepository interface {
	Create(ctx context.Context, group *models.DeviceGroup) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.DeviceGroup, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.DeviceGroup, int, error)
	Update(ctx context.Context, group *models.DeviceGroup) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, groupID, deviceID uuid.UUID) error
	RemoveMember(ctx context.Context, groupID, deviceID uuid.UUID) error
	ListMembers(ctx context.Context, groupID uuid.UUID, limit, offset int) ([]*models.Device, int, error)
	ListGroupsForDevice(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceGroup, error)
	CountMembersByGroupIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error)
}

// PolicyAssignmentRepository is the interface for policy assignment data access.
type PolicyAssignmentRepository interface {
	Create(ctx context.Context, a *models.PolicyAssignment) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]*models.PolicyAssignment, error)
	ListByPolicy(ctx context.Context, policyID uuid.UUID) ([]*models.PolicyAssignment, error)
	GetEffectivePolicies(ctx context.Context, deviceID uuid.UUID, groupIDs []uuid.UUID, enterpriseID uuid.UUID) ([]*models.PolicyAssignment, error)
}

// GroupService handles device group and policy assignment business logic.
type GroupService struct {
	groupRepo      GroupRepository
	assignmentRepo PolicyAssignmentRepository
	logger         *slog.Logger
}

// NewGroupService creates a new group service.
func NewGroupService(groupRepo GroupRepository, assignmentRepo PolicyAssignmentRepository, logger *slog.Logger) *GroupService {
	return &GroupService{
		groupRepo:      groupRepo,
		assignmentRepo: assignmentRepo,
		logger:         logger,
	}
}

// CreateGroup creates a new device group.
func (s *GroupService) CreateGroup(ctx context.Context, group *models.DeviceGroup) error {
	return s.groupRepo.Create(ctx, group)
}

// GetGroup returns a group by ID.
func (s *GroupService) GetGroup(ctx context.Context, id uuid.UUID) (*models.DeviceGroup, error) {
	return s.groupRepo.GetByID(ctx, id)
}

// ListGroups returns groups for an enterprise.
func (s *GroupService) ListGroups(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.DeviceGroup, int, error) {
	return s.groupRepo.List(ctx, enterpriseID, limit, offset)
}

// UpdateGroup updates a group.
func (s *GroupService) UpdateGroup(ctx context.Context, group *models.DeviceGroup) error {
	return s.groupRepo.Update(ctx, group)
}

// DeleteGroup deletes a group.
func (s *GroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	return s.groupRepo.Delete(ctx, id)
}

// AddMember adds a device to a group.
func (s *GroupService) AddMember(ctx context.Context, groupID, deviceID uuid.UUID) error {
	return s.groupRepo.AddMember(ctx, groupID, deviceID)
}

// RemoveMember removes a device from a group.
func (s *GroupService) RemoveMember(ctx context.Context, groupID, deviceID uuid.UUID) error {
	return s.groupRepo.RemoveMember(ctx, groupID, deviceID)
}

// ListMembers returns devices in a group.
func (s *GroupService) ListMembers(ctx context.Context, groupID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	return s.groupRepo.ListMembers(ctx, groupID, limit, offset)
}

// AssignPolicy assigns a policy to a target (device, group, or enterprise).
func (s *GroupService) AssignPolicy(ctx context.Context, policyID uuid.UUID, targetType string, targetID uuid.UUID, priority int) (*models.PolicyAssignment, error) {
	if targetType != models.TargetTypeDevice && targetType != models.TargetTypeGroup && targetType != models.TargetTypeEnterprise {
		return nil, fmt.Errorf("invalid target_type: %s", targetType)
	}

	a := &models.PolicyAssignment{
		PolicyID:   policyID,
		TargetType: targetType,
		TargetID:   targetID,
		Priority:   priority,
	}
	if err := s.assignmentRepo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("failed to assign policy: %w", err)
	}
	return a, nil
}

// UnassignPolicy removes a policy assignment.
func (s *GroupService) UnassignPolicy(ctx context.Context, assignmentID uuid.UUID) error {
	return s.assignmentRepo.Delete(ctx, assignmentID)
}

// ListAssignments returns policy assignments for a target.
func (s *GroupService) ListAssignments(ctx context.Context, targetType string, targetID uuid.UUID) ([]*models.PolicyAssignment, error) {
	return s.assignmentRepo.ListByTarget(ctx, targetType, targetID)
}

// ListAssignmentsByPolicy returns all assignments for a policy.
func (s *GroupService) ListAssignmentsByPolicy(ctx context.Context, policyID uuid.UUID) ([]*models.PolicyAssignment, error) {
	return s.assignmentRepo.ListByPolicy(ctx, policyID)
}

// GetEffectivePolicies returns all policies that apply to a device (via direct, group, and enterprise assignments), ordered by priority.
func (s *GroupService) GetEffectivePolicies(ctx context.Context, deviceID, enterpriseID uuid.UUID) ([]*models.PolicyAssignment, error) {
	groups, err := s.groupRepo.ListGroupsForDevice(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list device groups: %w", err)
	}

	var groupIDs []uuid.UUID
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}

	return s.assignmentRepo.GetEffectivePolicies(ctx, deviceID, groupIDs, enterpriseID)
}

// GetDeviceGroups returns all groups a device belongs to.
func (s *GroupService) GetDeviceGroups(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceGroup, error) {
	return s.groupRepo.ListGroupsForDevice(ctx, deviceID)
}

// CountMembersByGroupIDs returns member counts for multiple groups in a single query.
func (s *GroupService) CountMembersByGroupIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return s.groupRepo.CountMembersByGroupIDs(ctx, ids)
}
