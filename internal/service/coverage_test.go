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

// ============================================================
// 1. ExtraUUID / ExtraString
// ============================================================

func TestMDMEvent_ExtraUUID(t *testing.T) {
	validID := uuid.New()
	tests := []struct {
		name    string
		extra   map[string]interface{}
		key     string
		wantID  uuid.UUID
		wantOK  bool
	}{
		{"valid", map[string]interface{}{"id": validID.String()}, "id", validID, true},
		{"missing key", map[string]interface{}{}, "id", uuid.Nil, false},
		{"non-string", map[string]interface{}{"id": 42}, "id", uuid.Nil, false},
		{"unparseable", map[string]interface{}{"id": "not-a-uuid"}, "id", uuid.Nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := MDMEvent{Extra: tt.extra}
			got, ok := e.ExtraUUID(tt.key)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantID, got)
		})
	}
}

func TestMDMEvent_ExtraString(t *testing.T) {
	tests := []struct {
		name   string
		extra  map[string]interface{}
		key    string
		wantS  string
		wantOK bool
	}{
		{"valid", map[string]interface{}{"k": "hello"}, "k", "hello", true},
		{"missing", map[string]interface{}{}, "k", "", false},
		{"non-string", map[string]interface{}{"k": 123}, "k", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := MDMEvent{Extra: tt.extra}
			got, ok := e.ExtraString(tt.key)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantS, got)
		})
	}
}

// ============================================================
// 2. translateMacOS missing branches
// ============================================================

func TestTranslateMacOS_VPN(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	p := &models.Policy{
		Name:       "VPN",
		PolicyType: models.PolicyTypeVPN,
		PolicyConfig: models.JSONB{
			"vpn_type": "IKEv2", "server": "vpn.example.com", "remote_id": "vpn.example.com",
		},
	}
	r, err := svc.Translate(p, models.PlatformMacOS)
	require.NoError(t, err)
	assert.Contains(t, r.Data.(string), "vpn.example.com")
}

func TestTranslateMacOS_Restriction(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	p := &models.Policy{
		Name:       "Restrict",
		PolicyType: models.PolicyTypeRestriction,
		PolicyConfig: models.JSONB{
			"allow_camera": false, "allow_screen_capture": true,
		},
	}
	r, err := svc.Translate(p, models.PlatformMacOS)
	require.NoError(t, err)
	assert.NotEmpty(t, r.Data)
}

func TestTranslateMacOS_Security(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	p := &models.Policy{
		Name:       "Sec",
		PolicyType: models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{
			"min_password_length": float64(8),
		},
	}
	r, err := svc.Translate(p, models.PlatformMacOS)
	require.NoError(t, err)
	assert.NotEmpty(t, r.Unsupported)
}

func TestTranslateMacOS_Unsupported(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	_, err := svc.Translate(&models.Policy{PolicyType: "custom", PolicyConfig: models.JSONB{}}, models.PlatformMacOS)
	assert.Error(t, err)
}

// ============================================================
// 3. translateWindows missing branches
// ============================================================

func TestTranslateWindows_WiFi(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	p := &models.Policy{
		PolicyType: models.PolicyTypeWiFi,
		PolicyConfig: models.JSONB{
			"ssid": "Corp", "password": "pass", "security_type": "WPA2",
		},
	}
	r, err := svc.Translate(p, models.PlatformWindows)
	require.NoError(t, err)
	assert.NotNil(t, r.Data)
}

func TestTranslateWindows_VPN(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	p := &models.Policy{
		PolicyType: models.PolicyTypeVPN,
		PolicyConfig: models.JSONB{
			"name": "CorpVPN", "server": "vpn.example.com", "remote_id": "vpn.example.com", "always_on": true,
		},
	}
	r, err := svc.Translate(p, models.PlatformWindows)
	require.NoError(t, err)
	assert.NotNil(t, r.Data)
}

func TestTranslateWindows_Restriction(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	p := &models.Policy{
		PolicyType:   models.PolicyTypeRestriction,
		PolicyConfig: models.JSONB{"require_encryption": true},
	}
	r, err := svc.Translate(p, models.PlatformWindows)
	require.NoError(t, err)
	assert.NotNil(t, r.Data)
}

func TestTranslateWindows_Unsupported(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	_, err := svc.Translate(&models.Policy{PolicyType: "custom", PolicyConfig: models.JSONB{}}, models.PlatformWindows)
	assert.Error(t, err)
}

// ============================================================
// 4. TranslateAll with failure path
// ============================================================

func TestTranslateAll_WithUnsupportedType(t *testing.T) {
	svc := NewPolicyService(nil, nil, testLogger())
	p := &models.Policy{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		PolicyType:   "custom",
		PolicyConfig: models.JSONB{},
	}
	// macOS and Windows will fail, Android always succeeds
	results := svc.TranslateAll(p)
	assert.Len(t, results, 1) // only android
}

// ============================================================
// 5. EvaluateAllForPolicy with Group and Enterprise targets
// ============================================================

func TestEvaluateAllForPolicy_GroupTarget(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	gr := newMockGroupRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(gr, ar, testLogger())
	dr := newMockComplianceDeviceRepo()
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	eid := uuid.New()
	deviceID := uuid.New()
	groupID := uuid.New()
	policyID := uuid.New()

	gr.groups[groupID] = &models.DeviceGroup{BaseModel: models.BaseModel{ID: groupID}, EnterpriseID: eid}
	gr.members[groupID] = []uuid.UUID{deviceID}
	dr.devices[deviceID] = &models.Device{
		BaseModel: models.BaseModel{ID: deviceID}, EnterpriseID: eid, PlatformData: models.JSONB{},
	}
	pr.policies[policyID] = &models.Policy{
		BaseModel: models.BaseModel{ID: policyID}, PolicyType: models.PolicyTypeSecurity, PolicyConfig: models.JSONB{},
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeGroup, groupID, 10)

	err := svc.EvaluateAllForPolicy(context.Background(), policyID)
	require.NoError(t, err)
}

func TestEvaluateAllForPolicy_EnterpriseTarget(t *testing.T) {
	cr := newMockComplianceRepo()
	pr := newMockPolicyRepo()
	gr := newMockGroupRepo()
	ar := &mockAssignmentRepo{}
	gs := NewGroupService(gr, ar, testLogger())

	eid := uuid.New()
	deviceID := uuid.New()
	policyID := uuid.New()

	// Use a device repo that supports List by enterprise
	dr := &mockEntDeviceRepo{devices: map[uuid.UUID]*models.Device{
		deviceID: {BaseModel: models.BaseModel{ID: deviceID}, EnterpriseID: eid, PlatformData: models.JSONB{}},
	}}
	svc := NewComplianceService(cr, gs, pr, dr, testLogger())

	pr.policies[policyID] = &models.Policy{
		BaseModel: models.BaseModel{ID: policyID}, PolicyType: models.PolicyTypeSecurity, PolicyConfig: models.JSONB{},
	}
	gs.AssignPolicy(context.Background(), policyID, models.TargetTypeEnterprise, eid, 100)

	err := svc.EvaluateAllForPolicy(context.Background(), policyID)
	require.NoError(t, err)
}

// mockEntDeviceRepo returns devices by enterprise for List
type mockEntDeviceRepo struct {
	devices map[uuid.UUID]*models.Device
}

func (m *mockEntDeviceRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Device, error) {
	if d, ok := m.devices[id]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("device not found")
}
func (m *mockEntDeviceRepo) List(_ context.Context, eid uuid.UUID, _, _ int) ([]*models.Device, int, error) {
	var out []*models.Device
	for _, d := range m.devices {
		if d.EnterpriseID == eid {
			out = append(out, d)
		}
	}
	return out, len(out), nil
}
func (m *mockEntDeviceRepo) Update(_ context.Context, d *models.Device) error {
	m.devices[d.ID] = d
	return nil
}
func (m *mockEntDeviceRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.devices, id)
	return nil
}
func (m *mockEntDeviceRepo) ListFiltered(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _, _ int) ([]*models.Device, int, error) {
	var out []*models.Device
	for _, d := range m.devices {
		out = append(out, d)
	}
	return out, len(out), nil
}

// ============================================================
// 6. Pass-through methods
// ============================================================

func TestGroupService_ListAssignments(t *testing.T) {
	ar := &mockAssignmentRepo{}
	svc := NewGroupService(newMockGroupRepo(), ar, testLogger())
	policyID := uuid.New()
	deviceID := uuid.New()
	svc.AssignPolicy(context.Background(), policyID, models.TargetTypeDevice, deviceID, 1)

	list, err := svc.ListAssignments(context.Background(), models.TargetTypeDevice, deviceID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestGroupService_GetDeviceGroups(t *testing.T) {
	gr := newMockGroupRepo()
	svc := NewGroupService(gr, &mockAssignmentRepo{}, testLogger())
	deviceID := uuid.New()
	g := &models.DeviceGroup{EnterpriseID: uuid.New(), Name: "G"}
	gr.Create(context.Background(), g)
	gr.AddMember(context.Background(), g.ID, deviceID)

	groups, err := svc.GetDeviceGroups(context.Background(), deviceID)
	require.NoError(t, err)
	assert.Len(t, groups, 1)
}

func TestGroupService_CountMembersByGroupIDs(t *testing.T) {
	gr := newMockGroupRepo()
	svc := NewGroupService(gr, &mockAssignmentRepo{}, testLogger())
	g := &models.DeviceGroup{EnterpriseID: uuid.New(), Name: "G"}
	gr.Create(context.Background(), g)
	gr.AddMember(context.Background(), g.ID, uuid.New())

	counts, err := svc.CountMembersByGroupIDs(context.Background(), []uuid.UUID{g.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, counts[g.ID])
}

func TestPolicyService_ListVersions(t *testing.T) {
	vr := newMockVersionRepo()
	pr := newMockPolicyRepo()
	svc := NewPolicyService(pr, vr, testLogger())

	p := &models.Policy{EnterpriseID: uuid.New(), Name: "P", PolicyType: models.PolicyTypeWiFi, Platform: models.PlatformAll, PolicyConfig: models.JSONB{"ssid": "x"}}
	require.NoError(t, svc.Create(context.Background(), p, "admin"))

	versions, total, err := svc.ListVersions(context.Background(), p.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, versions, 1)
}

// ============================================================
// 7. Device action error paths
// ============================================================

type mockCmdRepoErr struct{ err error }

func (m *mockCmdRepoErr) Create(_ context.Context, _ *models.DeviceCommand) error { return m.err }

type mockDeviceRepoUpdateErr struct {
	mockDeviceRepo
	updateErr error
}

func (m *mockDeviceRepoUpdateErr) Update(_ context.Context, _ *models.Device) error {
	return m.updateErr
}

func TestDeviceService_Lock_CmdRepoError(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepoErr{err: fmt.Errorf("db down")}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	_, err := svc.Lock(context.Background(), d.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock command")
}

func TestDeviceService_Wipe_UpdateError(t *testing.T) {
	dr := &mockDeviceRepoUpdateErr{
		mockDeviceRepo: mockDeviceRepo{devices: map[uuid.UUID]*models.Device{}},
		updateErr:      fmt.Errorf("update failed"),
	}
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	_, err := svc.Wipe(context.Background(), d.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update device status")
}

func TestDeviceService_Delete_NotFound(t *testing.T) {
	dr := newMockDeviceRepo()
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	err := svc.Delete(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestDeviceService_Unenroll_UpdateError(t *testing.T) {
	dr := &mockDeviceRepoUpdateErr{
		mockDeviceRepo: mockDeviceRepo{devices: map[uuid.UUID]*models.Device{}},
		updateErr:      fmt.Errorf("update failed"),
	}
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	_, err := svc.Unenroll(context.Background(), d.ID)
	assert.Error(t, err)
}

// ============================================================
// 8. toBool float64 branch
// ============================================================

func TestToBool_Float64(t *testing.T) {
	assert.True(t, toBool(float64(1)))
	assert.False(t, toBool(float64(0)))
	assert.False(t, toBool("not a bool"))
}

// ============================================================
// 9. checkPolicy default case
// ============================================================

func TestCheckPolicy_UnknownType(t *testing.T) {
	svc := NewComplianceService(nil, nil, nil, nil, testLogger())
	d := &models.Device{PlatformData: models.JSONB{"x": true}}
	violations := svc.checkPolicy(d, "unknown_type", models.JSONB{"x": true})
	assert.Nil(t, violations)
}

// ============================================================
// 10. ComplianceCleanupHook error
// ============================================================

type mockComplianceRepoDeleteErr struct{ mockComplianceRepo }

func (m *mockComplianceRepoDeleteErr) DeleteByDevice(_ context.Context, _ uuid.UUID) error {
	return fmt.Errorf("delete failed")
}

func TestComplianceCleanupHook_Error(t *testing.T) {
	hook := NewComplianceCleanupHook(&mockComplianceRepoDeleteErr{}, testLogger())
	d := &models.Device{BaseModel: models.BaseModel{ID: uuid.New()}}

	assert.Error(t, hook.OnUnenroll(context.Background(), d))
	assert.Error(t, hook.OnWipe(context.Background(), d))
	assert.Error(t, hook.OnDelete(context.Background(), d))
}

// ============================================================
// 11. Lifecycle hook errors (all 3 methods)
// ============================================================

func TestLifecycleService_OnWipe_HookError(t *testing.T) {
	svc := NewLifecycleService(testLogger())
	svc.RegisterHook(&mockHook{err: fmt.Errorf("fail")})
	second := &mockHook{}
	svc.RegisterHook(second)
	svc.OnWipe(context.Background(), testDevice())
	assert.True(t, second.wipeCalled)
}

func TestLifecycleService_OnDelete_HookError(t *testing.T) {
	svc := NewLifecycleService(testLogger())
	svc.RegisterHook(&mockHook{err: fmt.Errorf("fail")})
	second := &mockHook{}
	svc.RegisterHook(second)
	svc.OnDelete(context.Background(), testDevice())
	assert.True(t, second.deleteCalled)
}

// ============================================================
// 12. PolicyService error paths
// ============================================================

type mockVersionRepoErr struct {
	createErr        error
	latestVersionErr error
	getByVersionErr  error
}

func (m *mockVersionRepoErr) Create(_ context.Context, _ *models.PolicyVersion) error {
	return m.createErr
}
func (m *mockVersionRepoErr) ListByPolicy(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.PolicyVersion, int, error) {
	return nil, 0, nil
}
func (m *mockVersionRepoErr) GetByVersion(_ context.Context, _ uuid.UUID, _ int) (*models.PolicyVersion, error) {
	return nil, m.getByVersionErr
}
func (m *mockVersionRepoErr) LatestVersion(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, m.latestVersionErr
}

func TestPolicyService_Create_VersionError(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := &mockVersionRepoErr{createErr: fmt.Errorf("version create failed")}
	svc := NewPolicyService(pr, vr, testLogger())

	p := &models.Policy{EnterpriseID: uuid.New(), Name: "P", PolicyType: models.PolicyTypeWiFi, Platform: models.PlatformAll, PolicyConfig: models.JSONB{"ssid": "x"}}
	// Create succeeds (version error is logged, not returned)
	err := svc.Create(context.Background(), p, "admin")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, p.ID)
}

func TestPolicyService_Update_LatestVersionError(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := &mockVersionRepoErr{latestVersionErr: fmt.Errorf("latest failed")}
	svc := NewPolicyService(pr, vr, testLogger())

	p := &models.Policy{EnterpriseID: uuid.New(), Name: "P", PolicyType: models.PolicyTypeWiFi, Platform: models.PlatformAll, PolicyConfig: models.JSONB{"ssid": "x"}}
	pr.Create(context.Background(), p)

	// Update succeeds; LatestVersion error falls back to 0
	err := svc.Update(context.Background(), p, "admin")
	require.NoError(t, err)
}

func TestPolicyService_Rollback_GetByVersionError(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := &mockVersionRepoErr{getByVersionErr: fmt.Errorf("not found")}
	svc := NewPolicyService(pr, vr, testLogger())

	_, err := svc.Rollback(context.Background(), uuid.New(), 1, "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPolicyService_CloneTemplate_NotTemplate(t *testing.T) {
	pr := newMockPolicyRepo()
	vr := newMockVersionRepo()
	svc := NewPolicyService(pr, vr, testLogger())

	p := &models.Policy{EnterpriseID: uuid.New(), Name: "Not Template", PolicyType: models.PolicyTypeWiFi, Platform: models.PlatformAll, PolicyConfig: models.JSONB{"ssid": "x"}, IsTemplate: false}
	require.NoError(t, svc.Create(context.Background(), p, "admin"))

	_, err := svc.CloneTemplate(context.Background(), p.ID, uuid.New(), "Clone", "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a template")
}

// ============================================================
// 13. TokenService edge cases (already mostly covered, add Validate with bad hash)
// ============================================================

func TestTokenService_Validate_BadToken(t *testing.T) {
	tr := newMockTokenRepo()
	ur := &mockUserRepoForToken{users: map[uuid.UUID]*models.User{}}
	svc := NewTokenService(tr, ur, testLogger())

	_, _, err := svc.Validate(context.Background(), "lmdm_nonexistent")
	assert.Error(t, err)
}

// ============================================================
// 14. UserService.Update optional fields
// ============================================================

func TestUserService_Update_IsActive(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, testLogger())
	u := &models.User{Email: "a@b.com", Role: models.RoleAdmin, EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), u))

	inactive := false
	updated, err := svc.Update(context.Background(), u.ID, nil, nil, &inactive)
	require.NoError(t, err)
	assert.False(t, updated.IsActive)
}

func TestUserService_Update_AllFields(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, testLogger())
	u := &models.User{Email: "a@b.com", Role: models.RoleAdmin, EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), u))

	name := "New Name"
	role := models.RoleOperator
	active := true
	updated, err := svc.Update(context.Background(), u.ID, &name, &role, &active)
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.FullName)
	assert.Equal(t, models.RoleOperator, updated.Role)
	assert.True(t, updated.IsActive)
}

// ============================================================
// 15. AppService.Update optional fields and Deploy error
// ============================================================

func TestAppService_Update_AllFields(t *testing.T) {
	ar := newMockAppRepo()
	svc := NewAppService(ar, newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), app))

	name := "Teams"
	ver := "3.0"
	it := models.AppInstallAvailable
	cfg := models.JSONB{"key": "val"}
	updated, err := svc.Update(context.Background(), app.ID, &name, &ver, &it, &cfg)
	require.NoError(t, err)
	assert.Equal(t, "Teams", updated.Name)
	assert.Equal(t, "3.0", updated.Version)
	assert.Equal(t, models.AppInstallAvailable, updated.InstallType)
	assert.Equal(t, "val", updated.AppConfig["key"])
}

func TestAppService_Update_NilFields(t *testing.T) {
	ar := newMockAppRepo()
	svc := NewAppService(ar, newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New(), Version: "1.0"}
	require.NoError(t, svc.Create(context.Background(), app))

	updated, err := svc.Update(context.Background(), app.ID, nil, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "Slack", updated.Name)
	assert.Equal(t, "1.0", updated.Version)
}

func TestAppService_Deploy_CmdRepoError(t *testing.T) {
	ar := newMockAppRepo()
	dr := newMockDeviceRepo()
	svc := NewAppService(ar, dr, &mockCmdRepoErr{err: fmt.Errorf("db down")}, &mockDispatcher{}, testLogger())

	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), app))

	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d

	result, err := svc.Deploy(context.Background(), app.ID, []uuid.UUID{d.ID})
	require.NoError(t, err) // Deploy logs errors per-device, doesn't fail
	assert.Empty(t, result.Commands)
}
