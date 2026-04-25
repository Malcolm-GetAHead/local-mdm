package windows

import (
	"context"
	"testing"

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
	t.Run("valid request", func(t *testing.T) {
		xml := `<?xml version="1.0" encoding="UTF-8"?>
<Discover>
  <request>
    <EmailAddress>user@example.com</EmailAddress>
    <RequestVersion>5.0</RequestVersion>
    <DeviceType>WindowsPC</DeviceType>
    <ApplicationVersion>10.0.19041</ApplicationVersion>
    <OSEdition>Professional</OSEdition>
  </request>
</Discover>`
		
		req, err := ParseDiscoverRequest([]byte(xml))
		require.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "user@example.com", req.Request.EmailAddress)
		assert.Equal(t, "5.0", req.Request.RequestVersion)
		assert.Equal(t, "WindowsPC", req.Request.DeviceType)
	})

	t.Run("invalid XML", func(t *testing.T) {
		xml := `<invalid>xml`
		
		req, err := ParseDiscoverRequest([]byte(xml))
		require.Error(t, err)
		assert.Nil(t, req)
	})

	t.Run("empty request", func(t *testing.T) {
		req, err := ParseDiscoverRequest([]byte{})
		require.Error(t, err)
		assert.Nil(t, req)
	})
}

func TestGenerateDiscoverResponse(t *testing.T) {
	enrollmentURL := "https://mdm.example.com/EnrollmentServer/Enrollment.svc"
	policyURL := "https://mdm.example.com/EnrollmentServer/Policy.svc"

	t.Run("success", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse(enrollmentURL, policyURL)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		
		respStr := string(resp)
		assert.Contains(t, respStr, "<?xml version=\"1.0\"")
		assert.Contains(t, respStr, "DiscoverResponse")
		assert.Contains(t, respStr, enrollmentURL)
		assert.Contains(t, respStr, policyURL)
		assert.Contains(t, respStr, "OnPremise")
		assert.Contains(t, respStr, "5.0")
	})

	t.Run("empty URLs", func(t *testing.T) {
		resp, err := GenerateDiscoverResponse("", "")
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestGenerateProvisioningXML(t *testing.T) {
	serverURL := "https://mdm.example.com/omadm"
	certThumbprint := "1234567890ABCDEF"

	t.Run("success", func(t *testing.T) {
		xml := GenerateProvisioningXML(serverURL, certThumbprint)
		assert.NotEmpty(t, xml)
		assert.Contains(t, xml, "<?xml version=\"1.0\"")
		assert.Contains(t, xml, "wap-provisioningdoc")
		assert.Contains(t, xml, serverURL)
		assert.Contains(t, xml, certThumbprint)
		assert.Contains(t, xml, "LocalMDM")
		assert.Contains(t, xml, "DMClient")
	})

	t.Run("empty parameters", func(t *testing.T) {
		xml := GenerateProvisioningXML("", "")
		assert.NotEmpty(t, xml)
		// Should still generate valid XML structure
		assert.Contains(t, xml, "wap-provisioningdoc")
	})
}

func BenchmarkParseDiscoverRequest(b *testing.B) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Discover>
  <request>
    <EmailAddress>user@example.com</EmailAddress>
    <RequestVersion>5.0</RequestVersion>
    <DeviceType>WindowsPC</DeviceType>
  </request>
</Discover>`
	
	data := []byte(xml)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDiscoverRequest(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateDiscoverResponse(b *testing.B) {
	enrollmentURL := "https://mdm.example.com/EnrollmentServer/Enrollment.svc"
	policyURL := "https://mdm.example.com/EnrollmentServer/Policy.svc"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateDiscoverResponse(enrollmentURL, policyURL)
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
