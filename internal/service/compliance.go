package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

type ComplianceRepository interface {
	Upsert(ctx context.Context, result *models.ComplianceResult) error
	GetByDevice(ctx context.Context, deviceID uuid.UUID) ([]*models.ComplianceResult, error)
	GetSummary(ctx context.Context, enterpriseID uuid.UUID) (*models.ComplianceSummary, error)
}

type ComplianceService struct {
	complianceRepo ComplianceRepository
	groupService   *GroupService
	policyRepo     PolicyRepository
	deviceRepo     DeviceRepository
	logger         *slog.Logger
}

func NewComplianceService(complianceRepo ComplianceRepository, groupService *GroupService, policyRepo PolicyRepository, deviceRepo DeviceRepository, logger *slog.Logger) *ComplianceService {
	return &ComplianceService{
		complianceRepo: complianceRepo,
		groupService:   groupService,
		policyRepo:     policyRepo,
		deviceRepo:     deviceRepo,
		logger:         logger,
	}
}

func (s *ComplianceService) EvaluateDevice(ctx context.Context, deviceID, enterpriseID uuid.UUID) ([]*models.ComplianceResult, error) {
	assignments, err := s.groupService.GetEffectivePolicies(ctx, deviceID, enterpriseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective policies: %w", err)
	}
	if len(assignments) == 0 {
		return nil, nil
	}

	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	var results []*models.ComplianceResult
	for _, a := range assignments {
		policy, err := s.policyRepo.GetByID(ctx, a.PolicyID)
		if err != nil {
			s.logger.Error("failed to get policy for compliance", "error", err, "policy_id", a.PolicyID)
			continue
		}
		result := s.evaluatePolicy(device, policy)
		if err := s.complianceRepo.Upsert(ctx, result); err != nil {
			s.logger.Error("failed to store compliance result", "error", err, "device_id", deviceID, "policy_id", a.PolicyID)
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

// evaluatePolicy compares device state against policy requirements.
func (s *ComplianceService) evaluatePolicy(device *models.Device, policy *models.Policy) *models.ComplianceResult {
	if device.PlatformData == nil || len(device.PlatformData) == 0 {
		return &models.ComplianceResult{
			DeviceID: device.ID,
			PolicyID: policy.ID,
			Status:   models.ComplianceStatusUnknown,
			Details:  models.JSONB{"reason": "no device state reported"},
		}
	}

	config := policy.PolicyConfig
	if config == nil {
		return &models.ComplianceResult{
			DeviceID: device.ID,
			PolicyID: policy.ID,
			Status:   models.ComplianceStatusCompliant,
			Details:  models.JSONB{"reason": "empty policy config"},
		}
	}

	violations := s.checkPolicy(device, policy.PolicyType, config)

	status := models.ComplianceStatusCompliant
	if len(violations) > 0 {
		status = models.ComplianceStatusNonCompliant
	}

	return &models.ComplianceResult{
		DeviceID: device.ID,
		PolicyID: policy.ID,
		Status:   status,
		Details: models.JSONB{
			"policy_type": policy.PolicyType,
			"violations":  violations,
		},
	}
}

// checkPolicy returns a list of violation descriptions.
func (s *ComplianceService) checkPolicy(device *models.Device, policyType string, config models.JSONB) []string {
	switch policyType {
	case "security":
		return s.checkSecurityPolicy(device, config)
	case "restriction":
		return s.checkRestrictionPolicy(device, config)
	default:
		return nil // wifi, vpn, app policies don't have compliance checks
	}
}

func (s *ComplianceService) checkSecurityPolicy(device *models.Device, config models.JSONB) []string {
	var violations []string
	pd := device.PlatformData

	// Check password requirements
	if req, ok := getBool(config, "require_password"); ok && req {
		if val, ok := getBool(pd, "password_present"); ok && !val {
			violations = append(violations, "password not set")
		}
	}
	if minLen, ok := getFloat(config, "min_password_length"); ok {
		if actual, ok := getFloat(pd, "password_length"); ok && actual < minLen {
			violations = append(violations, fmt.Sprintf("password length %v < required %v", actual, minLen))
		}
	}

	// Check encryption
	if req, ok := getBool(config, "require_encryption"); ok && req {
		encrypted := false
		if v, ok := getBool(pd, "encryption_enabled"); ok {
			encrypted = v
		}
		// macOS uses FileVault
		if v, ok := getBool(pd, "FileVaultEnabled"); ok {
			encrypted = v
		}
		// Windows uses BitLocker
		if v, ok := getString(pd, "bitlocker_status"); ok && strings.EqualFold(v, "enabled") {
			encrypted = true
		}
		if !encrypted {
			violations = append(violations, "disk encryption not enabled")
		}
	}

	// Check firewall
	if req, ok := getBool(config, "require_firewall"); ok && req {
		firewallOn := false
		if v, ok := getBool(pd, "firewall_enabled"); ok {
			firewallOn = v
		}
		if v, ok := getBool(pd, "FirewallEnabled"); ok {
			firewallOn = v
		}
		if !firewallOn {
			violations = append(violations, "firewall not enabled")
		}
	}

	return violations
}

func (s *ComplianceService) checkRestrictionPolicy(device *models.Device, config models.JSONB) []string {
	var violations []string
	pd := device.PlatformData

	if req, ok := getBool(config, "allow_camera"); ok && !req {
		if v, ok := getBool(pd, "camera_enabled"); ok && v {
			violations = append(violations, "camera is enabled but restricted")
		}
	}

	return violations
}

// JSONB helpers
func getBool(j models.JSONB, key string) (bool, bool) {
	v, ok := j[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func getFloat(j models.JSONB, key string) (float64, bool) {
	v, ok := j[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

func getString(j models.JSONB, key string) (string, bool) {
	v, ok := j[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (s *ComplianceService) GetDeviceCompliance(ctx context.Context, deviceID uuid.UUID) ([]*models.ComplianceResult, error) {
	return s.complianceRepo.GetByDevice(ctx, deviceID)
}

func (s *ComplianceService) GetSummary(ctx context.Context, enterpriseID uuid.UUID) (*models.ComplianceSummary, error) {
	return s.complianceRepo.GetSummary(ctx, enterpriseID)
}
