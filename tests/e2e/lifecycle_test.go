package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func setupDB(t *testing.T) *db.DB {
	t.Helper()
	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "postgres", Password: "postgres",
		Database: "localmdm", SSLMode: "disable", MaxOpenConns: 2, MaxIdleConns: 1, ConnMaxLifetime: 5 * time.Minute,
	}
	database, err := db.New(cfg)
	if err != nil {
		t.Skipf("skipping E2E: %v", err)
	}
	return database
}

func TestE2E_DeviceLifecycle(t *testing.T) {
	database := setupDB(t)
	defer database.Close()
	ctx := context.Background()
	logger := slog.Default()

	// Create repos
	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	policyRepo, err := repository.NewPolicyRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	cmdRepo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	groupRepo, err := repository.NewGroupRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	assignRepo, err := repository.NewPolicyAssignmentRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	complianceRepo, err := repository.NewComplianceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	// Create services
	lifecycleSvc := service.NewLifecycleService(logger)
	groupSvc := service.NewGroupService(groupRepo, assignRepo, logger)
	complianceSvc := service.NewComplianceService(complianceRepo, groupSvc, policyRepo, deviceRepo, logger)

	// 1. Create enterprise
	enterprise := &models.Enterprise{Name: "e2e-test-" + uuid.New().String()[:8], Slug: "e2e-" + uuid.New().String()[:8]}
	require.NoError(t, entRepo.Create(ctx, enterprise))
	t.Cleanup(func() { entRepo.Delete(ctx, enterprise.ID) })

	// 2. Enroll a device (simulated)
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformWindows,
		DeviceID:     "E2E-WIN-" + uuid.New().String()[:8],
		Name:         "E2E Test Device",
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{
			"encryption_enabled": true,
			"firewall_enabled":   true,
			"password_present":   true,
		},
	}
	require.NoError(t, deviceRepo.Create(ctx, device))
	t.Cleanup(func() { deviceRepo.Delete(ctx, device.ID) })

	// 3. Create a security policy
	policy := &models.Policy{
		EnterpriseID: enterprise.ID,
		Name:         "E2E Security Policy",
		Platform:     models.PlatformWindows,
		PolicyType:   "security",
		PolicyConfig: models.JSONB{
			"require_encryption": true,
			"require_firewall":   true,
			"require_password":   true,
		},
		IsActive: true,
	}
	require.NoError(t, policyRepo.Create(ctx, policy))
	t.Cleanup(func() { policyRepo.Delete(ctx, policy.ID) })

	// 4. Create group and add device
	group := &models.DeviceGroup{EnterpriseID: enterprise.ID, Name: "E2E Group"}
	require.NoError(t, groupRepo.Create(ctx, group))
	t.Cleanup(func() { groupRepo.Delete(ctx, group.ID) })
	require.NoError(t, groupRepo.AddMember(ctx, group.ID, device.ID))

	// 5. Assign policy to group
	_, err = groupSvc.AssignPolicy(ctx, policy.ID, models.TargetTypeGroup, group.ID, 1)
	require.NoError(t, err)

	// 6. Evaluate compliance — device should be compliant
	results, err := complianceSvc.EvaluateDevice(ctx, device.ID, enterprise.ID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status)

	// 7. Update device to be non-compliant (disable encryption)
	device.PlatformData["encryption_enabled"] = false
	require.NoError(t, deviceRepo.Update(ctx, device))

	// 8. Re-evaluate — should be non-compliant
	results, err = complianceSvc.EvaluateDevice(ctx, device.ID, enterprise.ID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, models.ComplianceStatusNonCompliant, results[0].Status)

	// 9. Send lock command
	lockCmd := &models.DeviceCommand{DeviceID: device.ID, CommandType: models.CommandTypeLock}
	require.NoError(t, cmdRepo.Create(ctx, lockCmd))
	assert.Equal(t, models.CommandStatusPending, lockCmd.Status)

	// 10. Mark command sent and completed
	require.NoError(t, cmdRepo.MarkSent(ctx, lockCmd.ID))
	require.NoError(t, cmdRepo.MarkCompleted(ctx, lockCmd.ID))

	// 11. Verify command lifecycle
	fetched, err := cmdRepo.GetByID(ctx, lockCmd.ID)
	require.NoError(t, err)
	assert.Equal(t, models.CommandStatusCompleted, fetched.Status)
	assert.NotNil(t, fetched.SentAt)
	assert.NotNil(t, fetched.CompletedAt)

	// 12. Delete device (lifecycle hooks)
	_ = lifecycleSvc
	require.NoError(t, deviceRepo.Delete(ctx, device.ID))

	// 13. Verify device is soft-deleted
	_, err = deviceRepo.GetByID(ctx, device.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestE2E_CrossPlatformPolicy(t *testing.T) {
	database := setupDB(t)
	defer database.Close()
	ctx := context.Background()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	policyRepo, err := repository.NewPolicyRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	groupRepo, err := repository.NewGroupRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	assignRepo, err := repository.NewPolicyAssignmentRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	complianceRepo, err := repository.NewComplianceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	groupSvc := service.NewGroupService(groupRepo, assignRepo, slog.Default())
	complianceSvc := service.NewComplianceService(complianceRepo, groupSvc, policyRepo, deviceRepo, slog.Default())

	enterprise := &models.Enterprise{Name: "e2e-xplat-" + uuid.New().String()[:8], Slug: "e2e-xplat-" + uuid.New().String()[:8]}
	require.NoError(t, entRepo.Create(ctx, enterprise))
	t.Cleanup(func() { entRepo.Delete(ctx, enterprise.ID) })

	// Create devices for each platform
	platforms := []struct {
		platform string
		data     models.JSONB
	}{
		{models.PlatformWindows, models.JSONB{"encryption_enabled": true, "firewall_enabled": true}},
		{models.PlatformMacOS, models.JSONB{"FileVaultEnabled": true, "FirewallEnabled": true}},
		{models.PlatformAndroid, models.JSONB{"encryption_enabled": true}},
	}

	var deviceIDs []uuid.UUID
	for _, p := range platforms {
		d := &models.Device{
			EnterpriseID: enterprise.ID, Platform: p.platform,
			DeviceID: "E2E-" + p.platform + "-" + uuid.New().String()[:8],
			Status: models.DeviceStatusEnrolled, PlatformData: p.data,
		}
		require.NoError(t, deviceRepo.Create(ctx, d))
		deviceIDs = append(deviceIDs, d.ID)
		t.Cleanup(func() { deviceRepo.Delete(ctx, d.ID) })
	}

	// Enterprise-wide security policy
	policy := &models.Policy{
		EnterpriseID: enterprise.ID, Name: "Enterprise Security",
		Platform: "all", PolicyType: "security",
		PolicyConfig: models.JSONB{"require_encryption": true, "require_firewall": true},
		IsActive: true,
	}
	require.NoError(t, policyRepo.Create(ctx, policy))
	t.Cleanup(func() { policyRepo.Delete(ctx, policy.ID) })

	// Assign to enterprise
	_, err = groupSvc.AssignPolicy(ctx, policy.ID, models.TargetTypeEnterprise, enterprise.ID, 100)
	require.NoError(t, err)

	// Evaluate each device
	for i, did := range deviceIDs {
		results, err := complianceSvc.EvaluateDevice(ctx, did, enterprise.ID)
		require.NoError(t, err)
		require.Len(t, results, 1, "device %d should have 1 result", i)
		// Windows and macOS have firewall data, Android doesn't
		if platforms[i].platform == models.PlatformAndroid {
			assert.Equal(t, models.ComplianceStatusNonCompliant, results[0].Status, "Android missing firewall")
		} else {
			assert.Equal(t, models.ComplianceStatusCompliant, results[0].Status, "%s should be compliant", platforms[i].platform)
		}
	}

	// Get enterprise compliance summary
	summary, err := complianceSvc.GetSummary(ctx, enterprise.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, summary.Total)
	assert.Equal(t, 2, summary.Compliant)
	assert.Equal(t, 1, summary.NonCompliant)
}
