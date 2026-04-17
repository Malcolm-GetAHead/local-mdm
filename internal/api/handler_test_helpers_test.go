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
	"github.com/malcolm-getahead/local-mdm/internal/scep"
)

// --- Mock Repositories ---

type mockEnterpriseRepo struct {
	enterprises []*models.Enterprise
	createErr   error
	getErr      error
	listErr     error
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

func (m *mockEnterpriseRepo) Update(_ context.Context, e *models.Enterprise) error { return nil }
func (m *mockEnterpriseRepo) Delete(_ context.Context, id uuid.UUID) error        { return nil }

type mockDeviceRepo struct {
	devices   []*models.Device
	createErr error
	getErr    error
	listErr   error
	updateErr error
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

func (m *mockDeviceRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

type mockPolicyRepo struct {
	policies  []*models.Policy
	createErr error
	getErr    error
	listErr   error
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

func (m *mockPolicyRepo) Update(_ context.Context, _ *models.Policy) error { return nil }
func (m *mockPolicyRepo) Delete(_ context.Context, _ uuid.UUID) error      { return nil }
func (m *mockPolicyRepo) AssignToDevice(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *mockPolicyRepo) UnassignFromDevice(_ context.Context, _, _ uuid.UUID) error {
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

type mockAuditLogger struct {
	events []audit.Event
}

func (m *mockAuditLogger) Log(_ context.Context, event audit.Event) error {
	m.events = append(m.events, event)
	return nil
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
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	er := &mockEnterpriseRepo{}
	dr := &mockDeviceRepo{}
	pr := &mockPolicyRepo{}
	cr := &mockCertRepo{}
	ar := &mockAuditLogRepo{}
	al := &mockAuditLogger{}

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
		enrollmentLimiter: newRateLimiterWithSize(10, time.Minute, 100),
	}

	// Register only the routes we're testing (no auth middleware)
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/enterprises", s.handleListEnterprises).Methods("GET")
	api.HandleFunc("/enterprises", s.handleCreateEnterprise).Methods("POST")
	api.HandleFunc("/enterprises/{id}", s.handleGetEnterprise).Methods("GET")
	api.HandleFunc("/devices", s.handleListDevices).Methods("GET")
	api.HandleFunc("/devices/{id}", s.handleGetDevice).Methods("GET")
	api.HandleFunc("/devices/{id}/lock", s.handleLockDevice).Methods("POST")
	api.HandleFunc("/devices/{id}/wipe", s.handleWipeDevice).Methods("POST")
	api.HandleFunc("/policies", s.handleListPolicies).Methods("GET")
	api.HandleFunc("/policies", s.handleCreatePolicy).Methods("POST")
	api.HandleFunc("/policies/{id}", s.handleGetPolicy).Methods("GET")
	api.HandleFunc("/certificates", s.handleListCertificates).Methods("GET")
	api.HandleFunc("/audit-logs", s.handleListAuditLogs).Methods("GET")
	api.HandleFunc("/android/webhook", s.handleAndroidWebhook).Methods("POST")
	api.HandleFunc("/macos/enroll/{enterprise_id}", s.handleMacOSEnrollmentProfile).Methods("GET")

	return &testServer{
		server:         s,
		enterpriseRepo: er,
		deviceRepo:     dr,
		policyRepo:     pr,
		certRepo:       cr,
		auditLogRepo:   ar,
		auditLogger:    al,
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
