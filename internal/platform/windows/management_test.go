package windows

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SyncML Parser Tests ---

func TestParseSyncML(t *testing.T) {
	t.Run("parses client pkg1", func(t *testing.T) {
		xml := `<?xml version="1.0" encoding="UTF-8"?>
		<SyncML xmlns="SYNCML:SYNCML1.2">
			<SyncHdr>
				<VerDTD>1.2</VerDTD>
				<VerProto>DM/1.2</VerProto>
				<SessionID>1</SessionID>
				<MsgID>1</MsgID>
				<Target><LocURI>https://mdm.example.com/ManagementServer/MDM.svc</LocURI></Target>
				<Source><LocURI>device-001</LocURI></Source>
			</SyncHdr>
			<SyncBody>
				<Alert>
					<CmdID>1</CmdID>
					<Data>1201</Data>
				</Alert>
				<Replace>
					<CmdID>2</CmdID>
					<Item>
						<Source><LocURI>./DevInfo/DevId</LocURI></Source>
						<Data>device-001</Data>
					</Item>
				</Replace>
				<Final/>
			</SyncBody>
		</SyncML>`

		msg, err := ParseSyncML([]byte(xml))
		require.NoError(t, err)

		assert.Equal(t, "1", msg.SyncHdr.SessionID)
		assert.Equal(t, "1", msg.SyncHdr.MsgID)
		assert.Equal(t, "device-001", msg.GetDeviceID())
		assert.Len(t, msg.SyncBody.Alert, 1)
		assert.Equal(t, AlertClientInitiated, msg.SyncBody.Alert[0].Data)
		assert.Len(t, msg.SyncBody.Replace, 1)
		assert.NotNil(t, msg.SyncBody.Final)
	})

	t.Run("parses status responses", func(t *testing.T) {
		xml := `<?xml version="1.0" encoding="UTF-8"?>
		<SyncML xmlns="SYNCML:SYNCML1.2">
			<SyncHdr>
				<VerDTD>1.2</VerDTD>
				<VerProto>DM/1.2</VerProto>
				<SessionID>1</SessionID>
				<MsgID>2</MsgID>
				<Target><LocURI>https://mdm.example.com</LocURI></Target>
				<Source><LocURI>device-001</LocURI></Source>
			</SyncHdr>
			<SyncBody>
				<Status>
					<CmdID>1</CmdID>
					<MsgRef>1</MsgRef>
					<CmdRef>0</CmdRef>
					<Cmd>SyncHdr</Cmd>
					<Data>212</Data>
				</Status>
				<Status>
					<CmdID>2</CmdID>
					<MsgRef>1</MsgRef>
					<CmdRef>3</CmdRef>
					<Cmd>Get</Cmd>
					<Data>200</Data>
				</Status>
				<Results>
					<CmdID>3</CmdID>
					<MsgRef>1</MsgRef>
					<CmdRef>3</CmdRef>
					<Item>
						<Source><LocURI>./DevDetail/SwV</LocURI></Source>
						<Data>10.0.19045</Data>
					</Item>
				</Results>
				<Final/>
			</SyncBody>
		</SyncML>`

		msg, err := ParseSyncML([]byte(xml))
		require.NoError(t, err)

		assert.Len(t, msg.SyncBody.Status, 2)
		assert.Equal(t, StatusAuthAccepted, msg.SyncBody.Status[0].Data)
		assert.Len(t, msg.SyncBody.Results, 1)
		assert.Equal(t, "10.0.19045", msg.SyncBody.Results[0].Item[0].Data)
	})

	t.Run("rejects invalid XML", func(t *testing.T) {
		_, err := ParseSyncML([]byte("not xml"))
		assert.Error(t, err)
	})
}

func TestGenerateSyncML(t *testing.T) {
	t.Run("generates valid response", func(t *testing.T) {
		resp := NewSyncMLResponse("1", "1", "https://mdm.example.com", "device-001")
		resp.AddStatus("1", "1", "0", "SyncHdr", StatusAuthAccepted)
		resp.AddGet("2", "./DevDetail/SwV", "./DevInfo/Man")
		resp.AddExec("3", "./Vendor/MSFT/RemoteLock/Lock")

		data, err := GenerateSyncML(resp)
		require.NoError(t, err)

		// Verify it's valid XML by parsing it back
		var parsed SyncML
		err = xml.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "1", parsed.SyncHdr.SessionID)
		assert.Len(t, parsed.SyncBody.Status, 1)
		assert.Len(t, parsed.SyncBody.Get, 1)
		assert.Len(t, parsed.SyncBody.Get[0].Item, 2)
		assert.Len(t, parsed.SyncBody.Exec, 1)
	})
}

func TestGetDeviceID(t *testing.T) {
	t.Run("returns source LocURI", func(t *testing.T) {
		msg := &SyncML{SyncHdr: SyncHdr{Source: &LocURI{LocURI: "device-123"}}}
		assert.Equal(t, "device-123", msg.GetDeviceID())
	})

	t.Run("returns empty for nil source", func(t *testing.T) {
		msg := &SyncML{SyncHdr: SyncHdr{}}
		assert.Equal(t, "", msg.GetDeviceID())
	})
}

// --- Mock repos for management handler tests ---

type mockDeviceRepoForMgmt struct {
	devices map[uuid.UUID]*models.Device
}

func (m *mockDeviceRepoForMgmt) Create(_ context.Context, d *models.Device) error { return nil }
func (m *mockDeviceRepoForMgmt) GetByID(_ context.Context, id uuid.UUID) (*models.Device, error) {
	if d, ok := m.devices[id]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("device not found")
}
func (m *mockDeviceRepoForMgmt) GetBySerial(_ context.Context, _ uuid.UUID, _ string) (*models.Device, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDeviceRepoForMgmt) GetByPlatformID(_ context.Context, _, _ string) (*models.Device, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDeviceRepoForMgmt) List(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockDeviceRepoForMgmt) ListFiltered(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _, _ int) ([]*models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockDeviceRepoForMgmt) Update(_ context.Context, d *models.Device) error {
	m.devices[d.ID] = d
	return nil
}
func (m *mockDeviceRepoForMgmt) Delete(_ context.Context, _ uuid.UUID) error { return nil }

type mockCmdRepoForMgmt struct {
	commands []*models.DeviceCommand
	sent     []uuid.UUID
}

func (m *mockCmdRepoForMgmt) Create(_ context.Context, cmd *models.DeviceCommand) error {
	m.commands = append(m.commands, cmd)
	return nil
}
func (m *mockCmdRepoForMgmt) GetByID(_ context.Context, id uuid.UUID) (*models.DeviceCommand, error) {
	for _, c := range m.commands {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("command not found")
}
func (m *mockCmdRepoForMgmt) ListPending(_ context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error) {
	var pending []*models.DeviceCommand
	for _, c := range m.commands {
		if c.DeviceID == deviceID && c.Status == models.CommandStatusPending {
			pending = append(pending, c)
		}
	}
	return pending, nil
}
func (m *mockCmdRepoForMgmt) MarkSent(_ context.Context, id uuid.UUID) error {
	m.sent = append(m.sent, id)
	for _, c := range m.commands {
		if c.ID == id {
			c.Status = models.CommandStatusSent
		}
	}
	return nil
}
func (m *mockCmdRepoForMgmt) MarkCompleted(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockCmdRepoForMgmt) MarkFailed(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockCmdRepoForMgmt) ListByDevice(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.DeviceCommand, int, error) {
	return m.commands, len(m.commands), nil
}

// --- Management Handler Tests ---

func TestManagementHandler_HandleSyncML(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	t.Run("handles client-initiated session", func(t *testing.T) {
		deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
			deviceID: {
				BaseModel:    models.BaseModel{ID: deviceID},
				Platform:     models.PlatformWindows,
				PlatformData: models.JSONB{},
			},
		}}
		cmdRepo := &mockCmdRepoForMgmt{}

		handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

		clientMsg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
		<SyncML xmlns="SYNCML:SYNCML1.2">
			<SyncHdr>
				<VerDTD>1.2</VerDTD>
				<VerProto>DM/1.2</VerProto>
				<SessionID>42</SessionID>
				<MsgID>1</MsgID>
				<Target><LocURI>https://mdm.example.com</LocURI></Target>
				<Source><LocURI>%s</LocURI></Source>
			</SyncHdr>
			<SyncBody>
				<Alert>
					<CmdID>1</CmdID>
					<Data>1201</Data>
				</Alert>
				<Final/>
			</SyncBody>
		</SyncML>`, deviceID.String())

		resp, _, err := handler.HandleSyncML(context.Background(), []byte(clientMsg))
		require.NoError(t, err)

		// Parse response
		parsed, err := ParseSyncML(resp)
		require.NoError(t, err)

		assert.Equal(t, "42", parsed.SyncHdr.SessionID)
		assert.Equal(t, deviceID.String(), parsed.SyncHdr.Target.LocURI)
		// Should have status for SyncHdr and Alert
		assert.GreaterOrEqual(t, len(parsed.SyncBody.Status), 2)
	})

	t.Run("delivers pending commands", func(t *testing.T) {
		deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
			deviceID: {
				BaseModel:    models.BaseModel{ID: deviceID},
				Platform:     models.PlatformWindows,
				PlatformData: models.JSONB{},
			},
		}}

		lockCmdID := uuid.New()
		infoCmdID := uuid.New()
		cmdRepo := &mockCmdRepoForMgmt{commands: []*models.DeviceCommand{
			{BaseModel: models.BaseModel{ID: lockCmdID}, DeviceID: deviceID, CommandType: models.CommandTypeLock, Status: models.CommandStatusPending},
			{BaseModel: models.BaseModel{ID: infoCmdID}, DeviceID: deviceID, CommandType: models.CommandTypeDeviceInfo, Status: models.CommandStatusPending},
		}}

		handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

		clientMsg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
		<SyncML xmlns="SYNCML:SYNCML1.2">
			<SyncHdr>
				<VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
				<SessionID>1</SessionID><MsgID>1</MsgID>
				<Target><LocURI>https://mdm.example.com</LocURI></Target>
				<Source><LocURI>%s</LocURI></Source>
			</SyncHdr>
			<SyncBody><Alert><CmdID>1</CmdID><Data>1201</Data></Alert><Final/></SyncBody>
		</SyncML>`, deviceID.String())

		resp, _, err := handler.HandleSyncML(context.Background(), []byte(clientMsg))
		require.NoError(t, err)

		parsed, err := ParseSyncML(resp)
		require.NoError(t, err)

		// Should have Exec (lock) and Get (device info) commands
		assert.Len(t, parsed.SyncBody.Exec, 1)
		assert.Equal(t, "./Vendor/MSFT/RemoteLock/Lock", parsed.SyncBody.Exec[0].Item[0].Target.LocURI)
		assert.Len(t, parsed.SyncBody.Get, 1)
		assert.Len(t, parsed.SyncBody.Get[0].Item, len(DevDetailNodes()))

		// Both commands should be marked sent
		assert.Len(t, cmdRepo.sent, 2)
	})

	t.Run("processes device info results", func(t *testing.T) {
		deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
			deviceID: {
				BaseModel:    models.BaseModel{ID: deviceID},
				Platform:     models.PlatformWindows,
				PlatformData: models.JSONB{},
			},
		}}
		cmdRepo := &mockCmdRepoForMgmt{}

		handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

		// Client sends results from a previous Get command
		clientMsg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
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
					<Item><Source><LocURI>./DevDetail/SwV</LocURI></Source><Data>10.0.22631</Data></Item>
					<Item><Source><LocURI>./DevInfo/Man</LocURI></Source><Data>Dell Inc.</Data></Item>
					<Item><Source><LocURI>./DevInfo/Mod</LocURI></Source><Data>Latitude 5540</Data></Item>
					<Item><Source><LocURI>./DevDetail/Ext/Microsoft/DeviceName</LocURI></Source><Data>DESKTOP-ABC123</Data></Item>
				</Results>
				<Final/>
			</SyncBody>
		</SyncML>`, deviceID.String())

		_, _, err := handler.HandleSyncML(context.Background(), []byte(clientMsg))
		require.NoError(t, err)

		// Verify device was updated
		device := deviceRepo.devices[deviceID]
		assert.Equal(t, "10.0.22631", device.OSVersion)
		assert.Equal(t, "Latitude 5540", device.Model)
		assert.Equal(t, "DESKTOP-ABC123", device.Name)
		assert.Equal(t, "Dell Inc.", device.PlatformData["manufacturer"])
	})

	t.Run("rejects missing device ID", func(t *testing.T) {
		handler := NewManagementHandler("https://mdm.example.com", nil, nil, logger)

		clientMsg := `<?xml version="1.0" encoding="UTF-8"?>
		<SyncML xmlns="SYNCML:SYNCML1.2">
			<SyncHdr>
				<VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
				<SessionID>1</SessionID><MsgID>1</MsgID>
				<Target><LocURI>https://mdm.example.com</LocURI></Target>
			</SyncHdr>
			<SyncBody><Final/></SyncBody>
		</SyncML>`

		_, _, err := handler.HandleSyncML(context.Background(), []byte(clientMsg))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing device ID")
	})

	t.Run("rejects invalid XML", func(t *testing.T) {
		handler := NewManagementHandler("https://mdm.example.com", nil, nil, logger)
		_, _, err := handler.HandleSyncML(context.Background(), []byte("not xml"))
		assert.Error(t, err)
	})
}

func TestDevDetailNodes(t *testing.T) {
	nodes := DevDetailNodes()
	assert.Len(t, nodes, 11)
	// Verify key nodes are present
	found := strings.Join(nodes, ",")
	assert.Contains(t, found, "DeviceName")
	assert.Contains(t, found, "SwV")
	assert.Contains(t, found, "DevId")
	assert.Contains(t, found, "Man")
	assert.Contains(t, found, "Mod")
}

func TestManagementHandler_WipeCommand(t *testing.T) {
	deviceID := uuid.New()
	logger := slog.Default()

	deviceRepo := &mockDeviceRepoForMgmt{devices: map[uuid.UUID]*models.Device{
		deviceID: {
			BaseModel:    models.BaseModel{ID: deviceID},
			Platform:     models.PlatformWindows,
			PlatformData: models.JSONB{},
		},
	}}

	wipeCmdID := uuid.New()
	cmdRepo := &mockCmdRepoForMgmt{commands: []*models.DeviceCommand{
		{BaseModel: models.BaseModel{ID: wipeCmdID, CreatedAt: time.Now()}, DeviceID: deviceID, CommandType: models.CommandTypeWipe, Status: models.CommandStatusPending},
	}}

	handler := NewManagementHandler("https://mdm.example.com", deviceRepo, cmdRepo, logger)

	clientMsg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
	<SyncML xmlns="SYNCML:SYNCML1.2">
		<SyncHdr>
			<VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
			<SessionID>1</SessionID><MsgID>1</MsgID>
			<Target><LocURI>https://mdm.example.com</LocURI></Target>
			<Source><LocURI>%s</LocURI></Source>
		</SyncHdr>
		<SyncBody><Alert><CmdID>1</CmdID><Data>1201</Data></Alert><Final/></SyncBody>
	</SyncML>`, deviceID.String())

	resp, _, err := handler.HandleSyncML(context.Background(), []byte(clientMsg))
	require.NoError(t, err)

	parsed, err := ParseSyncML(resp)
	require.NoError(t, err)

	// Should have wipe Exec command
	require.Len(t, parsed.SyncBody.Exec, 1)
	assert.Equal(t, "./Vendor/MSFT/RemoteWipe/doWipe", parsed.SyncBody.Exec[0].Item[0].Target.LocURI)
	assert.Len(t, cmdRepo.sent, 1)
	assert.Equal(t, wipeCmdID, cmdRepo.sent[0])
}
