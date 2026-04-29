package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Windows Discovery Handler Tests ---

func TestHandleWindowsDiscoveryService_SOAPRequest(t *testing.T) {
	soapBody := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:a="http://www.w3.org/2005/08/addressing">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/Discover</a:Action>
    <a:MessageID>urn:uuid:test-message-id-001</a:MessageID>
    <a:To s:mustUnderstand="1">https://enterpriseenrollment.localmdm.local/EnrollmentServer/Discovery.svc</a:To>
  </s:Header>
  <s:Body>
    <Discover xmlns="http://schemas.microsoft.com/windows/management/2012/01/enrollment">
      <request xmlns:i="http://www.w3.org/2001/XMLSchema-instance">
        <EmailAddress>admin@localmdm.local</EmailAddress>
        <RequestVersion>4.0</RequestVersion>
        <DeviceType>CIMClient_Windows</DeviceType>
        <ApplicationVersion>10.0.19045.0</ApplicationVersion>
        <OSEdition>72</OSEdition>
      </request>
    </Discover>
  </s:Body>
</s:Envelope>`

	t.Run("returns SOAP discovery response", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/EnrollmentServer/Discovery.svc", ts.server.handleWindowsDiscoveryService).Methods("GET", "POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/Discovery.svc", strings.NewReader(soapBody))
		req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/soap+xml; charset=utf-8", w.Header().Get("Content-Type"))
		assert.NotEmpty(t, w.Header().Get("Content-Length"))
		body := w.Body.String()
		assert.Contains(t, body, "DiscoverResponse")
		assert.Contains(t, body, "EnrollmentServiceUrl")
		assert.Contains(t, body, "EnrollmentPolicyServiceUrl")
		assert.Contains(t, body, "Enrollment.svc")
		assert.Contains(t, body, "Policy.svc")
	})

	t.Run("includes enterprise_id in enrollment URL when present", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/EnrollmentServer/{enterprise_id}/Discovery.svc", ts.server.handleWindowsDiscoveryService).Methods("GET", "POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/00000000-0000-0000-0000-000000000001/Discovery.svc", strings.NewReader(soapBody))
		req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "00000000-0000-0000-0000-000000000001/Enrollment.svc")
	})

	t.Run("returns 400 for invalid SOAP", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/EnrollmentServer/Discovery.svc", ts.server.handleWindowsDiscoveryService).Methods("GET", "POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/Discovery.svc", strings.NewReader("<not-valid-soap/>"))
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Windows Policy Handler Tests ---

func TestHandleWindowsPolicyService_SOAPRequest(t *testing.T) {
	soapBody := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:a="http://www.w3.org/2005/08/addressing">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy/IPolicy/GetPolicies</a:Action>
    <a:MessageID>urn:uuid:policy-msg-001</a:MessageID>
  </s:Header>
  <s:Body>
    <GetPolicies xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy">
      <client>
        <lastUpdate xsi:nil="true" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"/>
        <preferredLanguage xsi:nil="true" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"/>
      </client>
      <requestFilter xsi:nil="true" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"/>
    </GetPolicies>
  </s:Body>
</s:Envelope>`

	t.Run("returns policy with RelatesTo matching MessageID", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/EnrollmentServer/Policy.svc", ts.server.handleWindowsPolicyService).Methods("POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/Policy.svc", strings.NewReader(soapBody))
		req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "GetPoliciesResponse")
		assert.Contains(t, body, "urn:uuid:policy-msg-001")
		assert.Contains(t, body, "LocalMDMEnrollment")
		assert.Contains(t, body, "2048") // minimalKeyLength
		assert.NotEmpty(t, w.Header().Get("Content-Length"))
	})
}

// --- Windows Enrollment Handler Tests ---

func TestHandleWindowsEnrollmentService(t *testing.T) {
	t.Run("returns 400 for empty body", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/EnrollmentServer/Enrollment.svc", ts.server.handleWindowsEnrollmentService).Methods("POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/Enrollment.svc", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid SOAP", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/EnrollmentServer/Enrollment.svc", ts.server.handleWindowsEnrollmentService).Methods("POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/Enrollment.svc", strings.NewReader("<garbage/>"))
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 503 when certService is nil", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.certService = nil
		ts.server.router.HandleFunc("/EnrollmentServer/Enrollment.svc", ts.server.handleWindowsEnrollmentService).Methods("POST")

		body := buildEnrollmentSOAP(t)
		req := httptest.NewRequest("POST", "/EnrollmentServer/Enrollment.svc", strings.NewReader(body))
		w := ts.do(req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("completes enrollment with valid SOAP and CSR", func(t *testing.T) {
		ts := newTestServer(t)

		// Create real CA for signing
		dir := t.TempDir()
		ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
		require.NoError(t, err)
		ts.server.certService = certs.NewCertificateService(ca, &mockCertStore{})

		ts.server.router.HandleFunc("/EnrollmentServer/Enrollment.svc", ts.server.handleWindowsEnrollmentService).Methods("POST")

		body := buildEnrollmentSOAP(t)
		req := httptest.NewRequest("POST", "/EnrollmentServer/Enrollment.svc", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/soap+xml; charset=utf-8", w.Header().Get("Content-Type"))
		assert.NotEmpty(t, w.Header().Get("Content-Length"))
		respBody := w.Body.String()
		assert.Contains(t, respBody, "RequestSecurityTokenResponseCollection")
		assert.Contains(t, respBody, "BinarySecurityToken")

		// Verify device was created
		require.Len(t, ts.deviceRepo.devices, 1)
		assert.Equal(t, "windows", ts.deviceRepo.devices[0].Platform)
	})
}

// --- Windows OMA-DM Management Sync Tests ---

func TestHandleWindowsManagementSync(t *testing.T) {
	syncMLBody := `<?xml version="1.0" encoding="utf-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD>
    <VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID>
    <MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com/ManagementServer/MDM.svc</LocURI></Target>
    <Source><LocURI>test-device-001</LocURI></Source>
  </SyncHdr>
  <SyncBody>
    <Status>
      <CmdID>1</CmdID>
      <MsgRef>1</MsgRef>
      <CmdRef>0</CmdRef>
      <Cmd>SyncHdr</Cmd>
      <Data>200</Data>
    </Status>
    <Final/>
  </SyncBody>
</SyncML>`

	t.Run("returns 400 for empty body", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/ManagementServer/MDM.svc", ts.server.handleWindowsManagementSync).Methods("POST")

		req := httptest.NewRequest("POST", "/ManagementServer/MDM.svc", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("processes SyncML and returns response", func(t *testing.T) {
		ts := newTestServer(t)
		logger := ts.server.logger
		ts.server.windowsMgmtHandler = windows.NewManagementHandler(
			"https://mdm.example.com/ManagementServer/MDM.svc",
			ts.deviceRepo, ts.commandRepo, logger,
		)
		ts.server.router.HandleFunc("/ManagementServer/MDM.svc", ts.server.handleWindowsManagementSync).Methods("POST")

		req := httptest.NewRequest("POST", "/ManagementServer/MDM.svc", strings.NewReader(syncMLBody))
		req.Header.Set("Content-Type", "application/vnd.syncml.dm+xml")
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/vnd.syncml.dm+xml", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "SyncML")
	})
}

// --- Helpers ---

// mockCertStore satisfies the CertStore interface for enrollment tests
type mockCertStore struct{}

func (m *mockCertStore) StoreCertificate(_ context.Context, _, _ uuid.UUID, _, _, _, _ string, _, _ time.Time) error {
	return nil
}
func (m *mockCertStore) RevokeCertificate(_ context.Context, _ string) error { return nil }
func (m *mockCertStore) GetCertificateBySerial(_ context.Context, _ string) (*models.Certificate, error) {
	return nil, nil
}

// buildEnrollmentSOAP creates a minimal MS-MDE2 WSTEP enrollment SOAP request with a real CSR
func buildEnrollmentSOAP(t *testing.T) string {
	t.Helper()
	csr := windows.GenerateTestCSR(t)
	return windows.BuildTestEnrollmentSOAP(t, csr)
}
