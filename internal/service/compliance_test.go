package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock compliance repo ---

type mockComplianceDeviceRepo struct {
	devices map[uuid.UUID]*models.Device
}

func newMockComplianceDeviceRepo() *mockComplianceDeviceRepo {
	return &mockComplianceDeviceRepo{devices: make(map[uuid.UUID]*models.Device)}
}
func (m *mockComplianceDeviceRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Device, error) {
	d, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found")
	}
	return d, nil
}
func (m *mockComplianceDeviceRepo) GetByPlatformID(_ context.Context, _, _ string) (*models.Device, error) {
	return nil, fmt.Errorf("device not found")
}
func (m *mockComplianceDeviceRepo) Create(_ context.Context, d *models.Device) error {
	m.devices[d.ID] = d
	return nil
}
func (m *mockComplianceDeviceRepo) List(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockComplianceDeviceRepo) ListFiltered(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _, _ int) ([]*models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockComplianceDeviceRepo) Update(_ context.Context, d *models.Device) error {
	m.devices[d.ID] = d
	return nil
}
func (m *mockComplianceDeviceRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.devices, id)
	return nil
}

type mockComplianceRepo struct {
	results map[string]*models.ComplianceResult // "deviceID:policyID" -> result
}

func newMockComplianceRepo() *mockComplianceRepo {
	return &mockComplianceRepo{results: make(map[string]*models.ComplianceResult)}
}

func (m *mockComplianceRepo) Upsert(_ context.Context, r *models.ComplianceResult) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	key := r.DeviceID.String() + ":" + r.PolicyID.String()
	m.results[key] = r
	return nil
}
func (m *mockComplianceRepo) GetByDevice(_ context.Context, deviceID uuid.UUID) ([]*models.ComplianceResult, error) {
	var result []*models.ComplianceResult
	for _, r := range m.results {
		if r.DeviceID == deviceID {
			result = append(result, r)
		}
	}
	return result, nil
}
func (m *mockComplianceRepo) GetSummary(_ context.Context, _ uuid.UUID) (*models.ComplianceSummary, error) {
	s := &models.ComplianceSummary{}
	for _, r := range m.results {
		switch r.Status {
		case models.ComplianceStatusCompliant:
			s.Compliant++
		case models.ComplianceStatusNonCompliant:
			s.NonCompliant++
		case models.ComplianceStatusUnknown:
			s.Unknown++
		case models.ComplianceStatusError:
			s.Error++
		}
		s.Total++
	}
	return s, nil
}
func (m *mockComplianceRepo) DeleteByDevice(_ context.Context, deviceID uuid.UUID) error {
	for key, r := range m.results {
		if r.DeviceID == deviceID {
			delete(m.results, key)
		}
	}
	return nil
}

// --- Tests ---

func TestComplianceService_EvaluateDevice(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	gr := newMockGroupRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(gr, ar, testLogger())

	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	eid := uuid.New()
	deviceID := uuid.New()
	policyID := uuid.New()

	// Add device to mock repo
	dr.devices[deviceID] = &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		EnterpriseID: eid,
		Platform:     models.PlatformWindows,
		PlatformData: models.JSONB{},
	}

	// Create a policy and assign it to the device
	policy := &models.Policy{
		BaseModel:    models.BaseModel{ID: policyID},
		EnterpriseID: eid,
		Name:         "Password Policy",
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{"min_password_length": float64(8)},
		IsActive:     true,
	}
	pr.policies[policyID] = policy

	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	// Evaluate
	results, err := svc.EvaluateDevice(context.Background(), deviceID, eid)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, deviceID, results[0].DeviceID)
	assert.Equal(t, policyID, results[0].PolicyID)
	assert.Equal(t, models.ComplianceStatusUnknown, results[0].Status)
	assert.Equal(t, "no device state reported", results[0].Details["reason"])
}

func TestComplianceService_EvaluateDevice_NoPolicies(t *testing.T) {
	cr := newMockComplianceRepo()
	gs := NewGroupService(newMockGroupRepo(), &mockAssignmentRepo{}, testLogger())
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), newMockComplianceDeviceRepo(), testLogger())

	results, err := svc.EvaluateDevice(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestComplianceService_EvaluateDevice_MultipleAssignments(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	gr := newMockGroupRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(gr, ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	eid := uuid.New()
	deviceID := uuid.New()

	// Add device to mock repo
	dr.devices[deviceID] = &models.Device{BaseModel: models.BaseModel{ID: deviceID}, EnterpriseID: eid, PlatformData: models.JSONB{}}

	// Create group, add device
	group := &models.DeviceGroup{EnterpriseID: eid, Name: "Eng"}
	gr.Create(context.Background(), group)
	gr.AddMember(context.Background(), group.ID, deviceID)

	// Two policies: one direct, one via group
	p1 := &models.Policy{BaseModel: models.BaseModel{ID: uuid.New()}, PolicyType: models.PolicyTypeSecurity, PolicyConfig: models.JSONB{}}
	p2 := &models.Policy{BaseModel: models.BaseModel{ID: uuid.New()}, PolicyType: models.PolicyTypeWiFi, PolicyConfig: models.JSONB{}}
	pr.policies[p1.ID] = p1
	pr.policies[p2.ID] = p2

	gs.AssignPolicy(context.Background(), p1.ID, models.TargetTypeDevice, deviceID, 1)
	gs.AssignPolicy(context.Background(), p2.ID, models.TargetTypeGroup, group.ID, 10)

	results, err := svc.EvaluateDevice(context.Background(), deviceID, eid)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestComplianceService_EvaluateDevice_PolicyNotFound(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	dr.devices[deviceID] = &models.Device{BaseModel: models.BaseModel{ID: deviceID}, PlatformData: models.JSONB{}}
	// Assign a policy that doesn't exist in the repo
	gs.AssignPolicy(context.Background(), uuid.New(), models.TargetTypeDevice, deviceID, 1)

	// Should not error — just skip the missing policy
	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestComplianceService_GetDeviceCompliance(t *testing.T) {
	cr := newMockComplianceRepo()
	gs := NewGroupService(newMockGroupRepo(), &mockAssignmentRepo{}, testLogger())
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), newMockComplianceDeviceRepo(), testLogger())

	deviceID := uuid.New()
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: deviceID, PolicyID: uuid.New(),
		Status: models.ComplianceStatusCompliant, Details: models.JSONB{},
	})
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: deviceID, PolicyID: uuid.New(),
		Status: models.ComplianceStatusNonCompliant, Details: models.JSONB{"reason": "password too short"},
	})

	results, err := svc.GetDeviceCompliance(context.Background(), deviceID)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestComplianceService_GetSummary(t *testing.T) {
	cr := newMockComplianceRepo()
	gs := NewGroupService(newMockGroupRepo(), &mockAssignmentRepo{}, testLogger())
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), newMockComplianceDeviceRepo(), testLogger())

	eid := uuid.New()
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: uuid.New(), PolicyID: uuid.New(), Status: models.ComplianceStatusCompliant,
	})
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: uuid.New(), PolicyID: uuid.New(), Status: models.ComplianceStatusNonCompliant,
	})
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: uuid.New(), PolicyID: uuid.New(), Status: models.ComplianceStatusUnknown,
	})

	summary, err := svc.GetSummary(context.Background(), eid)
	require.NoError(t, err)
	assert.Equal(t, 3, summary.Total)
	assert.Equal(t, 1, summary.Compliant)
	assert.Equal(t, 1, summary.NonCompliant)
	assert.Equal(t, 1, summary.Unknown)
}

func TestComplianceService_UpsertOverwrites(t *testing.T) {
	cr := newMockComplianceRepo()
	gs := NewGroupService(newMockGroupRepo(), &mockAssignmentRepo{}, testLogger())
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), newMockComplianceDeviceRepo(), testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()

	// First evaluation: unknown
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: deviceID, PolicyID: policyID, Status: models.ComplianceStatusUnknown,
	})

	// Second evaluation: compliant (overwrites)
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: deviceID, PolicyID: policyID, Status: models.ComplianceStatusCompliant,
	})

	results, err := svc.GetDeviceCompliance(context.Background(), deviceID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)
}

func TestComplianceService_EvaluatePolicy_NoDeviceState(t *testing.T) {
	svc := NewComplianceService(nil, nil, nil, nil, testLogger())

	device := &models.Device{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		PlatformData: nil, // no state reported
	}
	policy := &models.Policy{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{"min_password_length": float64(8)},
	}

	result := svc.evaluatePolicy(device, policy)
	assert.Equal(t, models.ComplianceStatusUnknown, result.Status)
	assert.Equal(t, "no device state reported", result.Details["reason"])
}

// mockComplianceRepoErr simulates a repo that returns errors
type mockComplianceRepoErr struct {
	upsertErr error
}

func (m *mockComplianceRepoErr) Upsert(_ context.Context, _ *models.ComplianceResult) error {
	return m.upsertErr
}
func (m *mockComplianceRepoErr) GetByDevice(_ context.Context, _ uuid.UUID) ([]*models.ComplianceResult, error) {
	return nil, nil
}
func (m *mockComplianceRepoErr) GetSummary(_ context.Context, _ uuid.UUID) (*models.ComplianceSummary, error) {
	return &models.ComplianceSummary{}, nil
}
func (m *mockComplianceRepoErr) DeleteByDevice(_ context.Context, _ uuid.UUID) error {
	return nil
}

func TestComplianceService_EvaluateDevice_UpsertError(t *testing.T) {
	cr := &mockComplianceRepoErr{upsertErr: fmt.Errorf("db connection lost")}
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()
	dr.devices[deviceID] = &models.Device{BaseModel: models.BaseModel{ID: deviceID}, PlatformData: models.JSONB{}}
	pr.policies[policyID] = &models.Policy{BaseModel: models.BaseModel{ID: policyID}, PolicyType: models.PolicyTypeSecurity, PolicyConfig: models.JSONB{}}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	// Should not error — upsert failures are logged, not propagated
	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	assert.Len(t, results, 0) // result not added because upsert failed
}

func TestComplianceService_SecurityPolicy_Violations(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()

	dr.devices[deviceID] = &models.Device{
		BaseModel: models.BaseModel{ID: deviceID},
		PlatformData: models.JSONB{
			"password_present": false,
			"password_length":  float64(4),
			"encryption_enabled": false,
			"firewall_enabled":   false,
		},
	}
	pr.policies[policyID] = &models.Policy{
		BaseModel:  models.BaseModel{ID: policyID},
		PolicyType: models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{
			"require_password":     true,
			"min_password_length":  float64(8),
			"require_encryption":   true,
			"require_firewall":     true,
		},
		IsActive: true,
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusNonCompliant, results[0].Status)
	violations := results[0].Details["violations"].(map[string]string)
	assert.Len(t, violations, 4)
	assert.Contains(t, violations, "require_password")
	assert.Contains(t, violations, "min_password_length")
	assert.Contains(t, violations, "require_encryption")
	assert.Contains(t, violations, "require_firewall")
}

func TestComplianceService_SecurityPolicy_Compliant(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()

	dr.devices[deviceID] = &models.Device{
		BaseModel: models.BaseModel{ID: deviceID},
		PlatformData: models.JSONB{
			"password_present":   true,
			"password_length":    float64(12),
			"encryption_enabled": true,
			"firewall_enabled":   true,
		},
	}
	pr.policies[policyID] = &models.Policy{
		BaseModel:  models.BaseModel{ID: policyID},
		PolicyType: models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{
			"require_password":    true,
			"min_password_length": float64(8),
			"require_encryption":  true,
			"require_firewall":    true,
		},
		IsActive: true,
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)
}

func TestComplianceService_SecurityPolicy_MacOSFileVault(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()

	dr.devices[deviceID] = &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		PlatformData: models.JSONB{"FileVaultEnabled": true, "FirewallEnabled": true},
	}
	pr.policies[policyID] = &models.Policy{
		BaseModel:    models.BaseModel{ID: policyID},
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{"require_encryption": true, "require_firewall": true},
		IsActive:     true,
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)
}

func TestComplianceService_SecurityPolicy_WindowsBitLocker(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()

	dr.devices[deviceID] = &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		PlatformData: models.JSONB{"bitlocker_status": "Enabled"},
	}
	pr.policies[policyID] = &models.Policy{
		BaseModel:    models.BaseModel{ID: policyID},
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{"require_encryption": true},
		IsActive:     true,
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)
}

func TestComplianceService_RestrictionPolicy(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()

	t.Run("camera restricted but enabled", func(t *testing.T) {
		dr.devices[deviceID] = &models.Device{
			BaseModel:    models.BaseModel{ID: deviceID},
			PlatformData: models.JSONB{"camera_enabled": true},
		}
		pr.policies[policyID] = &models.Policy{
			BaseModel:    models.BaseModel{ID: policyID},
			PolicyType:   "restriction",
			PolicyConfig: models.JSONB{"allow_camera": false},
			IsActive:     true,
		}
		gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

		results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, models.ComplianceStatusNonCompliant, results[0].Status)
	})

	t.Run("camera restricted and disabled", func(t *testing.T) {
		dr.devices[deviceID] = &models.Device{
			BaseModel:    models.BaseModel{ID: deviceID},
			PlatformData: models.JSONB{"camera_enabled": false},
		}
		results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)
	})
}

func TestComplianceService_EmptyPolicyConfig(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()

	dr.devices[deviceID] = &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		PlatformData: models.JSONB{"some_data": true},
	}
	pr.policies[policyID] = &models.Policy{
		BaseModel:    models.BaseModel{ID: policyID},
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: nil,
		IsActive:     true,
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)
	assert.Equal(t, "empty policy config", results[0].Details["reason"])
}

func TestComplianceService_EvaluateDeviceByID(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	entID := uuid.New()
	dr.devices[deviceID] = &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		EnterpriseID: entID,
		PlatformData: models.JSONB{"password_present": true},
	}

	policyID := uuid.New()
	pr.policies[policyID] = &models.Policy{
		BaseModel:    models.BaseModel{ID: policyID},
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{"require_password": true},
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	err := svc.EvaluateDeviceByID(context.Background(), deviceID)
	require.NoError(t, err)

	results, _ := cr.GetByDevice(context.Background(), deviceID)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)
}

func TestComplianceService_EvaluateDeviceByID_NotFound(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	err := svc.EvaluateDeviceByID(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestComplianceService_EvaluateAllForPolicy(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gr := newMockGroupRepo()
	gs := NewGroupService(gr, ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	deviceID := uuid.New()
	entID := uuid.New()
	dr.devices[deviceID] = &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		EnterpriseID: entID,
		PlatformData: models.JSONB{},
	}

	policyID := uuid.New()
	pr.policies[policyID] = &models.Policy{
		BaseModel:    models.BaseModel{ID: policyID},
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{},
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	err := svc.EvaluateAllForPolicy(context.Background(), policyID)
	require.NoError(t, err)

	results, _ := cr.GetByDevice(context.Background(), deviceID)
	assert.NotEmpty(t, results)
}

func TestComplianceCleanupHook(t *testing.T) {
	cr := newMockComplianceRepo()
	hook := NewComplianceCleanupHook(cr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: deviceID,
		PolicyID: policyID,
		Status:   models.ComplianceStatusCompliant,
	})

	device := &models.Device{BaseModel: models.BaseModel{ID: deviceID}}

	// OnUnenroll should clear results
	err := hook.OnUnenroll(context.Background(), device)
	require.NoError(t, err)
	results, _ := cr.GetByDevice(context.Background(), deviceID)
	assert.Empty(t, results)

	// Re-add and test OnWipe
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: deviceID,
		PolicyID: policyID,
		Status:   models.ComplianceStatusCompliant,
	})
	err = hook.OnWipe(context.Background(), device)
	require.NoError(t, err)
	results, _ = cr.GetByDevice(context.Background(), deviceID)
	assert.Empty(t, results)

	// Re-add and test OnDelete
	cr.Upsert(context.Background(), &models.ComplianceResult{
		DeviceID: deviceID,
		PolicyID: policyID,
		Status:   models.ComplianceStatusCompliant,
	})
	err = hook.OnDelete(context.Background(), device)
	require.NoError(t, err)
	results, _ = cr.GetByDevice(context.Background(), deviceID)
	assert.Empty(t, results)
}
