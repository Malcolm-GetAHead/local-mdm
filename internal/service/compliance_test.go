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

// --- Tests ---

func TestComplianceService_EvaluateDevice(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	gr := newMockGroupRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(gr, ar, testLogger())

	svc := NewComplianceService(cr, gs, pr, testLogger())

	eid := uuid.New()
	deviceID := uuid.New()
	policyID := uuid.New()

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
	assert.Equal(t, "awaiting device state report", results[0].Details["reason"])
}

func TestComplianceService_EvaluateDevice_NoPolicies(t *testing.T) {
	cr := newMockComplianceRepo()
	gs := NewGroupService(newMockGroupRepo(), &mockAssignmentRepo{}, testLogger())
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), testLogger())

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
	svc := NewComplianceService(cr, gs, pr, testLogger())

	eid := uuid.New()
	deviceID := uuid.New()

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
	svc := NewComplianceService(cr, gs, pr, testLogger())

	deviceID := uuid.New()
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
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), testLogger())

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
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), testLogger())

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
	svc := NewComplianceService(cr, gs, newMockPolicyRepo(), testLogger())

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

func TestComplianceService_EvaluatePolicy_ReturnsUnknown(t *testing.T) {
	// Directly test the evaluatePolicy method to document current behavior
	// and catch when S5-09 changes it
	svc := NewComplianceService(nil, nil, nil, testLogger())

	deviceID := uuid.New()
	policy := &models.Policy{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{"min_password_length": float64(8)},
	}

	result := svc.evaluatePolicy(deviceID, policy)
	assert.Equal(t, models.ComplianceStatusUnknown, result.Status)
	assert.Equal(t, "awaiting device state report", result.Details["reason"])
	assert.Equal(t, models.PolicyTypeSecurity, result.Details["policy_type"])
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

func TestComplianceService_EvaluateDevice_UpsertError(t *testing.T) {
	cr := &mockComplianceRepoErr{upsertErr: fmt.Errorf("db connection lost")}
	pr := newMockPolicyRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(newMockGroupRepo(), ar, testLogger())
	svc := NewComplianceService(cr, gs, pr, testLogger())

	deviceID := uuid.New()
	policyID := uuid.New()
	pr.policies[policyID] = &models.Policy{BaseModel: models.BaseModel{ID: policyID}, PolicyType: models.PolicyTypeSecurity, PolicyConfig: models.JSONB{}}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	// Should not error — upsert failures are logged, not propagated
	results, err := svc.EvaluateDevice(context.Background(), deviceID, uuid.New())
	require.NoError(t, err)
	assert.Len(t, results, 0) // result not added because upsert failed
}
