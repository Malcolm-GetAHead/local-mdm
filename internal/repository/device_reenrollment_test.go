package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceRepository_ReenrollmentAfterSoftDelete(t *testing.T) {
	db := testutil.ConnectDB(t)
	entID := testutil.CreateTestEnterprise(t, db.Writer, "inttest-reenroll")

	repo, err := repository.NewDeviceRepository(db.Writer, db.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	deviceID := "reenroll-udid-" + uuid.New().String()[:8]

	// Step 1: Create device
	device := &models.Device{
		EnterpriseID: entID,
		Platform:     "macos",
		DeviceID:     deviceID,
		SerialNumber: "SN-ORIG",
		Name:         "Original Device",
		Model:        "MacBookPro",
		OSVersion:    "14.0",
		Status:       "enrolled",
		PlatformData: models.JSONB{"test": true},
	}
	err = repo.Create(ctx, device)
	require.NoError(t, err)
	originalID := device.ID
	require.NotEqual(t, uuid.Nil, originalID)

	// Step 2: Soft-delete the device
	err = repo.Delete(ctx, originalID)
	require.NoError(t, err)

	// Verify it's gone from normal queries
	_, err = repo.GetByID(ctx, originalID)
	require.Error(t, err, "soft-deleted device should not be found by GetByID")

	// Step 3: Re-enroll with same enterprise/platform/device_id
	newDevice := &models.Device{
		EnterpriseID: entID,
		Platform:     "macos",
		DeviceID:     deviceID,
		SerialNumber: "SN-NEW",
		Name:         "Re-enrolled Device",
		Model:        "MacBookPro",
		OSVersion:    "15.0",
		Status:       "pending",
		PlatformData: models.JSONB{"reenrolled": true},
	}
	err = repo.Create(ctx, newDevice)
	require.NoError(t, err)

	// Verify: same UUID is returned (restored, not new)
	assert.Equal(t, originalID, newDevice.ID, "re-enrollment should restore the original device UUID")
	assert.Equal(t, "enrolled", newDevice.Status, "restored device should have enrolled status")
	assert.Nil(t, newDevice.DeletedAt, "restored device should have nil deleted_at")

	// Verify: device is visible again via normal queries
	fetched, err := repo.GetByID(ctx, originalID)
	require.NoError(t, err)
	assert.Equal(t, "SN-NEW", fetched.SerialNumber, "serial should be updated on re-enrollment")
	assert.Equal(t, "Re-enrolled Device", fetched.Name)
	assert.Equal(t, "15.0", fetched.OSVersion)
	assert.Equal(t, "enrolled", fetched.Status)
	assert.Nil(t, fetched.DeletedAt)
}

func TestDeviceRepository_CreateDuplicateActiveDevice(t *testing.T) {
	db := testutil.ConnectDB(t)
	entID := testutil.CreateTestEnterprise(t, db.Writer, "inttest-dup-active")

	repo, err := repository.NewDeviceRepository(db.Writer, db.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	deviceID := "dup-active-" + uuid.New().String()[:8]

	// Create first device (active)
	device := &models.Device{
		EnterpriseID: entID,
		Platform:     "macos",
		DeviceID:     deviceID,
		Status:       "enrolled",
		PlatformData: models.JSONB{},
	}
	err = repo.Create(ctx, device)
	require.NoError(t, err)

	// Try to create duplicate (same enterprise/platform/device_id, still active)
	dup := &models.Device{
		EnterpriseID: entID,
		Platform:     "macos",
		DeviceID:     deviceID,
		Status:       "pending",
		PlatformData: models.JSONB{},
	}
	err = repo.Create(ctx, dup)
	require.Error(t, err, "creating a duplicate active device should fail")
	assert.Contains(t, err.Error(), "already exists")
}

func TestDeviceRepository_DifferentEnterpriseSameUDID(t *testing.T) {
	db := testutil.ConnectDB(t)
	entID1 := testutil.CreateTestEnterprise(t, db.Writer, "inttest-ent1")
	entID2 := testutil.CreateTestEnterprise(t, db.Writer, "inttest-ent2")

	repo, err := repository.NewDeviceRepository(db.Writer, db.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	deviceID := "multi-ent-" + uuid.New().String()[:8]

	// Create device in enterprise 1
	d1 := &models.Device{
		EnterpriseID: entID1,
		Platform:     "macos",
		DeviceID:     deviceID,
		Status:       "enrolled",
		PlatformData: models.JSONB{},
	}
	err = repo.Create(ctx, d1)
	require.NoError(t, err)

	// Create device with same UDID in enterprise 2 — should succeed (different enterprise)
	d2 := &models.Device{
		EnterpriseID: entID2,
		Platform:     "macos",
		DeviceID:     deviceID,
		Status:       "enrolled",
		PlatformData: models.JSONB{},
	}
	err = repo.Create(ctx, d2)
	require.NoError(t, err)
	assert.NotEqual(t, d1.ID, d2.ID, "different enterprises should get different device UUIDs")
}

func TestDeviceRepository_GetByPlatformIDIncludeDeleted(t *testing.T) {
	db := testutil.ConnectDB(t)
	entID := testutil.CreateTestEnterprise(t, db.Writer, "inttest-include-deleted")

	repo, err := repository.NewDeviceRepository(db.Writer, db.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	deviceID := "incl-del-" + uuid.New().String()[:8]

	// Create and soft-delete a device
	device := &models.Device{
		EnterpriseID: entID,
		Platform:     "macos",
		DeviceID:     deviceID,
		Status:       "enrolled",
		PlatformData: models.JSONB{},
	}
	err = repo.Create(ctx, device)
	require.NoError(t, err)
	originalID := device.ID

	err = repo.Delete(ctx, originalID)
	require.NoError(t, err)

	// Normal lookup should fail
	_, err = repo.GetByPlatformID(ctx, "macos", deviceID)
	require.Error(t, err)

	// IncludeDeleted lookup should find it
	found, err := repo.GetByPlatformIDIncludeDeleted(ctx, "macos", deviceID)
	require.NoError(t, err)
	assert.Equal(t, originalID, found.ID)
	assert.Equal(t, entID, found.EnterpriseID)
	assert.NotNil(t, found.DeletedAt, "should have deleted_at set")
}
