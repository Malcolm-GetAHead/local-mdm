package windows

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- enrollment.go pure functions ---

func TestGetContextValue(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var ac *AdditionalContext
		assert.Equal(t, "", ac.GetContextValue("anything"))
	})

	t.Run("empty items", func(t *testing.T) {
		ac := &AdditionalContext{}
		assert.Equal(t, "", ac.GetContextValue("key"))
	})

	t.Run("matching key", func(t *testing.T) {
		ac := &AdditionalContext{ContextItems: []ContextItem{
			{Name: "DeviceType", Value: "WindowsPC"},
			{Name: "OSVersion", Value: "10.0"},
		}}
		assert.Equal(t, "WindowsPC", ac.GetContextValue("DeviceType"))
		assert.Equal(t, "10.0", ac.GetContextValue("OSVersion"))
	})

	t.Run("missing key", func(t *testing.T) {
		ac := &AdditionalContext{ContextItems: []ContextItem{
			{Name: "DeviceType", Value: "WindowsPC"},
		}}
		assert.Equal(t, "", ac.GetContextValue("Missing"))
	})
}

func TestParseEnrollmentRequest(t *testing.T) {
	validSOAP := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing"><s:Header><a:Action>http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RST/wstep</a:Action><a:MessageID>urn:uuid:test</a:MessageID><a:To>https://mdm.local/enrollment</a:To></s:Header><s:Body><RequestSecurityToken xmlns="http://docs.oasis-open.org/ws-sx/ws-trust/200512"><TokenType>http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken</TokenType><RequestType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</RequestType><BinarySecurityToken xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" ValueType="http://schemas.microsoft.com/windows/pki/2009/01/enrollment#PKCS10" EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd#base64binary">dGVzdA==</BinarySecurityToken></RequestSecurityToken></s:Body></s:Envelope>`

	t.Run("valid SOAP XML", func(t *testing.T) {
		env, err := ParseEnrollmentRequest([]byte(validSOAP))
		require.NoError(t, err)
		assert.Equal(t, "urn:uuid:test", env.Header.MessageID)
		assert.Equal(t, "https://mdm.local/enrollment", env.Header.To)
		require.NotNil(t, env.Body.RequestSecurityToken)
		assert.Contains(t, env.Body.RequestSecurityToken.TokenType, "DeviceEnrollmentToken")
		require.NotNil(t, env.Body.RequestSecurityToken.BinarySecurityToken)
		assert.Equal(t, "dGVzdA==", env.Body.RequestSecurityToken.BinarySecurityToken.Value)
	})

	t.Run("invalid XML", func(t *testing.T) {
		_, err := ParseEnrollmentRequest([]byte("<broken"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse SOAP envelope")
	})
}

func TestExtractCSR(t *testing.T) {
	validSOAP := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing"><s:Header><a:Action>test</a:Action><a:MessageID>urn:uuid:test</a:MessageID><a:To>https://mdm.local</a:To></s:Header><s:Body><RequestSecurityToken xmlns="http://docs.oasis-open.org/ws-sx/ws-trust/200512"><TokenType>token</TokenType><RequestType>issue</RequestType><BinarySecurityToken xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" ValueType="pkcs10" EncodingType="base64">dGVzdA==</BinarySecurityToken></RequestSecurityToken></s:Body></s:Envelope>`

	t.Run("valid envelope", func(t *testing.T) {
		env, err := ParseEnrollmentRequest([]byte(validSOAP))
		require.NoError(t, err)
		csr, err := ExtractCSR(env)
		require.NoError(t, err)
		assert.Equal(t, []byte("test"), csr)
	})

	t.Run("nil RST", func(t *testing.T) {
		env := &SOAPEnvelope{}
		_, err := ExtractCSR(env)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no RequestSecurityToken")
	})

	t.Run("nil BST", func(t *testing.T) {
		env := &SOAPEnvelope{Body: SOAPBody{RequestSecurityToken: &RequestSecurityToken{}}}
		_, err := ExtractCSR(env)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no BinarySecurityToken")
	})

	t.Run("invalid base64", func(t *testing.T) {
		env := &SOAPEnvelope{Body: SOAPBody{RequestSecurityToken: &RequestSecurityToken{
			BinarySecurityToken: &BinarySecurityToken{Value: "!!!not-base64!!!"},
		}}}
		_, err := ExtractCSR(env)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode CSR")
	})
}

func TestExtractAdditionalContext(t *testing.T) {
	t.Run("with RST and context", func(t *testing.T) {
		ac := &AdditionalContext{ContextItems: []ContextItem{{Name: "k", Value: "v"}}}
		env := &SOAPEnvelope{Body: SOAPBody{RequestSecurityToken: &RequestSecurityToken{AdditionalContext: ac}}}
		result := ExtractAdditionalContext(env)
		require.NotNil(t, result)
		assert.Len(t, result.ContextItems, 1)
	})

	t.Run("without RST", func(t *testing.T) {
		env := &SOAPEnvelope{}
		assert.Nil(t, ExtractAdditionalContext(env))
	})

	t.Run("RST without context", func(t *testing.T) {
		env := &SOAPEnvelope{Body: SOAPBody{RequestSecurityToken: &RequestSecurityToken{}}}
		assert.Nil(t, ExtractAdditionalContext(env))
	})
}

func TestGenerateEnrollmentResponse(t *testing.T) {
	resp, err := GenerateEnrollmentResponse("<wap-provisioningdoc/>", "urn:uuid:test-msg")
	require.NoError(t, err)
	s := string(resp)
	assert.Contains(t, s, "RelatesTo")
	assert.Contains(t, s, "urn:uuid:test-msg")
	assert.Contains(t, s, "RequestSecurityTokenResponseCollection")
	assert.Contains(t, s, "BinarySecurityToken")
	assert.Contains(t, s, "DeviceEnrollmentProvisionDoc")
	assert.Contains(t, s, "RSTRC/wstep")
}

// --- HandleSyncML error paths ---

func TestHandleSyncML_ErrorPaths(t *testing.T) {
	logger := slog.Default()

	t.Run("invalid XML", func(t *testing.T) {
		h := NewManagementHandler("https://mdm.example.com", nil, nil, logger)
		_, _, err := h.HandleSyncML(context.Background(), []byte("<<<not xml"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse SyncML")
	})

	t.Run("empty Source LocURI", func(t *testing.T) {
		msg := `<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI></LocURI></Source>
  </SyncHdr>
  <SyncBody><Final/></SyncBody>
</SyncML>`
		h := NewManagementHandler("https://mdm.example.com", nil, nil, logger)
		_, _, err := h.HandleSyncML(context.Background(), []byte(msg))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing device ID")
	})
}

// --- processStatus with non-zero CmdRef ---

func TestHandleSyncML_ProcessStatusNonZeroCmdRef(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
		deviceID: {
			BaseModel:    models.BaseModel{ID: deviceID},
			Platform:     models.PlatformWindows,
			PlatformData: models.JSONB{},
		},
	}}
	cmdRepo := &mockCmdRepoForMgmt{}

	handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

	// Client sends Status with CmdRef="1" (acknowledging a server command)
	msg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>2</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>%s</LocURI></Source>
  </SyncHdr>
  <SyncBody>
    <Status><CmdID>1</CmdID><MsgRef>1</MsgRef><CmdRef>0</CmdRef><Cmd>SyncHdr</Cmd><Data>212</Data></Status>
    <Status><CmdID>2</CmdID><MsgRef>1</MsgRef><CmdRef>1</CmdRef><Cmd>Get</Cmd><Data>200</Data></Status>
    <Final/>
  </SyncBody>
</SyncML>`, deviceID.String())

	resp, devID, err := handler.HandleSyncML(context.Background(), []byte(msg))
	require.NoError(t, err)
	assert.Equal(t, deviceID.String(), devID)
	assert.NotEmpty(t, resp)
}

// --- CSP helper branches ---

func TestBuildVPNCSPCommands_AlwaysOn(t *testing.T) {
	data := models.JSONB{
		"name":      "TestVPN",
		"server":    "vpn.test.com",
		"always_on": true,
	}
	cmds, err := BuildVPNCSPCommands(data)
	require.NoError(t, err)
	// Should have base 4 + AlwaysOn = 5
	assert.Len(t, cmds, 5)
	found := false
	for _, c := range cmds {
		if c.URI == "./Vendor/MSFT/VPNv2/TestVPN/AlwaysOn" {
			assert.Equal(t, "true", c.Value)
			found = true
		}
	}
	assert.True(t, found, "AlwaysOn command not found")
}

func TestIntFromJSONB_IntType(t *testing.T) {
	data := models.JSONB{"val": int(7)}
	v, ok := intFromJSONB(data, "val")
	assert.True(t, ok)
	assert.Equal(t, 7, v)
}

func TestIntFromJSONB_UnsupportedType(t *testing.T) {
	data := models.JSONB{"val": []string{"nope"}}
	_, ok := intFromJSONB(data, "val")
	assert.False(t, ok)
}

func TestBoolFromJSONB_Float64(t *testing.T) {
	data := models.JSONB{"on": float64(1), "off": float64(0)}
	v, ok := boolFromJSONB(data, "on")
	assert.True(t, ok)
	assert.True(t, v)

	v, ok = boolFromJSONB(data, "off")
	assert.True(t, ok)
	assert.False(t, v)
}

// --- extractJSONString missing-quote edge case ---

func TestExtractJSONString_MissingClosingQuote(t *testing.T) {
	body := `{"access_token":"abc123`
	assert.Equal(t, "", extractJSONString(body, "access_token"))
}

// --- UpdateDeviceStatus ---

func TestService_UpdateDeviceStatus(t *testing.T) {
	ctx := context.Background()
	deviceID := uuid.New()

	t.Run("success", func(t *testing.T) {
		deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
			deviceID: {
				BaseModel: models.BaseModel{ID: deviceID},
				Status:    models.DeviceStatusPending,
			},
		}}
		svc := NewService(deviceRepo)
		err := svc.UpdateDeviceStatus(ctx, deviceID, models.DeviceStatusEnrolled)
		require.NoError(t, err)
		assert.Equal(t, models.DeviceStatusEnrolled, deviceRepo.devices[deviceID].Status)
	})

	t.Run("device not found", func(t *testing.T) {
		deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{}}
		svc := NewService(deviceRepo)
		err := svc.UpdateDeviceStatus(ctx, uuid.New(), models.DeviceStatusEnrolled)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get device")
	})
}

// --- processReplace via HandleSyncML ---

func TestHandleSyncML_ProcessReplace(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
		deviceID: {
			BaseModel:    models.BaseModel{ID: deviceID},
			Platform:     models.PlatformWindows,
			PlatformData: models.JSONB{},
		},
	}}
	cmdRepo := &mockCmdRepoForMgmt{}
	handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

	msg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>%s</LocURI></Source>
  </SyncHdr>
  <SyncBody>
    <Alert><CmdID>1</CmdID><Data>1201</Data></Alert>
    <Replace>
      <CmdID>2</CmdID>
      <Item><Source><LocURI>./DevInfo/Man</LocURI></Source><Data>Lenovo</Data></Item>
    </Replace>
    <Final/>
  </SyncBody>
</SyncML>`, deviceID.String())

	resp, _, err := handler.HandleSyncML(context.Background(), []byte(msg))
	require.NoError(t, err)
	assert.NotEmpty(t, resp)
	assert.Equal(t, "Lenovo", deviceRepo.devices[deviceID].PlatformData["manufacturer"])
}

// --- deliverPendingCommands: profile commands ---

func TestHandleSyncML_DeliverProfileCommands(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
		deviceID: {
			BaseModel:    models.BaseModel{ID: deviceID},
			Platform:     models.PlatformWindows,
			PlatformData: models.JSONB{},
		},
	}}

	// Queue a wifi profile command and a security profile command
	wifiCmdID := uuid.New()
	secCmdID := uuid.New()
	vpnCmdID := uuid.New()
	unknownCmdID := uuid.New()
	cmdRepo := &mockCmdRepoForMgmt{commands: []*models.DeviceCommand{
		{
			BaseModel:   models.BaseModel{ID: wifiCmdID},
			DeviceID:    deviceID,
			CommandType: models.CommandTypeInstallProfile,
			Status:      models.CommandStatusPending,
			CommandData: models.JSONB{
				"profile_type": "wifi",
				"profile_data": map[string]interface{}{"ssid": "TestNet", "password": "pass123"},
			},
		},
		{
			BaseModel:   models.BaseModel{ID: secCmdID},
			DeviceID:    deviceID,
			CommandType: models.CommandTypeInstallProfile,
			Status:      models.CommandStatusPending,
			CommandData: models.JSONB{
				"profile_type": "security",
				"profile_data": map[string]interface{}{"min_password_length": float64(8)},
			},
		},
		{
			BaseModel:   models.BaseModel{ID: vpnCmdID},
			DeviceID:    deviceID,
			CommandType: models.CommandTypeInstallProfile,
			Status:      models.CommandStatusPending,
			CommandData: models.JSONB{
				"profile_type": "vpn",
				"profile_data": map[string]interface{}{"name": "Corp", "server": "vpn.test.com"},
			},
		},
		{
			BaseModel:   models.BaseModel{ID: unknownCmdID},
			DeviceID:    deviceID,
			CommandType: "custom_unknown",
			Status:      models.CommandStatusPending,
		},
	}}

	handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

	msg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>%s</LocURI></Source>
  </SyncHdr>
  <SyncBody><Alert><CmdID>1</CmdID><Data>1201</Data></Alert><Final/></SyncBody>
</SyncML>`, deviceID.String())

	resp, _, err := handler.HandleSyncML(context.Background(), []byte(msg))
	require.NoError(t, err)
	assert.NotEmpty(t, resp)
	// wifi, security, vpn should be sent; unknown skipped
	assert.Len(t, cmdRepo.sent, 3)
}

// --- buildProfileCSPCommands: unsupported type ---

func TestHandleSyncML_UnsupportedProfileType(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
		deviceID: {
			BaseModel:    models.BaseModel{ID: deviceID},
			Platform:     models.PlatformWindows,
			PlatformData: models.JSONB{},
		},
	}}
	cmdRepo := &mockCmdRepoForMgmt{commands: []*models.DeviceCommand{
		{
			BaseModel:   models.BaseModel{ID: uuid.New()},
			DeviceID:    deviceID,
			CommandType: models.CommandTypeInstallProfile,
			Status:      models.CommandStatusPending,
			CommandData: models.JSONB{
				"profile_type": "bluetooth",
				"profile_data": map[string]interface{}{},
			},
		},
	}}

	handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

	msg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>%s</LocURI></Source>
  </SyncHdr>
  <SyncBody><Alert><CmdID>1</CmdID><Data>1201</Data></Alert><Final/></SyncBody>
</SyncML>`, deviceID.String())

	resp, _, err := handler.HandleSyncML(context.Background(), []byte(msg))
	require.NoError(t, err)
	assert.NotEmpty(t, resp)
}

// --- AppList command delivery ---

func TestHandleSyncML_AppListCommand(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
		deviceID: {
			BaseModel:    models.BaseModel{ID: deviceID},
			Platform:     models.PlatformWindows,
			PlatformData: models.JSONB{},
		},
	}}
	cmdRepo := &mockCmdRepoForMgmt{commands: []*models.DeviceCommand{
		{
			BaseModel:   models.BaseModel{ID: uuid.New()},
			DeviceID:    deviceID,
			CommandType: models.CommandTypeAppList,
			Status:      models.CommandStatusPending,
		},
	}}

	handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

	msg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>%s</LocURI></Source>
  </SyncHdr>
  <SyncBody><Alert><CmdID>1</CmdID><Data>1201</Data></Alert><Final/></SyncBody>
</SyncML>`, deviceID.String())

	resp, _, err := handler.HandleSyncML(context.Background(), []byte(msg))
	require.NoError(t, err)

	parsed, err := ParseSyncML(resp)
	require.NoError(t, err)
	// AppList delivers Get commands for app inventory
	assert.NotEmpty(t, parsed.SyncBody.Get)
}

// --- updateDeviceInfo: empty uri/value, unknown uri ---

func TestHandleSyncML_ResultsWithTargetURI(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
		deviceID: {
			BaseModel:    models.BaseModel{ID: deviceID},
			Platform:     models.PlatformWindows,
			PlatformData: models.JSONB{},
		},
	}}
	cmdRepo := &mockCmdRepoForMgmt{}
	handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

	// Results with Target (not Source) URI, and an unknown URI
	msg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>2</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>%s</LocURI></Source>
  </SyncHdr>
  <SyncBody>
    <Status><CmdID>1</CmdID><MsgRef>1</MsgRef><CmdRef>0</CmdRef><Cmd>SyncHdr</Cmd><Data>212</Data></Status>
    <Results>
      <CmdID>2</CmdID><MsgRef>1</MsgRef><CmdRef>2</CmdRef>
      <Item><Target><LocURI>./DevInfo/Mod</LocURI></Target><Data>ThinkPad</Data></Item>
      <Item><Source><LocURI>./Unknown/Path</LocURI></Source><Data>ignored</Data></Item>
      <Item><Data>no-uri</Data></Item>
    </Results>
    <Final/>
  </SyncBody>
</SyncML>`, deviceID.String())

	_, _, err := handler.HandleSyncML(context.Background(), []byte(msg))
	require.NoError(t, err)
	assert.Equal(t, "ThinkPad", deviceRepo.devices[deviceID].Model)
}

// --- findDeviceByDeviceID: non-UUID device ID ---

func TestHandleSyncML_NonUUIDDeviceID(t *testing.T) {
	logger := slog.Default()

	// Override GetByPlatformID to return a device for a non-UUID ID
	deviceRepo2 := &mockDeviceRepoWithPlatformID{
		mockDeviceRepoForMgmt: mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{}},
	}
	devID := uuid.New()
	deviceRepo2.devices[devID] = &models.Device{
		BaseModel:    models.BaseModel{ID: devID},
		Platform:     models.PlatformWindows,
		PlatformData: models.JSONB{},
	}
	deviceRepo2.platformDevice = deviceRepo2.devices[devID]

	cmdRepo := &mockCmdRepoForMgmt{}
	handler := NewManagementHandler("https://mdm.example.com", deviceRepo2, cmdRepo, logger)

	msg := `<?xml version="1.0" encoding="UTF-8"?>
<SyncML xmlns="SYNCML:SYNCML1.2">
  <SyncHdr>
    <VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
    <SessionID>1</SessionID><MsgID>1</MsgID>
    <Target><LocURI>https://mdm.example.com</LocURI></Target>
    <Source><LocURI>hw-device-id-abc</LocURI></Source>
  </SyncHdr>
  <SyncBody><Alert><CmdID>1</CmdID><Data>1201</Data></Alert><Final/></SyncBody>
</SyncML>`

	resp, devIDStr, err := handler.HandleSyncML(context.Background(), []byte(msg))
	require.NoError(t, err)
	assert.Equal(t, "hw-device-id-abc", devIDStr)
	assert.NotEmpty(t, resp)
}

// mockDeviceRepoWithPlatformID extends mockDeviceRepoForMgmt to return a device for GetByPlatformID
type mockDeviceRepoWithPlatformID struct {
	mockDeviceRepoForMgmt
	platformDevice *models.Device
}

func (m *mockDeviceRepoWithPlatformID) GetByPlatformID(_ context.Context, _, _ string) (*models.Device, error) {
	if m.platformDevice != nil {
		return m.platformDevice, nil
	}
	return nil, fmt.Errorf("not found")
}

// --- boolFromJSONB unsupported type ---

func TestBoolFromJSONB_UnsupportedType(t *testing.T) {
	data := models.JSONB{"val": []string{"nope"}}
	_, ok := boolFromJSONB(data, "val")
	assert.False(t, ok)
}
