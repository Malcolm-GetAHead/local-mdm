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

// --- Mock repos ---

type mockGroupRepo struct {
	groups  map[uuid.UUID]*models.DeviceGroup
	members map[uuid.UUID][]uuid.UUID // groupID -> deviceIDs
}

func newMockGroupRepo() *mockGroupRepo {
	return &mockGroupRepo{
		groups:  make(map[uuid.UUID]*models.DeviceGroup),
		members: make(map[uuid.UUID][]uuid.UUID),
	}
}

func (m *mockGroupRepo) Create(_ context.Context, g *models.DeviceGroup) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	m.groups[g.ID] = g
	return nil
}
func (m *mockGroupRepo) GetByID(_ context.Context, id uuid.UUID) (*models.DeviceGroup, error) {
	if g, ok := m.groups[id]; ok {
		return g, nil
	}
	return nil, fmt.Errorf("group not found")
}
func (m *mockGroupRepo) List(_ context.Context, eid uuid.UUID, limit, offset int) ([]*models.DeviceGroup, int, error) {
	var result []*models.DeviceGroup
	for _, g := range m.groups {
		if g.EnterpriseID == eid {
			result = append(result, g)
		}
	}
	return result, len(result), nil
}
func (m *mockGroupRepo) Update(_ context.Context, g *models.DeviceGroup) error {
	if _, ok := m.groups[g.ID]; !ok {
		return fmt.Errorf("group not found")
	}
	m.groups[g.ID] = g
	return nil
}
func (m *mockGroupRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.groups[id]; !ok {
		return fmt.Errorf("group not found")
	}
	delete(m.groups, id)
	return nil
}
func (m *mockGroupRepo) AddMember(_ context.Context, groupID, deviceID uuid.UUID) error {
	m.members[groupID] = append(m.members[groupID], deviceID)
	return nil
}
func (m *mockGroupRepo) RemoveMember(_ context.Context, groupID, deviceID uuid.UUID) error {
	devs := m.members[groupID]
	for i, d := range devs {
		if d == deviceID {
			m.members[groupID] = append(devs[:i], devs[i+1:]...)
			return nil
		}
	}
	return nil
}
func (m *mockGroupRepo) ListMembers(_ context.Context, groupID uuid.UUID, _, _ int) ([]*models.Device, int, error) {
	var devices []*models.Device
	for _, did := range m.members[groupID] {
		devices = append(devices, &models.Device{BaseModel: models.BaseModel{ID: did}})
	}
	return devices, len(devices), nil
}
func (m *mockGroupRepo) ListGroupsForDevice(_ context.Context, deviceID uuid.UUID) ([]*models.DeviceGroup, error) {
	var result []*models.DeviceGroup
	for gid, devs := range m.members {
		for _, d := range devs {
			if d == deviceID {
				if g, ok := m.groups[gid]; ok {
					result = append(result, g)
				}
			}
		}
	}
	return result, nil
}

type mockAssignmentRepo struct {
	assignments []*models.PolicyAssignment
}

func (m *mockAssignmentRepo) Create(_ context.Context, a *models.PolicyAssignment) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	m.assignments = append(m.assignments, a)
	return nil
}
func (m *mockAssignmentRepo) Delete(_ context.Context, id uuid.UUID) error {
	for i, a := range m.assignments {
		if a.ID == id {
			m.assignments = append(m.assignments[:i], m.assignments[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("assignment not found")
}
func (m *mockAssignmentRepo) ListByTarget(_ context.Context, tt string, tid uuid.UUID) ([]*models.PolicyAssignment, error) {
	var result []*models.PolicyAssignment
	for _, a := range m.assignments {
		if a.TargetType == tt && a.TargetID == tid {
			result = append(result, a)
		}
	}
	return result, nil
}
func (m *mockAssignmentRepo) ListByPolicy(_ context.Context, pid uuid.UUID) ([]*models.PolicyAssignment, error) {
	var result []*models.PolicyAssignment
	for _, a := range m.assignments {
		if a.PolicyID == pid {
			result = append(result, a)
		}
	}
	return result, nil
}
func (m *mockAssignmentRepo) GetEffectivePolicies(_ context.Context, deviceID uuid.UUID, groupIDs []uuid.UUID, enterpriseID uuid.UUID) ([]*models.PolicyAssignment, error) {
	var result []*models.PolicyAssignment
	for _, a := range m.assignments {
		if a.TargetType == models.TargetTypeDevice && a.TargetID == deviceID {
			result = append(result, a)
		}
		if a.TargetType == models.TargetTypeEnterprise && a.TargetID == enterpriseID {
			result = append(result, a)
		}
		for _, gid := range groupIDs {
			if a.TargetType == models.TargetTypeGroup && a.TargetID == gid {
				result = append(result, a)
			}
		}
	}
	return result, nil
}

// --- GroupService Tests ---

func TestGroupService_CRUD(t *testing.T) {
	gr := newMockGroupRepo()
	svc := NewGroupService(gr, &mockAssignmentRepo{}, testLogger())

	eid := uuid.New()
	group := &models.DeviceGroup{EnterpriseID: eid, Name: "Engineering", Description: "Eng team"}

	// Create
	err := svc.CreateGroup(context.Background(), group)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, group.ID)

	// Get
	got, err := svc.GetGroup(context.Background(), group.ID)
	require.NoError(t, err)
	assert.Equal(t, "Engineering", got.Name)

	// Update
	group.Name = "Platform Engineering"
	err = svc.UpdateGroup(context.Background(), group)
	require.NoError(t, err)

	got, _ = svc.GetGroup(context.Background(), group.ID)
	assert.Equal(t, "Platform Engineering", got.Name)

	// List
	groups, total, err := svc.ListGroups(context.Background(), eid, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, groups, 1)

	// Delete
	err = svc.DeleteGroup(context.Background(), group.ID)
	require.NoError(t, err)

	_, err = svc.GetGroup(context.Background(), group.ID)
	assert.Error(t, err)
}

func TestGroupService_Membership(t *testing.T) {
	gr := newMockGroupRepo()
	svc := NewGroupService(gr, &mockAssignmentRepo{}, testLogger())

	group := &models.DeviceGroup{EnterpriseID: uuid.New(), Name: "Test"}
	require.NoError(t, svc.CreateGroup(context.Background(), group))

	d1, d2 := uuid.New(), uuid.New()

	// Add members
	require.NoError(t, svc.AddMember(context.Background(), group.ID, d1))
	require.NoError(t, svc.AddMember(context.Background(), group.ID, d2))

	// List members
	devices, total, err := svc.ListMembers(context.Background(), group.ID, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, devices, 2)

	// Remove member
	require.NoError(t, svc.RemoveMember(context.Background(), group.ID, d1))

	devices, total, _ = svc.ListMembers(context.Background(), group.ID, 100, 0)
	assert.Equal(t, 1, total)
	assert.Len(t, devices, 1)
}

func TestGroupService_AssignPolicy(t *testing.T) {
	ar := &mockAssignmentRepo{}
	svc := NewGroupService(newMockGroupRepo(), ar, testLogger())

	policyID := uuid.New()
	groupID := uuid.New()

	// Assign
	a, err := svc.AssignPolicy(context.Background(), policyID, models.TargetTypeGroup, groupID, 10)
	require.NoError(t, err)
	assert.Equal(t, policyID, a.PolicyID)
	assert.Equal(t, models.TargetTypeGroup, a.TargetType)
	assert.Equal(t, 10, a.Priority)

	// List by policy
	assignments, err := svc.ListAssignmentsByPolicy(context.Background(), policyID)
	require.NoError(t, err)
	assert.Len(t, assignments, 1)

	// Unassign
	err = svc.UnassignPolicy(context.Background(), a.ID)
	require.NoError(t, err)

	assignments, _ = svc.ListAssignmentsByPolicy(context.Background(), policyID)
	assert.Len(t, assignments, 0)
}

func TestGroupService_AssignPolicy_InvalidTargetType(t *testing.T) {
	svc := NewGroupService(newMockGroupRepo(), &mockAssignmentRepo{}, testLogger())

	_, err := svc.AssignPolicy(context.Background(), uuid.New(), "invalid", uuid.New(), 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid target_type")
}

func TestGroupService_EffectivePolicies(t *testing.T) {
	gr := newMockGroupRepo()
	ar := &mockAssignmentRepo{}
	svc := NewGroupService(gr, ar, testLogger())

	eid := uuid.New()
	deviceID := uuid.New()

	// Create a group and add device
	group := &models.DeviceGroup{EnterpriseID: eid, Name: "Eng"}
	require.NoError(t, svc.CreateGroup(context.Background(), group))
	require.NoError(t, svc.AddMember(context.Background(), group.ID, deviceID))

	// Assign policies at different levels
	p1, p2, p3 := uuid.New(), uuid.New(), uuid.New()
	svc.AssignPolicy(context.Background(), p1, models.TargetTypeDevice, deviceID, 1)
	svc.AssignPolicy(context.Background(), p2, models.TargetTypeGroup, group.ID, 10)
	svc.AssignPolicy(context.Background(), p3, models.TargetTypeEnterprise, eid, 100)

	// Get effective — should include all three
	effective, err := svc.GetEffectivePolicies(context.Background(), deviceID, eid)
	require.NoError(t, err)
	assert.Len(t, effective, 3)
}
