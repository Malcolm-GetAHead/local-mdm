package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/audit"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/scep"
	depClient "github.com/micromdm/nanodep/client"
)

// --- Mock Repositories ---

type mockEnterpriseRepo struct {
	enterprises []*models.Enterprise
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	deleteErr   error
}

func (m *mockEnterpriseRepo) Create(_ context.Context, e *models.Enterprise) error {
	if m.createErr != nil {
		return m.createErr
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	m.enterprises = append(m.enterprises, e)
	return nil
}

func (m *mockEnterpriseRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Enterprise, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, e := range m.enterprises {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, fmt.Errorf("enterprise not found")
}

func (m *mockEnterpriseRepo) GetBySlug(_ context.Context, slug string) (*models.Enterprise, error) {
	for _, e := range m.enterprises {
		if e.Slug == slug {
			return e, nil
		}
	}
	return nil, fmt.Errorf("enterprise not found")
}

func (m *mockEnterpriseRepo) List(_ context.Context, limit, offset int) ([]*models.Enterprise, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	total := len(m.enterprises)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.Enterprise{}, total, nil
	}
	return m.enterprises[offset:end], total, nil
}

func (m *mockEnterpriseRepo) Update(_ context.Context, e *models.Enterprise) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i, existing := range m.enterprises {
		if existing.ID == e.ID {
			m.enterprises[i] = e
			return nil
		}
	}
	return fmt.Errorf("enterprise not found")
}
func (m *mockEnterpriseRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, e := range m.enterprises {
		if e.ID == id {
			m.enterprises = append(m.enterprises[:i], m.enterprises[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("enterprise not found")
}

type mockDeviceRepo struct {
	devices   []*models.Device
	createErr error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error
}

func (m *mockDeviceRepo) Create(_ context.Context, d *models.Device) error {
	if m.createErr != nil {
		return m.createErr
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	m.devices = append(m.devices, d)
	return nil
}

func (m *mockDeviceRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Device, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, d := range m.devices {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, fmt.Errorf("device not found")
}

func (m *mockDeviceRepo) GetBySerial(_ context.Context, _ uuid.UUID, _ string) (*models.Device, error) {
	return nil, fmt.Errorf("device not found")
}

func (m *mockDeviceRepo) List(_ context.Context, _ uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	total := len(m.devices)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.Device{}, total, nil
	}
	return m.devices[offset:end], total, nil
}

func (m *mockDeviceRepo) Update(_ context.Context, d *models.Device) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i, existing := range m.devices {
		if existing.ID == d.ID {
			m.devices[i] = d
			return nil
		}
	}
	return fmt.Errorf("device not found")
}

func (m *mockDeviceRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, d := range m.devices {
		if d.ID == id {
			m.devices = append(m.devices[:i], m.devices[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("device not found")
}

type mockPolicyRepo struct {
	policies    []*models.Policy
	assignments map[string]bool // "deviceID:policyID" -> true
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	deleteErr   error
	assignErr   error
}

func (m *mockPolicyRepo) Create(_ context.Context, p *models.Policy) error {
	if m.createErr != nil {
		return m.createErr
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	m.policies = append(m.policies, p)
	return nil
}

func (m *mockPolicyRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Policy, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, p := range m.policies {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("policy not found")
}

func (m *mockPolicyRepo) List(_ context.Context, _ uuid.UUID, limit, offset int) ([]*models.Policy, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	total := len(m.policies)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.Policy{}, total, nil
	}
	return m.policies[offset:end], total, nil
}

func (m *mockPolicyRepo) Update(_ context.Context, p *models.Policy) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i, existing := range m.policies {
		if existing.ID == p.ID {
			m.policies[i] = p
			return nil
		}
	}
	return fmt.Errorf("policy not found")
}
func (m *mockPolicyRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, p := range m.policies {
		if p.ID == id {
			m.policies = append(m.policies[:i], m.policies[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("policy not found")
}
func (m *mockPolicyRepo) AssignToDevice(_ context.Context, deviceID, policyID uuid.UUID) error {
	if m.assignErr != nil {
		return m.assignErr
	}
	if m.assignments == nil {
		m.assignments = make(map[string]bool)
	}
	m.assignments[deviceID.String()+":"+policyID.String()] = true
	return nil
}
func (m *mockPolicyRepo) UnassignFromDevice(_ context.Context, deviceID, policyID uuid.UUID) error {
	if m.assignments != nil {
		delete(m.assignments, deviceID.String()+":"+policyID.String())
	}
	return nil
}

type mockCertRepo struct {
	certs   []*models.Certificate
	listErr error
}

func (m *mockCertRepo) GetBySerial(_ context.Context, serial string) (*models.Certificate, error) {
	for _, c := range m.certs {
		if c.SerialNumber == serial {
			return c, nil
		}
	}
	return nil, fmt.Errorf("certificate not found")
}

func (m *mockCertRepo) List(_ context.Context, _ *uuid.UUID, limit, offset int) ([]*models.Certificate, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	total := len(m.certs)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.Certificate{}, total, nil
	}
	return m.certs[offset:end], total, nil
}

type mockAuditLogRepo struct {
	logs    []*models.AuditLog
	listErr error
}

func (m *mockAuditLogRepo) List(_ context.Context, _ uuid.UUID, limit, offset int) ([]*models.AuditLog, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	total := len(m.logs)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.AuditLog{}, total, nil
	}
	return m.logs[offset:end], total, nil
}

type mockCommandRepo struct {
	commands []*models.DeviceCommand
	createErr error
	getErr    error
	listErr   error
}

func (m *mockCommandRepo) Create(_ context.Context, cmd *models.DeviceCommand) error {
	if m.createErr != nil {
		return m.createErr
	}
	if cmd.ID == uuid.Nil {
		cmd.ID = uuid.New()
	}
	if cmd.Status == "" {
		cmd.Status = "pending"
	}
	cmd.CreatedAt = time.Now()
	cmd.UpdatedAt = time.Now()
	m.commands = append(m.commands, cmd)
	return nil
}

func (m *mockCommandRepo) GetByID(_ context.Context, id uuid.UUID) (*models.DeviceCommand, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, c := range m.commands {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("command not found")
}

func (m *mockCommandRepo) ListPending(_ context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error) {
	var result []*models.DeviceCommand
	for _, c := range m.commands {
		if c.DeviceID == deviceID && c.Status == "pending" {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCommandRepo) ListByDevice(_ context.Context, deviceID uuid.UUID, limit, offset int) ([]*models.DeviceCommand, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var all []*models.DeviceCommand
	for _, c := range m.commands {
		if c.DeviceID == deviceID {
			all = append(all, c)
		}
	}
	total := len(all)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.DeviceCommand{}, total, nil
	}
	return all[offset:end], total, nil
}

func (m *mockCommandRepo) MarkSent(_ context.Context, id uuid.UUID) error {
	for _, c := range m.commands {
		if c.ID == id {
			c.Status = "sent"
			return nil
		}
	}
	return fmt.Errorf("command not found")
}

func (m *mockCommandRepo) MarkCompleted(_ context.Context, id uuid.UUID) error {
	for _, c := range m.commands {
		if c.ID == id {
			c.Status = "completed"
			return nil
		}
	}
	return fmt.Errorf("command not found")
}

func (m *mockCommandRepo) MarkFailed(_ context.Context, id uuid.UUID, errMsg string) error {
	for _, c := range m.commands {
		if c.ID == id {
			c.Status = "failed"
			c.ErrorMessage = errMsg
			return nil
		}
	}
	return fmt.Errorf("command not found")
}

type mockAuditLogger struct {
	events []audit.Event
}

func (m *mockAuditLogger) Log(_ context.Context, event audit.Event) error {
	m.events = append(m.events, event)
	return nil
}

type mockAppRepo struct {
	apps      []*models.App
	createErr error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error
}

func (m *mockAppRepo) Create(_ context.Context, app *models.App) error {
	if m.createErr != nil {
		return m.createErr
	}
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	m.apps = append(m.apps, app)
	return nil
}

func (m *mockAppRepo) GetByID(_ context.Context, id uuid.UUID) (*models.App, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, a := range m.apps {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, fmt.Errorf("app not found")
}

func (m *mockAppRepo) List(_ context.Context, _ uuid.UUID, limit, offset int) ([]*models.App, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	total := len(m.apps)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.App{}, total, nil
	}
	return m.apps[offset:end], total, nil
}

func (m *mockAppRepo) Update(_ context.Context, app *models.App) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i, a := range m.apps {
		if a.ID == app.ID {
			m.apps[i] = app
			return nil
		}
	}
	return fmt.Errorf("app not found")
}

func (m *mockAppRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, a := range m.apps {
		if a.ID == id {
			m.apps = append(m.apps[:i], m.apps[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("app not found")
}

// --- Test Helper ---

type testServer struct {
	server         *Server
	enterpriseRepo *mockEnterpriseRepo
	deviceRepo     *mockDeviceRepo
	policyRepo     *mockPolicyRepo
	certRepo       *mockCertRepo
	auditLogRepo   *mockAuditLogRepo
	auditLogger    *mockAuditLogger
	commandRepo    *mockCommandRepo
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	er := &mockEnterpriseRepo{}
	dr := &mockDeviceRepo{}
	pr := &mockPolicyRepo{}
	cr := &mockCertRepo{}
	ar := &mockAuditLogRepo{}
	al := &mockAuditLogger{}
	cmdr := &mockCommandRepo{}
	appr := &mockAppRepo{}

	s := &Server{
		router:           mux.NewRouter(),
		config:           &config.Config{},
		logger:           slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError})),
		auditLogger:      al,
		challengeManager: scep.NewChallengeManager(),
		enterpriseRepo:   er,
		deviceRepo:       dr,
		policyRepo:       pr,
		certRepo:         cr,
		auditLogRepo:     ar,
		cmdRepo:          cmdr,
		appRepo:          appr,
		enrollmentLimiter: newRateLimiterWithSize(10, time.Minute, 100),
		depService:       macos.NewDEPService(&testDEPStorage{}, slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))),
	}

	// Register only the routes we're testing (no auth middleware)
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/enterprises", s.handleListEnterprises).Methods("GET")
	api.HandleFunc("/enterprises", s.handleCreateEnterprise).Methods("POST")
	api.HandleFunc("/enterprises/{id}", s.handleGetEnterprise).Methods("GET")
	api.HandleFunc("/enterprises/{id}", s.handleUpdateEnterprise).Methods("PUT")
	api.HandleFunc("/enterprises/{id}", s.handleDeleteEnterprise).Methods("DELETE")
	api.HandleFunc("/devices", s.handleListDevices).Methods("GET")
	api.HandleFunc("/devices/{id}", s.handleGetDevice).Methods("GET")
	api.HandleFunc("/devices/{id}", s.handleUpdateDevice).Methods("PUT")
	api.HandleFunc("/devices/{id}", s.handleDeleteDevice).Methods("DELETE")
	api.HandleFunc("/devices/{id}/lock", s.handleLockDevice).Methods("POST")
	api.HandleFunc("/devices/{id}/wipe", s.handleWipeDevice).Methods("POST")
	api.HandleFunc("/devices/{id}/restart", s.handleRestartDevice).Methods("POST")
	api.HandleFunc("/policies", s.handleListPolicies).Methods("GET")
	api.HandleFunc("/policies", s.handleCreatePolicy).Methods("POST")
	api.HandleFunc("/policies/{id}", s.handleGetPolicy).Methods("GET")
	api.HandleFunc("/policies/{id}", s.handleUpdatePolicy).Methods("PUT")
	api.HandleFunc("/policies/{id}", s.handleDeletePolicy).Methods("DELETE")
	api.HandleFunc("/policies/{id}/assign", s.handleAssignPolicy).Methods("POST")
	api.HandleFunc("/policies/{id}/assign/{device_id}", s.handleUnassignPolicy).Methods("DELETE")
	api.HandleFunc("/certificates", s.handleListCertificates).Methods("GET")
	api.HandleFunc("/audit-logs", s.handleListAuditLogs).Methods("GET")
	api.HandleFunc("/android/webhook", s.handleAndroidWebhook).Methods("POST")
	api.HandleFunc("/macos/enroll/{enterprise_id}", s.handleMacOSEnrollmentProfile).Methods("GET")
	api.HandleFunc("/dep/{name}/tokenpki", s.handleDEPTokenPKI).Methods("GET", "PUT")
	api.HandleFunc("/dep/{name}/assigner", s.handleDEPAssignerProfile).Methods("GET", "PUT")
	api.HandleFunc("/dep/{name}/devices", s.handleDEPDevices).Methods("GET")

	// Sprint 3: Command and profile routes
	api.HandleFunc("/devices/{id}/commands", s.handleSendCommand).Methods("POST")
	api.HandleFunc("/devices/{id}/commands", s.handleListCommands).Methods("GET")
	api.HandleFunc("/devices/{id}/profiles", s.handleInstallProfile).Methods("POST")
	api.HandleFunc("/devices/{id}/profiles/{profile_id}", s.handleRemoveProfile).Methods("DELETE")

	// Sprint 3: App management routes
	api.HandleFunc("/apps", s.handleListApps).Methods("GET")
	api.HandleFunc("/apps", s.handleCreateApp).Methods("POST")
	api.HandleFunc("/apps/{id}", s.handleGetApp).Methods("GET")
	api.HandleFunc("/apps/{id}", s.handleUpdateApp).Methods("PUT")
	api.HandleFunc("/apps/{id}", s.handleDeleteApp).Methods("DELETE")
	api.HandleFunc("/apps/{id}/deploy", s.handleDeployApp).Methods("POST")

	return &testServer{
		server:         s,
		enterpriseRepo: er,
		deviceRepo:     dr,
		policyRepo:     pr,
		certRepo:       cr,
		auditLogRepo:   ar,
		auditLogger:    al,
		commandRepo:    cmdr,
	}
}

// do executes a request against the test server and returns the recorder
func (ts *testServer) do(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	ts.server.router.ServeHTTP(w, req)
	return w
}

// doWithAuth executes a request with an authenticated user in context
func (ts *testServer) doWithAuth(req *http.Request, user *auth.AuthUser) *httptest.ResponseRecorder {
	ctx := auth.WithUser(req.Context(), user)
	return ts.do(req.WithContext(ctx))
}

func testUser() *auth.AuthUser {
	return &auth.AuthUser{
		ID:           uuid.New().String(),
		Email:        "admin@test.com",
		Roles:        []string{"admin"},
		EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func jsonBody(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// testDEPStorage is a minimal in-memory DEP storage for handler tests
type testDEPStorage struct {
	profile    string
	modTime    time.Time
}

func (s *testDEPStorage) RetrieveAuthTokens(_ context.Context, _ string) (*depClient.OAuth1Tokens, error) {
	return &depClient.OAuth1Tokens{}, nil
}
func (s *testDEPStorage) StoreAuthTokens(_ context.Context, _ string, _ *depClient.OAuth1Tokens) error {
	return nil
}
func (s *testDEPStorage) RetrieveConfig(_ context.Context, _ string) (*depClient.Config, error) {
	return &depClient.Config{}, nil
}
func (s *testDEPStorage) StoreConfig(_ context.Context, _ string, _ *depClient.Config) error {
	return nil
}
func (s *testDEPStorage) RetrieveCursor(_ context.Context, _ string) (string, error) { return "", nil }
func (s *testDEPStorage) StoreCursor(_ context.Context, _, _ string) error           { return nil }
func (s *testDEPStorage) RetrieveAssignerProfile(_ context.Context, _ string) (string, time.Time, error) {
	return s.profile, s.modTime, nil
}
func (s *testDEPStorage) StoreAssignerProfile(_ context.Context, _ string, p string) error {
	s.profile = p
	s.modTime = time.Now()
	return nil
}
func (s *testDEPStorage) GenerateTokenPKI(_ context.Context, _, _ string, _ int) ([]byte, error) {
	return []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"), nil
}
func (s *testDEPStorage) RetrieveCurrentTokenPKI(_ context.Context, _ string) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (s *testDEPStorage) RetrieveStagingTokenPKI(_ context.Context, _ string) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (s *testDEPStorage) UpstageTokenPKI(_ context.Context, _ string) error { return nil }
func (s *testDEPStorage) StoreSyncedDevice(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return nil
}
func (s *testDEPStorage) ListDEPDevices(_ context.Context, _ string, _, _ int) ([]macos.DEPDevice, int, error) {
	return []macos.DEPDevice{}, 0, nil
}
