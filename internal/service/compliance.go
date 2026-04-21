package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// ComplianceRepository provides data access for compliance results.
type ComplianceRepository interface {
	Upsert(ctx context.Context, result *models.ComplianceResult) error
	GetByDevice(ctx context.Context, deviceID uuid.UUID) ([]*models.ComplianceResult, error)
	GetSummary(ctx context.Context, enterpriseID uuid.UUID) (*models.ComplianceSummary, error)
}

// ComplianceService evaluates device compliance against assigned policies.
type ComplianceService struct {
	complianceRepo ComplianceRepository
	groupService   *GroupService
	policyRepo     PolicyRepository
	logger         *slog.Logger
}

// NewComplianceService creates a new compliance service.
func NewComplianceService(complianceRepo ComplianceRepository, groupService *GroupService, policyRepo PolicyRepository, logger *slog.Logger) *ComplianceService {
	return &ComplianceService{
		complianceRepo: complianceRepo,
		groupService:   groupService,
		policyRepo:     policyRepo,
		logger:         logger,
	}
}

// EvaluateDevice evaluates compliance for a single device against all its assigned policies.
func (s *ComplianceService) EvaluateDevice(ctx context.Context, deviceID, enterpriseID uuid.UUID) ([]*models.ComplianceResult, error) {
	assignments, err := s.groupService.GetEffectivePolicies(ctx, deviceID, enterpriseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective policies: %w", err)
	}

	var results []*models.ComplianceResult
	for _, a := range assignments {
		policy, err := s.policyRepo.GetByID(ctx, a.PolicyID)
		if err != nil {
			s.logger.Error("failed to get policy for compliance", "error", err, "policy_id", a.PolicyID)
			continue
		}

		result := s.evaluatePolicy(deviceID, policy)
		if err := s.complianceRepo.Upsert(ctx, result); err != nil {
			s.logger.Error("failed to store compliance result", "error", err, "device_id", deviceID, "policy_id", a.PolicyID)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// evaluatePolicy checks a device against a single policy.
// In Sprint 4 this is a basic check — the device state comparison
// requires device_info data which comes from check-in responses.
// For now, policies with assignments are marked "unknown" until
// the device reports state that can be compared.
func (s *ComplianceService) evaluatePolicy(deviceID uuid.UUID, policy *models.Policy) *models.ComplianceResult {
	return &models.ComplianceResult{
		DeviceID: deviceID,
		PolicyID: policy.ID,
		Status:   models.ComplianceStatusUnknown,
		Details:  models.JSONB{"reason": "awaiting device state report", "policy_type": policy.PolicyType},
	}
}

// GetDeviceCompliance returns compliance results for a device.
func (s *ComplianceService) GetDeviceCompliance(ctx context.Context, deviceID uuid.UUID) ([]*models.ComplianceResult, error) {
	return s.complianceRepo.GetByDevice(ctx, deviceID)
}

// GetSummary returns enterprise-wide compliance summary.
func (s *ComplianceService) GetSummary(ctx context.Context, enterpriseID uuid.UUID) (*models.ComplianceSummary, error) {
	return s.complianceRepo.GetSummary(ctx, enterpriseID)
}
