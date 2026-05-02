package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock repos ---

type mockPolicyRepo struct {
	policies  map[uuid.UUID]*models.Policy
	createErr error
	updateErr error
}

func newMockPolicyRepo() *mockPolicyRepo {
	return &mockPolicyRepo{policies: make(map[uuid.UUID]*models.Policy)}
}

func (m *mockPolicyRepo) Create(_ context.Context, p *models.Policy) error {
	if m.createErr != nil {
		return m.createErr
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	m.policies[p.ID] = p
	return nil
}
func (m *mockPolicyRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Policy, error) {
	if p, ok := m.policies[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("policy not found")
}
func (m *mockPolicyRepo) List(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.Policy, int, error) {
	var all []*models.Policy
	for _, p := range m.policies {
		all = append(all, p)
	}
	return all, len(all), nil
}
func (m *mockPolicyRepo) Update(_ context.Context, p *models.Policy) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.policies[p.ID] = p
	return nil
}
func (m *mockPolicyRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.policies, id)
	return nil
}
func (m *mockPolicyRepo) ListByIDs(_ context.Context, ids []uuid.UUID) ([]*models.Policy, error) {
	var result []*models.Policy
	for _, id := range ids {
		if p, ok := m.policies[id]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}
func (m *mockPolicyRepo) ListTemplates(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.Policy, int, error) {
	var templates []*models.Policy
	for _, p := range m.policies {
		if p.IsTemplate {
			templates = append(templates, p)
		}
	}
	return templates, len(templates), nil
}
func (m *mockPolicyRepo) AssignToDevice(_ context.Context, _, _ uuid.UUID) error   { return nil }
func (m *mockPolicyRepo) UnassignFromDevice(_ context.Context, _, _ uuid.UUID) error { return nil }

type mockVersionRepo struct {
	versions map[uuid.UUID][]*models.PolicyVersion
}

func newMockVersionRepo() *mockVersionRepo {
	return &mockVersionRepo{versions: make(map[uuid.UUID][]*models.PolicyVersion)}
}

func (m *mockVersionRepo) Create(_ context.Context, v *models.PolicyVersion) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	m.versions[v.PolicyID] = append(m.versions[v.PolicyID], v)
	return nil
}
func (m *mockVersionRepo) ListByPolicy(_ context.Context, policyID uuid.UUID, limit, offset int) ([]*models.PolicyVersion, int, error) {
	all := m.versions[policyID]
	total := len(all)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return nil, total, nil
	}
	return all[offset:end], total, nil
}
func (m *mockVersionRepo) GetByVersion(_ context.Context, policyID uuid.UUID, version int) (*models.PolicyVersion, error) {
	for _, v := range m.versions[policyID] {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, fmt.Errorf("policy version not found")
}
func (m *mockVersionRepo) LatestVersion(_ context.Context, policyID uuid.UUID) (int, error) {
	max := 0
	for _, v := range m.versions[policyID] {
		if v.Version > max {
			max = v.Version
		}
	}
	return max, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- Tests ---

func TestPolicyService_CreateAndVersion(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := newMockVersionRepo()
	svc := NewPolicyService(pr, vr, testLogger())

	policy := &models.Policy{
		EnterpriseID: uuid.New(),
		Name:         "Test WiFi",
		PolicyType:   models.PolicyTypeWiFi,
		Platform:     models.PlatformAll,
		PolicyConfig: models.JSONB{"ssid": "TestNet"},
		IsActive:     true,
	}

	err := svc.Create(context.Background(), policy, "admin@test.com")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, policy.ID)

	// Version 1 should exist
	versions := vr.versions[policy.ID]
	require.Len(t, versions, 1)
	assert.Equal(t, 1, versions[0].Version)
	assert.Equal(t, "TestNet", versions[0].PolicyConfig["ssid"])
}

func TestPolicyService_UpdateCreatesVersion(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := newMockVersionRepo()
	svc := NewPolicyService(pr, vr, testLogger())

	policy := &models.Policy{
		EnterpriseID: uuid.New(),
		Name:         "Original",
		PolicyType:   models.PolicyTypeSecurity,
		Platform:     models.PlatformAll,
		PolicyConfig: models.JSONB{"min_password_length": float64(6)},
		IsActive:     true,
	}
	require.NoError(t, svc.Create(context.Background(), policy, "admin"))

	policy.Name = "Updated"
	policy.PolicyConfig["min_password_length"] = float64(8)
	require.NoError(t, svc.Update(context.Background(), policy, "admin"))

	versions := vr.versions[policy.ID]
	require.Len(t, versions, 2)
	assert.Equal(t, 2, versions[1].Version)
	assert.Equal(t, "Updated", versions[1].Name)
}

func TestPolicyService_Rollback(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := newMockVersionRepo()
	svc := NewPolicyService(pr, vr, testLogger())

	policy := &models.Policy{
		EnterpriseID: uuid.New(),
		Name:         "V1",
		PolicyType:   models.PolicyTypeSecurity,
		Platform:     models.PlatformAll,
		PolicyConfig: models.JSONB{"min_password_length": float64(6)},
		IsActive:     true,
	}
	require.NoError(t, svc.Create(context.Background(), policy, "admin"))

	policy.Name = "V2"
	policy.PolicyConfig = models.JSONB{"min_password_length": float64(12)}
	require.NoError(t, svc.Update(context.Background(), policy, "admin"))

	// Rollback to version 1
	rolled, err := svc.Rollback(context.Background(), policy.ID, 1, "admin")
	require.NoError(t, err)
	assert.Equal(t, "V1", rolled.Name)

	// Should have 3 versions now (v1, v2, v3=rollback)
	assert.Len(t, vr.versions[policy.ID], 3)
}

func TestPolicyService_CloneTemplate(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := newMockVersionRepo()
	svc := NewPolicyService(pr, vr, testLogger())

	tmpl := &models.Policy{
		EnterpriseID: uuid.New(),
		Name:         "Basic Security Template",
		PolicyType:   models.PolicyTypeSecurity,
		Platform:     models.PlatformAll,
		PolicyConfig: models.JSONB{"min_password_length": float64(8)},
		IsTemplate:   true,
		IsActive:     true,
	}
	require.NoError(t, svc.Create(context.Background(), tmpl, "admin"))

	entID := uuid.New()
	cloned, err := svc.CloneTemplate(context.Background(), tmpl.ID, entID, "My Security Policy", "admin")
	require.NoError(t, err)
	assert.Equal(t, "My Security Policy", cloned.Name)
	assert.Equal(t, entID, cloned.EnterpriseID)
	assert.False(t, cloned.IsTemplate)
	assert.False(t, cloned.IsActive)
	assert.Equal(t, float64(8), cloned.PolicyConfig["min_password_length"])
}

func TestPolicyService_TranslateWiFi(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())

	policy := &models.Policy{
		Name:       "Corp WiFi",
		PolicyType: models.PolicyTypeWiFi,
		PolicyConfig: models.JSONB{
			"ssid":          "CorpNet",
			"security_type": "WPA2",
			"password":      "secret123",
			"auto_join":     true,
		},
	}

	// macOS
	result, err := svc.Translate(policy, models.PlatformMacOS)
	require.NoError(t, err)
	assert.Equal(t, models.PlatformMacOS, result.Platform)
	assert.Contains(t, result.Data.(string), "CorpNet")

	// Windows
	result, err = svc.Translate(policy, models.PlatformWindows)
	require.NoError(t, err)
	assert.Equal(t, models.PlatformWindows, result.Platform)

	// Android
	result, err = svc.Translate(policy, models.PlatformAndroid)
	require.NoError(t, err)
	assert.Equal(t, models.PlatformAndroid, result.Platform)
}

func TestPolicyService_TranslateSecurity(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())

	policy := &models.Policy{
		Name:       "Password Policy",
		PolicyType: models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{
			"min_password_length": float64(8),
			"max_failed_attempts": float64(5),
			"require_encryption":  true,
		},
	}

	// Windows
	result, err := svc.Translate(policy, models.PlatformWindows)
	require.NoError(t, err)
	assert.NotNil(t, result.Data)

	// Android
	result, err = svc.Translate(policy, models.PlatformAndroid)
	require.NoError(t, err)
	assert.NotNil(t, result.Data)
}

func TestPolicyService_TranslateAll(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())

	policy := &models.Policy{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:       "WiFi Policy",
		PolicyType: models.PolicyTypeWiFi,
		PolicyConfig: models.JSONB{
			"ssid":     "TestNet",
			"password": "pass",
		},
	}

	results := svc.TranslateAll(policy)
	assert.Len(t, results, 3)
}

func TestPolicyService_TranslateUnsupportedPlatform(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())

	_, err := svc.Translate(&models.Policy{PolicyType: models.PolicyTypeWiFi, PolicyConfig: models.JSONB{"ssid": "x"}}, "chromeos")
	assert.Error(t, err)
}
