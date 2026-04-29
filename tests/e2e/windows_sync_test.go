package e2e

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_WindowsOMADMSync exercises the full OMA-DM sync flow against a real database:
// 1. Creates a device in the DB
// 2. Sends a SyncML message as that device
// 3. Verifies the response contains auto-query Get commands
// 4. Sends a follow-up with Results containing device info
// 5. Verifies platform_data was updated in the DB
func TestE2E_WindowsOMADMSync(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	cmdRepo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	// Create test enterprise and device
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Windows Sync Test",
		Slug:      "win-sync-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	deviceID := uuid.New()
	hwID := "TESTDEVICE" + uuid.New().String()[:8]
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformWindows,
		DeviceID:     hwID,
		Name:         "Test Windows PC",
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(ctx, device))

	handler := windows.NewManagementHandler("https://mdm.test", deviceRepo, cmdRepo, slog.Default())

	// --- Sync 1: Client initiates session, server should respond with Get commands ---
	syncMsg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
	<SyncML xmlns="SYNCML:SYNCML1.2">
		<SyncHdr>
			<VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
			<SessionID>1</SessionID><MsgID>1</MsgID>
			<Target><LocURI>https://mdm.test</LocURI></Target>
			<Source><LocURI>%s</LocURI></Source>
		</SyncHdr>
		<SyncBody>
			<Alert><CmdID>1</CmdID><Data>1201</Data></Alert>
			<Replace><CmdID>2</CmdID>
				<Item><Source><LocURI>./DevInfo/DevId</LocURI></Source><Data>%s</Data></Item>
				<Item><Source><LocURI>./DevInfo/Man</LocURI></Source><Data>TestMfg</Data></Item>
				<Item><Source><LocURI>./DevInfo/Lang</LocURI></Source><Data>en-US</Data></Item>
			</Replace>
			<Final/>
		</SyncBody>
	</SyncML>`, hwID, hwID)

	respBytes, returnedID, err := handler.HandleSyncML(ctx, []byte(syncMsg))
	require.NoError(t, err)
	assert.Equal(t, hwID, returnedID)

	// Parse response and verify Get commands present
	var resp windows.SyncML
	require.NoError(t, xml.Unmarshal(respBytes, &resp))
	assert.GreaterOrEqual(t, len(resp.SyncBody.Get), 2, "should have DevDetail + SecurityCSP Get commands")

	// Verify Replace data was stored
	updated, err := deviceRepo.GetByID(ctx, deviceID)
	require.NoError(t, err)
	assert.Equal(t, "TestMfg", updated.PlatformData["manufacturer"])
	assert.Equal(t, "en-US", updated.PlatformData["language"])

	// --- Sync 2: Client returns Results from our Get commands ---
	resultsMsg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
	<SyncML xmlns="SYNCML:SYNCML1.2">
		<SyncHdr>
			<VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
			<SessionID>1</SessionID><MsgID>2</MsgID>
			<Target><LocURI>https://mdm.test</LocURI></Target>
			<Source><LocURI>%s</LocURI></Source>
		</SyncHdr>
		<SyncBody>
			<Status><CmdID>1</CmdID><MsgRef>1</MsgRef><CmdRef>0</CmdRef><Cmd>SyncHdr</Cmd><Data>212</Data></Status>
			<Results><CmdID>2</CmdID><MsgRef>1</MsgRef><CmdRef>2</CmdRef>
				<Item><Source><LocURI>./DevDetail/SwV</LocURI></Source><Data>10.0.22631</Data></Item>
				<Item><Source><LocURI>./DevDetail/Ext/Microsoft/DeviceName</LocURI></Source><Data>WIN-TESTPC</Data></Item>
				<Item><Source><LocURI>./DevDetail/Ext/Microsoft/TotalRAM</LocURI></Source><Data>8192</Data></Item>
				<Item><Source><LocURI>./Vendor/MSFT/BitLocker/Status/DeviceEncryptionStatus</LocURI></Source><Data>0</Data></Item>
			</Results>
			<Final/>
		</SyncBody>
	</SyncML>`, hwID)

	_, _, err = handler.HandleSyncML(ctx, []byte(resultsMsg))
	require.NoError(t, err)

	// Verify device info was updated in DB
	final, err := deviceRepo.GetByID(ctx, deviceID)
	require.NoError(t, err)
	assert.Equal(t, "10.0.22631", final.OSVersion)
	assert.Equal(t, "WIN-TESTPC", final.Name)
	assert.Equal(t, "8192", final.PlatformData["total_ram"])
	assert.Equal(t, "0", final.PlatformData["bitlocker_status"])
}

// TestE2E_WindowsDeviceIDMismatch verifies that HandleSyncML resolves a device
// when the OMA-DM sync ID differs from the enrollment device ID.
func TestE2E_WindowsDeviceIDMismatch(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	cmdRepo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Windows ID Mismatch Test",
		Slug:      "win-mismatch-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	deviceID := uuid.New()
	enrollmentHWID := "ENROLL-" + uuid.New().String()[:8]
	syncHWID := "SYNC-" + uuid.New().String()[:8]

	device := &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformWindows,
		DeviceID:     enrollmentHWID,
		Name:         "Mismatch Test PC",
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(ctx, device))

	handler := windows.NewManagementHandler("https://mdm.test", deviceRepo, cmdRepo, slog.Default())

	// Device syncs with a DIFFERENT ID but includes enrollment ID in Replace
	syncMsg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
	<SyncML xmlns="SYNCML:SYNCML1.2">
		<SyncHdr>
			<VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
			<SessionID>1</SessionID><MsgID>1</MsgID>
			<Target><LocURI>https://mdm.test</LocURI></Target>
			<Source><LocURI>%s</LocURI></Source>
		</SyncHdr>
		<SyncBody>
			<Alert><CmdID>1</CmdID><Data>1201</Data></Alert>
			<Replace><CmdID>2</CmdID>
				<Item><Source><LocURI>./DevInfo/DevId</LocURI></Source><Data>%s</Data></Item>
			</Replace>
			<Final/>
		</SyncBody>
	</SyncML>`, syncHWID, enrollmentHWID)

	_, _, err = handler.HandleSyncML(ctx, []byte(syncMsg))
	require.NoError(t, err)

	// Verify device_id was updated to the sync ID
	resolved, err := deviceRepo.GetByID(ctx, deviceID)
	require.NoError(t, err)
	assert.Equal(t, syncHWID, resolved.DeviceID, "device_id should be updated to sync ID")
	assert.Equal(t, enrollmentHWID, resolved.PlatformData["enrollment_device_id"], "original ID preserved")

	// Verify subsequent syncs work with the new ID
	syncMsg2 := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
	<SyncML xmlns="SYNCML:SYNCML1.2">
		<SyncHdr>
			<VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>
			<SessionID>2</SessionID><MsgID>1</MsgID>
			<Target><LocURI>https://mdm.test</LocURI></Target>
			<Source><LocURI>%s</LocURI></Source>
		</SyncHdr>
		<SyncBody>
			<Alert><CmdID>1</CmdID><Data>1201</Data></Alert>
			<Final/>
		</SyncBody>
	</SyncML>`, syncHWID)

	_, returnedID, err := handler.HandleSyncML(ctx, []byte(syncMsg2))
	require.NoError(t, err)
	assert.Equal(t, syncHWID, returnedID)
}
