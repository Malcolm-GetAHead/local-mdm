package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/android"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
)

// PolicyRepository is the interface for policy data access.
type PolicyRepository interface {
	Create(ctx context.Context, policy *models.Policy) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Policy, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Policy, int, error)
	Update(ctx context.Context, policy *models.Policy) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.Policy, error)
}

// PolicyVersionRepository stores policy version snapshots.
type PolicyVersionRepository interface {
	Create(ctx context.Context, v *models.PolicyVersion) error
	ListByPolicy(ctx context.Context, policyID uuid.UUID, limit, offset int) ([]*models.PolicyVersion, int, error)
	GetByVersion(ctx context.Context, policyID uuid.UUID, version int) (*models.PolicyVersion, error)
	LatestVersion(ctx context.Context, policyID uuid.UUID) (int, error)
}

// TranslationResult holds the output of translating a policy for a platform.
type TranslationResult struct {
	Platform     string      `json:"platform"`
	Data         interface{} `json:"data"`
	Unsupported  []string    `json:"unsupported,omitempty"`
}

// PolicyService handles policy business logic.
type PolicyService struct {
	policyRepo  PolicyRepository
	versionRepo PolicyVersionRepository
	logger      *slog.Logger
}

// NewPolicyService creates a new policy service.
func NewPolicyService(policyRepo PolicyRepository, versionRepo PolicyVersionRepository, logger *slog.Logger) *PolicyService {
	return &PolicyService{
		policyRepo:  policyRepo,
		versionRepo: versionRepo,
		logger:      logger,
	}
}

// Create creates a policy and stores version 1.
func (s *PolicyService) Create(ctx context.Context, policy *models.Policy, createdBy string) error {
	if err := s.policyRepo.Create(ctx, policy); err != nil {
		return fmt.Errorf("failed to create policy: %w", err)
	}

	v := &models.PolicyVersion{
		PolicyID:     policy.ID,
		Version:      1,
		PolicyConfig: policy.PolicyConfig,
		Name:         policy.Name,
		Description:  policy.Description,
		Platform:     policy.Platform,
		PolicyType:   policy.PolicyType,
		IsActive:     policy.IsActive,
		CreatedBy:    createdBy,
	}
	if err := s.versionRepo.Create(ctx, v); err != nil {
		s.logger.Error("failed to create policy version", "error", err, "policy_id", policy.ID)
	}

	return nil
}

// Update updates a policy and creates a new version snapshot.
func (s *PolicyService) Update(ctx context.Context, policy *models.Policy, updatedBy string) error {
	if err := s.policyRepo.Update(ctx, policy); err != nil {
		return fmt.Errorf("failed to update policy: %w", err)
	}

	latest, err := s.versionRepo.LatestVersion(ctx, policy.ID)
	if err != nil {
		latest = 0
	}

	v := &models.PolicyVersion{
		PolicyID:     policy.ID,
		Version:      latest + 1,
		PolicyConfig: policy.PolicyConfig,
		Name:         policy.Name,
		Description:  policy.Description,
		Platform:     policy.Platform,
		PolicyType:   policy.PolicyType,
		IsActive:     policy.IsActive,
		CreatedBy:    updatedBy,
	}
	if err := s.versionRepo.Create(ctx, v); err != nil {
		s.logger.Error("failed to create policy version", "error", err, "policy_id", policy.ID)
	}

	return nil
}

// Rollback restores a policy to a previous version.
func (s *PolicyService) Rollback(ctx context.Context, policyID uuid.UUID, version int, rolledBackBy string) (*models.Policy, error) {
	snapshot, err := s.versionRepo.GetByVersion(ctx, policyID, version)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", version, err)
	}

	policy, err := s.policyRepo.GetByID(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}

	policy.Name = snapshot.Name
	policy.Description = snapshot.Description
	policy.PolicyConfig = snapshot.PolicyConfig
	policy.IsActive = snapshot.IsActive

	if err := s.Update(ctx, policy, rolledBackBy); err != nil {
		return nil, fmt.Errorf("failed to rollback: %w", err)
	}

	return policy, nil
}

// ListVersions returns version history for a policy.
func (s *PolicyService) ListVersions(ctx context.Context, policyID uuid.UUID, limit, offset int) ([]*models.PolicyVersion, int, error) {
	return s.versionRepo.ListByPolicy(ctx, policyID, limit, offset)
}

// CloneTemplate creates a new policy from a template.
func (s *PolicyService) CloneTemplate(ctx context.Context, templateID, enterpriseID uuid.UUID, name, createdBy string) (*models.Policy, error) {
	tmpl, err := s.policyRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	if !tmpl.IsTemplate {
		return nil, fmt.Errorf("policy is not a template")
	}

	policy := &models.Policy{
		EnterpriseID: enterpriseID,
		Name:         name,
		Description:  tmpl.Description,
		Platform:     tmpl.Platform,
		PolicyType:   tmpl.PolicyType,
		PolicyConfig: tmpl.PolicyConfig,
		IsActive:     false,
		IsTemplate:   false,
	}

	if err := s.Create(ctx, policy, createdBy); err != nil {
		return nil, err
	}

	return policy, nil
}

// Translate converts a policy to platform-specific format.
func (s *PolicyService) Translate(policy *models.Policy, platform string) (*TranslationResult, error) {
	switch platform {
	case models.PlatformMacOS:
		return s.translateMacOS(policy)
	case models.PlatformWindows:
		return s.translateWindows(policy)
	case models.PlatformAndroid:
		return s.translateAndroid(policy)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// TranslateAll translates a policy for all platforms.
func (s *PolicyService) TranslateAll(policy *models.Policy) []*TranslationResult {
	platforms := []string{models.PlatformMacOS, models.PlatformWindows, models.PlatformAndroid}
	var results []*TranslationResult
	for _, p := range platforms {
		result, err := s.Translate(policy, p)
		if err != nil {
			s.logger.Warn("translation failed", "platform", p, "policy_id", policy.ID, "error", err)
			continue
		}
		results = append(results, result)
	}
	return results
}

func (s *PolicyService) translateMacOS(policy *models.Policy) (*TranslationResult, error) {
	result := &TranslationResult{Platform: models.PlatformMacOS}
	cfg := policy.PolicyConfig

	switch policy.PolicyType {
	case models.PolicyTypeWiFi:
		profile, err := macos.GenerateWiFiProfile(macos.WiFiProfileConfig{
			ProfileConfig: macos.ProfileConfig{DisplayName: policy.Name, Description: policy.Description},
			SSID:          strVal(cfg, "ssid"),
			SecurityType:  strVal(cfg, "security_type"),
			Password:      strVal(cfg, "password"),
			AutoJoin:      boolVal(cfg, "auto_join"),
			IsHidden:      boolVal(cfg, "is_hidden"),
		})
		if err != nil {
			return nil, err
		}
		result.Data = string(profile)

	case models.PolicyTypeVPN:
		profile, err := macos.GenerateVPNProfile(macos.VPNProfileConfig{
			ProfileConfig: macos.ProfileConfig{DisplayName: policy.Name, Description: policy.Description},
			VPNType:       strVal(cfg, "vpn_type"),
			ServerAddress: strVal(cfg, "server"),
			RemoteID:      strVal(cfg, "remote_id"),
			LocalID:       strVal(cfg, "local_id"),
			SharedSecret:  strVal(cfg, "shared_secret"),
		})
		if err != nil {
			return nil, err
		}
		result.Data = string(profile)

	case models.PolicyTypeRestriction:
		rcfg := macos.RestrictionsProfileConfig{
			ProfileConfig: macos.ProfileConfig{DisplayName: policy.Name, Description: policy.Description},
		}
		if v, ok := cfg["allow_camera"]; ok {
			b := toBool(v)
			rcfg.AllowCamera = &b
		}
		if v, ok := cfg["allow_screen_capture"]; ok {
			b := toBool(v)
			rcfg.AllowScreenCapture = &b
		}
		profile, err := macos.GenerateRestrictionsProfile(rcfg)
		if err != nil {
			return nil, err
		}
		result.Data = string(profile)

	case models.PolicyTypeSecurity:
		// macOS security policies map to restrictions + password payload
		var unsupported []string
		if _, ok := cfg["min_password_length"]; ok {
			unsupported = append(unsupported, "min_password_length (use MDM password payload)")
		}
		result.Data = map[string]interface{}{"note": "macOS security policies require MDM password payload"}
		result.Unsupported = unsupported

	default:
		return nil, fmt.Errorf("unsupported policy type for macOS: %s", policy.PolicyType)
	}

	return result, nil
}

func (s *PolicyService) translateWindows(policy *models.Policy) (*TranslationResult, error) {
	result := &TranslationResult{Platform: models.PlatformWindows}

	switch policy.PolicyType {
	case models.PolicyTypeSecurity:
		cmds := windows.BuildPolicyCSPCommands(policy.PolicyConfig)
		result.Data = cmds

	case models.PolicyTypeWiFi:
		cmds, err := windows.BuildWiFiCSPCommands(policy.PolicyConfig)
		if err != nil {
			return nil, err
		}
		result.Data = cmds

	case models.PolicyTypeVPN:
		cmds, err := windows.BuildVPNCSPCommands(policy.PolicyConfig)
		if err != nil {
			return nil, err
		}
		result.Data = cmds

	case models.PolicyTypeRestriction:
		// Map restriction fields to policy CSP commands
		cfg := models.JSONB{}
		if v, ok := policy.PolicyConfig["require_encryption"]; ok {
			cfg["require_encryption"] = v
		}
		cmds := windows.BuildPolicyCSPCommands(cfg)
		result.Data = cmds

	default:
		return nil, fmt.Errorf("unsupported policy type for Windows: %s", policy.PolicyType)
	}

	return result, nil
}

func (s *PolicyService) translateAndroid(policy *models.Policy) (*TranslationResult, error) {
	ap := android.TranslatePolicy(policy)
	return &TranslationResult{
		Platform: models.PlatformAndroid,
		Data:     ap,
	}, nil
}

// helpers

func strVal(m models.JSONB, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolVal(m models.JSONB, key string) bool {
	return toBool(m[key])
}

func toBool(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case float64:
		return b != 0
	}
	return false
}
