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

type mockAppRepo struct {
	apps map[uuid.UUID]*models.App
}

func newMockAppRepo() *mockAppRepo { return &mockAppRepo{apps: map[uuid.UUID]*models.App{}} }

func (m *mockAppRepo) Create(_ context.Context, a *models.App) error {
	a.ID = uuid.New()
	m.apps[a.ID] = a
	return nil
}
func (m *mockAppRepo) GetByID(_ context.Context, id uuid.UUID) (*models.App, error) {
	if a, ok := m.apps[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("app not found")
}
func (m *mockAppRepo) List(_ context.Context, eid uuid.UUID, limit, offset int) ([]*models.App, int, error) {
	var out []*models.App
	for _, a := range m.apps {
		if a.EnterpriseID == eid {
			out = append(out, a)
		}
	}
	return out, len(out), nil
}
func (m *mockAppRepo) Update(_ context.Context, a *models.App) error {
	m.apps[a.ID] = a
	return nil
}
func (m *mockAppRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.apps[id]; !ok {
		return fmt.Errorf("app not found")
	}
	delete(m.apps, id)
	return nil
}

func TestAppService_Create(t *testing.T) {
	svc := NewAppService(newMockAppRepo(), newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New()}
	err := svc.Create(context.Background(), app)
	require.NoError(t, err)
	assert.Equal(t, models.AppInstallRequired, app.InstallType)
}

func TestAppService_Create_Validation(t *testing.T) {
	svc := NewAppService(newMockAppRepo(), newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	err := svc.Create(context.Background(), &models.App{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestAppService_GetAndList(t *testing.T) {
	ar := newMockAppRepo()
	svc := NewAppService(ar, newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	eid := uuid.New()
	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: eid}
	require.NoError(t, svc.Create(context.Background(), app))

	got, err := svc.Get(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, "Slack", got.Name)

	list, total, err := svc.List(context.Background(), eid, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)
}

func TestAppService_Update(t *testing.T) {
	ar := newMockAppRepo()
	svc := NewAppService(ar, newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), app))

	newVer := "2.0"
	updated, err := svc.Update(context.Background(), app.ID, nil, &newVer, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "2.0", updated.Version)
}

func TestAppService_Delete(t *testing.T) {
	ar := newMockAppRepo()
	svc := NewAppService(ar, newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), app))

	err := svc.Delete(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Empty(t, ar.apps)
}

func TestAppService_Deploy(t *testing.T) {
	ar := newMockAppRepo()
	dr := newMockDeviceRepo()
	cr := &mockCmdRepo{}
	disp := &mockDispatcher{}
	svc := NewAppService(ar, dr, cr, disp, testLogger())

	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), app))

	d1 := newTestDevice(models.PlatformMacOS)
	d2 := newTestDevice(models.PlatformMacOS)
	dr.devices[d1.ID] = d1
	dr.devices[d2.ID] = d2

	result, err := svc.Deploy(context.Background(), app.ID, []uuid.UUID{d1.ID, d2.ID})
	require.NoError(t, err)
	assert.Equal(t, app.ID, result.App.ID)
	assert.Len(t, result.Commands, 2)
	assert.Len(t, disp.enqueued, 2)
}

func TestAppService_Deploy_EmptyDevices(t *testing.T) {
	ar := newMockAppRepo()
	svc := NewAppService(ar, newMockDeviceRepo(), &mockCmdRepo{}, &mockDispatcher{}, testLogger())
	app := &models.App{Name: "Slack", Platform: models.PlatformMacOS, Identifier: "com.slack", EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), app))

	_, err := svc.Deploy(context.Background(), app.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}
