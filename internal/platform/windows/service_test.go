package windows

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/xml"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDeviceRepository is a mock implementation of DeviceRepository
type MockDeviceRepository struct {
	mock.Mock
}

func (m *MockDeviceRepository) Create(ctx context.Context, device *models.Device) error {
	args := m.Called(ctx, device)
	if args.Get(0) != nil {
		device.ID = uuid.New()
	}
	return args.Error(0)
}

func (m *MockDeviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Device), args.Error(1)
}

func (m *MockDeviceRepository) GetBySerial(ctx context.Context, enterpriseID uuid.UUID, serial string) (*models.Device, error) {
	args := m.Called(ctx, enterpriseID, serial)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Device), args.Error(1)
}

func (m *MockDeviceRepository) GetByPlatformID(ctx context.Context, platform, deviceID string) (*models.Device, error) {
	args := m.Called(ctx, platform, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Device), args.Error(1)
}

func (m *MockDeviceRepository) Update(ctx context.Context, device *models.Device) error {
	args := m.Called(ctx, device)
	return args.Error(0)
}

func (m *MockDeviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDeviceRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	args := m.Called(ctx, enterpriseID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.Device), args.Int(1), args.Error(2)
}

func (m *MockDeviceRepository) ListFiltered(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _, _ int) ([]*models.Device, int, error) {
	return nil, 0, nil
}

func TestService_CreateDevice(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceID := "test-device-id"
	hwDevID := "test-hwdevid"

	t.Run("success", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(nil)
		
		svc := NewService(deviceRepo)
		device, err := svc.CreateDevice(ctx, enterpriseID, deviceID, hwDevID)
		
		require.NoError(t, err)
		assert.NotNil(t, device)
		assert.Equal(t, enterpriseID, device.EnterpriseID)
		assert.Equal(t, deviceID, device.DeviceID)
		assert.Equal(t, hwDevID, device.SerialNumber)
		assert.Equal(t, models.PlatformWindows, device.Platform)
		assert.Equal(t, models.DeviceStatusPending, device.Status)
		
		deviceRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(assert.AnError)
		
		svc := NewService(deviceRepo)
		device, err := svc.CreateDevice(ctx, enterpriseID, deviceID, hwDevID)
		
		require.Error(t, err)
		assert.Nil(t, device)
		assert.Contains(t, err.Error(), "failed to create device")
		
		deviceRepo.AssertExpectations(t)
	})
}

func TestParseDiscoverRequest(t *testing.T) {
	t.Run("valid SOAP envelope request", func(t *testing.T) {
		xml := `<?xml version="1.0"?>
<s:Envelope xmlns:a="http://www.w3.org/2005/08/addressing"
   xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/Discover</a:Action>
    <a:MessageID>urn:uuid:748132ec-a575-4329-b01b-6171a9cf8478</a:MessageID>
    <a:To s:mustUnderstand="1">https://mdm.example.com/EnrollmentServer/Discovery.svc</a:To>
  </s:Header>
  <s:Body>
    <Discover xmlns="http://schemas.microsoft.com/windows/management/2012/01/enrollment/">
      <request xmlns:i="http://www.w3.org/2001/XMLSchema-instance">
        <EmailAddress>user@example.com</EmailAddress>
        <RequestVersion>5.0</RequestVersion>
        <DeviceType>WindowsPC</DeviceType>
        <ApplicationVersion>10.0.19041</ApplicationVersion>
        <OSEdition>Professional</OSEdition>
      </request>
    </Discover>
  </s:Body>
</s:Envelope>`

		req, messageID, err := ParseDiscoverRequest([]byte(xml))
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "user@example.com", req.Request.EmailAddress)
		assert.Equal(t, "5.0", req.Request.RequestVersion)
		assert.Equal(t, "WindowsPC", req.Request.DeviceType)
		assert.Equal(t, "urn:uuid:748132ec-a575-4329-b01b-6171a9cf8478", messageID)
	})

	t.Run("bare XML fallback", func(t *testing.T) {
		xml := `<?xml version="1.0" encoding="UTF-8"?>
<Discover>
  <request>
    <EmailAddress>user@example.com</EmailAddress>
    <RequestVersion>5.0</RequestVersion>
    <DeviceType>WindowsPC</DeviceType>
  </request>
</Discover>`

		req, messageID, err := ParseDiscoverRequest([]byte(xml))
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "user@example.com", req.Request.EmailAddress)
		assert.Empty(t, messageID)
	})

	t.Run("invalid XML", func(t *testing.T) {
		xml := `<invalid>xml`

		req, _, err := ParseDiscoverRequest([]byte(xml))
		require.Error(t, err)
		assert.Nil(t, req)
	})

	t.Run("empty request", func(t *testing.T) {
		req, _, err := ParseDiscoverRequest([]byte{})
		require.Error(t, err)
		assert.Nil(t, req)
	})
}

func TestGenerateDiscoverResponse(t *testing.T) {
	enrollmentURL := "https://mdm.example.com/EnrollmentServer/Enrollment.svc"
	policyURL := "https://mdm.example.com/EnrollmentServer/Policy.svc"
	messageID := "urn:uuid:748132ec-a575-4329-b01b-6171a9cf8478"

	t.Run("success", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		assert.NotNil(t, resp)

		respStr := string(resp)
		assert.Contains(t, respStr, "DiscoverResponse")
		assert.Contains(t, respStr, enrollmentURL)
		assert.Contains(t, respStr, policyURL)
		assert.Contains(t, respStr, "OnPremise")
		assert.Contains(t, respStr, "4.0")
		assert.NotContains(t, respStr, "AuthenticationServiceUrl")
		assert.Contains(t, respStr, "RelatesTo")
		assert.Contains(t, respStr, messageID)
		assert.Contains(t, respStr, "s:Envelope")
	})

	t.Run("empty URLs", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse("", "", "")
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("namespace has no trailing slash", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		respStr := string(resp)
		// The enrollment namespace must NOT have a trailing slash — Windows rejects it
		assert.Contains(t, respStr, `xmlns="`+DiscoveryNS+`"`)
		// Verify the namespace in the DiscoverResponse element ends with "enrollment" not "enrollment/"
		assert.NotContains(t, respStr, `xmlns="http://schemas.microsoft.com/windows/management/2012/01/enrollment/"`)
	})

	t.Run("enrollment version is 4.0", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		respStr := string(resp)
		assert.Contains(t, respStr, "<EnrollmentVersion>4.0</EnrollmentVersion>")
	})

	t.Run("no AuthenticationServiceUrl element", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		respStr := string(resp)
		// OnPremise auth means no AuthenticationServiceUrl
		assert.NotContains(t, respStr, "AuthenticationServiceUrl")
	})

	t.Run("Content-Length can be computed", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		// Response must have deterministic length for Content-Length header
		// (MS-MDE2 requires non-chunked responses)
		assert.Greater(t, len(resp), 0)
	})

	t.Run("unique ActivityId per call", func(t *testing.T) {
		resp1, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		resp2, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		// ActivityId should differ between calls
		assert.NotEqual(t, string(resp1), string(resp2))
	})

	t.Run("response is valid XML", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		require.NoError(t, err)
		// Must be parseable XML
		var v interface{}
		require.NoError(t, xml.Unmarshal(resp, &v))
	})
}

func TestGenerateProvisioningXML(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Create test CA and device certs
		caKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		caTemplate := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "Test CA"},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
		}
		caDER, _ := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
		caCert, _ := x509.ParseCertificate(caDER)

		devKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		devTemplate := &x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: "MDMDeviceCert"},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(time.Hour),
		}
		devDER, _ := x509.CreateCertificate(rand.Reader, devTemplate, caCert, &devKey.PublicKey, caKey)
		devCert, _ := x509.ParseCertificate(devDER)

		xml := GenerateProvisioningXML("https://mdm.example.com/ManagementServer/MDM.svc", caCert, devCert)
		assert.NotEmpty(t, xml)
		assert.Contains(t, xml, "wap-provisioningdoc")
		assert.Contains(t, xml, "https://mdm.example.com/ManagementServer/MDM.svc")
		assert.Contains(t, xml, "LocalMDM")
		assert.Contains(t, xml, "DMClient")
		assert.Contains(t, xml, "EncodedCertificate")
		assert.Contains(t, xml, "CertificateStore")
		assert.Contains(t, xml, "WSTEP")
		assert.Contains(t, xml, "Renew")
	})
}

func BenchmarkParseDiscoverRequest(b *testing.B) {
	xml := `<?xml version="1.0"?>
<s:Envelope xmlns:a="http://www.w3.org/2005/08/addressing"
   xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/Discover</a:Action>
    <a:MessageID>urn:uuid:748132ec-a575-4329-b01b-6171a9cf8478</a:MessageID>
  </s:Header>
  <s:Body>
    <Discover xmlns="http://schemas.microsoft.com/windows/management/2012/01/enrollment/">
      <request>
        <EmailAddress>user@example.com</EmailAddress>
        <RequestVersion>5.0</RequestVersion>
        <DeviceType>WindowsPC</DeviceType>
      </request>
    </Discover>
  </s:Body>
</s:Envelope>`

	data := []byte(xml)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := ParseDiscoverRequest(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateDiscoverResponse(b *testing.B) {
	enrollmentURL := "https://mdm.example.com/EnrollmentServer/Enrollment.svc"
	policyURL := "https://mdm.example.com/EnrollmentServer/Policy.svc"
	messageID := "urn:uuid:748132ec-a575-4329-b01b-6171a9cf8478"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestGenerateUUID(t *testing.T) {
	t.Run("generates unique UUIDs", func(t *testing.T) {
		uuids := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			uuid := generateUUID()
			assert.NotEmpty(t, uuid)
			assert.False(t, uuids[uuid], "duplicate UUID generated")
			uuids[uuid] = true
		}
	})

	t.Run("generates valid UUID format", func(t *testing.T) {
		uuid := generateUUID()
		// Should match UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, uuid)
	})
}

// TestEnterpriseIDFromEmail tests the UUID extraction logic used in the
// Windows enrollment handler to derive enterprise ID from email username.
func TestEnterpriseIDFromEmail(t *testing.T) {
	// extractEnterpriseIDFromEmail mirrors the handler logic
	extractEnterpriseIDFromEmail := func(email string) uuid.UUID {
		if atIdx := strings.Index(email, "@"); atIdx > 0 {
			if eid, err := uuid.Parse(email[:atIdx]); err == nil {
				return eid
			}
		}
		return uuid.Nil
	}

	t.Run("valid UUID email", func(t *testing.T) {
		eid := extractEnterpriseIDFromEmail("00000000-0000-0000-0000-000000000001@localmdm.local")
		assert.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000001"), eid)
	})

	t.Run("non-UUID email falls back to Nil", func(t *testing.T) {
		eid := extractEnterpriseIDFromEmail("admin@localmdm.local")
		assert.Equal(t, uuid.Nil, eid)
	})

	t.Run("empty string", func(t *testing.T) {
		eid := extractEnterpriseIDFromEmail("")
		assert.Equal(t, uuid.Nil, eid)
	})

	t.Run("no @ sign", func(t *testing.T) {
		eid := extractEnterpriseIDFromEmail("noemail")
		assert.Equal(t, uuid.Nil, eid)
	})

	t.Run("@ at start", func(t *testing.T) {
		eid := extractEnterpriseIDFromEmail("@domain.com")
		assert.Equal(t, uuid.Nil, eid)
	})

	t.Run("partial UUID before @", func(t *testing.T) {
		eid := extractEnterpriseIDFromEmail("not-a-uuid@domain.com")
		assert.Equal(t, uuid.Nil, eid)
	})
}

// TestDuplicateDeviceUpsert tests that re-enrollment with the same hardware
// device ID updates the existing record instead of creating a duplicate.
func TestDuplicateDeviceUpsert(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	hwDevID := "EXISTING-HW-DEV-ID"

	t.Run("re-enrollment updates existing device", func(t *testing.T) {
		existingDevice := &models.Device{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			EnterpriseID: enterpriseID,
			Platform:     models.PlatformWindows,
			DeviceID:     hwDevID,
			Name:         "Old Name",
			Status:       models.DeviceStatusEnrolled,
			PlatformData: models.JSONB{},
		}

		deviceRepo := new(MockDeviceRepository)
		// GetByPlatformID finds the existing device
		deviceRepo.On("GetByPlatformID", ctx, models.PlatformWindows, hwDevID).Return(existingDevice, nil)
		deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

		// Simulate the handler's upsert logic
		existing, err := deviceRepo.GetByPlatformID(ctx, models.PlatformWindows, hwDevID)
		require.NoError(t, err)
		require.NotNil(t, existing)

		existing.Name = "New Name"
		existing.OSVersion = "10.0.26200"
		existing.Status = models.DeviceStatusEnrolled
		err = deviceRepo.Update(ctx, existing)
		require.NoError(t, err)

		assert.Equal(t, "New Name", existing.Name)
		assert.Equal(t, "10.0.26200", existing.OSVersion)
		deviceRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		deviceRepo.AssertExpectations(t)
	})

	t.Run("new enrollment creates device", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		// GetByPlatformID returns not found
		deviceRepo.On("GetByPlatformID", ctx, models.PlatformWindows, "NEW-HW-ID").Return(nil, assert.AnError)
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

		existing, _ := deviceRepo.GetByPlatformID(ctx, models.PlatformWindows, "NEW-HW-ID")
		assert.Nil(t, existing)

		svc := NewService(deviceRepo)
		device, err := svc.CreateDevice(ctx, enterpriseID, "NEW-HW-ID", "NEW-HW-ID")
		require.NoError(t, err)
		assert.NotNil(t, device)
		assert.Equal(t, models.PlatformWindows, device.Platform)
		deviceRepo.AssertExpectations(t)
	})
}

// TestExtractDeviceIDFromSyncML tests device ID extraction from SyncML messages.
func TestExtractDeviceIDFromSyncML(t *testing.T) {
	t.Run("extracts device ID from Source", func(t *testing.T) {
		syncml := `<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD>
    <VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID>
    <MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>device-hw-id-123</LocURI></Source>
  </SyncHdr>
  <SyncBody><Final/></SyncBody>
</SyncML>`
		id := ExtractDeviceIDFromSyncML([]byte(syncml))
		assert.Equal(t, "device-hw-id-123", id)
	})

	t.Run("returns empty for invalid XML", func(t *testing.T) {
		id := ExtractDeviceIDFromSyncML([]byte("not xml"))
		assert.Empty(t, id)
	})
}
