package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
)

func TestEnterpriseRepository(t *testing.T) {
	database := testutil.ConnectDB(t)
	
	repo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	ctx := context.Background()
	
	// Create
	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: "test-enterprise-" + uuid.New().String()[:8],
		Settings: models.JSONB{
			"feature_flags": map[string]bool{"mdm_enabled": true},
		},
	}
	
	err = repo.Create(ctx, enterprise)
	if err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })
	
	if enterprise.ID == uuid.Nil {
		t.Fatal("Enterprise ID should be set after create")
	}
	
	// Get by ID
	fetched, err := repo.GetByID(ctx, enterprise.ID)
	if err != nil {
		t.Fatalf("Failed to get enterprise: %v", err)
	}
	
	if fetched.Name != enterprise.Name {
		t.Errorf("Expected name %s, got %s", enterprise.Name, fetched.Name)
	}
	
	// Get by Slug
	fetchedBySlug, err := repo.GetBySlug(ctx, enterprise.Slug)
	if err != nil {
		t.Fatalf("Failed to get enterprise by slug: %v", err)
	}
	
	if fetchedBySlug.ID != enterprise.ID {
		t.Error("Enterprise IDs should match")
	}
	
	// Update
	enterprise.Name = "Updated Enterprise"
	err = repo.Update(ctx, enterprise)
	if err != nil {
		t.Fatalf("Failed to update enterprise: %v", err)
	}
	
	updated, _ := repo.GetByID(ctx, enterprise.ID)
	if updated.Name != "Updated Enterprise" {
		t.Error("Enterprise name should be updated")
	}
	
	// List
	enterprises, total, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list enterprises: %v", err)
	}
	
	if total == 0 {
		t.Error("Should have at least one enterprise")
	}
	
	if len(enterprises) == 0 {
		t.Error("Should return enterprises")
	}
	
	// Delete (soft delete)
	err = repo.Delete(ctx, enterprise.ID)
	if err != nil {
		t.Fatalf("Failed to delete enterprise: %v", err)
	}
	
	// Should not be found after delete
	_, err = repo.GetByID(ctx, enterprise.ID)
	if err == nil {
		t.Error("Should not find deleted enterprise")
	}
}

func TestDeviceRepository(t *testing.T) {
	database := testutil.ConnectDB(t)
	
	// Create enterprise first
	enterpriseRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}
	enterprise := &models.Enterprise{
		Name: "Device Test Enterprise",
		Slug: "device-test-" + uuid.New().String()[:8],
	}
	enterpriseRepo.Create(context.Background(), enterprise)
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })
	
	repo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}
	ctx := context.Background()
	
	// Create
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     "macos",
		DeviceID:     "test-device-" + uuid.New().String(),
		SerialNumber: "SN" + uuid.New().String()[:8],
		Name:         "Test MacBook",
		Model:        "MacBookPro18,1",
		OSVersion:    "14.0",
		Status:       "enrolled",
		PlatformData: models.JSONB{"test": "data"},
	}
	
	err = repo.Create(ctx, device)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	
	// Get by ID
	fetched, err := repo.GetByID(ctx, device.ID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	
	if fetched.Name != device.Name {
		t.Errorf("Expected name %s, got %s", device.Name, fetched.Name)
	}
	
	// Get by Serial
	fetchedBySerial, err := repo.GetBySerial(ctx, enterprise.ID, device.SerialNumber)
	if err != nil {
		t.Fatalf("Failed to get device by serial: %v", err)
	}
	
	if fetchedBySerial.ID != device.ID {
		t.Error("Device IDs should match")
	}
	
	// List
	devices, total, err := repo.List(ctx, enterprise.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}
	
	if total == 0 {
		t.Error("Should have at least one device")
	}
	
	if len(devices) == 0 {
		t.Error("Should return devices")
	}
	
	// Update
	device.Name = "Updated MacBook"
	err = repo.Update(ctx, device)
	if err != nil {
		t.Fatalf("Failed to update device: %v", err)
	}
	
	// Delete
	err = repo.Delete(ctx, device.ID)
	if err != nil {
		t.Fatalf("Failed to delete device: %v", err)
	}
	
	// Cleanup
	enterpriseRepo.Delete(ctx, enterprise.ID)
}

func TestPolicyRepository(t *testing.T) {
	database := testutil.ConnectDB(t)
	
	// Create enterprise first
	enterpriseRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}
	enterprise := &models.Enterprise{
		Name: "Policy Test Enterprise",
		Slug: "policy-test-" + uuid.New().String()[:8],
	}
	enterpriseRepo.Create(context.Background(), enterprise)
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })
	
	// Create device for assignment tests
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     "macos",
		DeviceID:     "test-device-" + uuid.New().String(),
		SerialNumber: "SN" + uuid.New().String()[:8],
		Name:         "Test Device",
		Status:       "enrolled",
	}
	deviceRepo.Create(context.Background(), device)
	
	repo, err := repository.NewPolicyRepository(database.Writer, database.Reader)
	if err != nil {
		t.Fatalf("Failed to create policy repository: %v", err)
	}
	ctx := context.Background()
	
	// Create
	policy := &models.Policy{
		EnterpriseID: enterprise.ID,
		Name:         "Test Policy",
		Description:  "Test Description",
		Platform:     "macos",
		PolicyType:   "wifi",
		PolicyConfig: models.JSONB{"ssid": "TestNetwork"},
		IsActive:     true,
	}
	
	err = repo.Create(ctx, policy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}
	
	// Get by ID
	fetched, err := repo.GetByID(ctx, policy.ID)
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}
	
	if fetched.Name != policy.Name {
		t.Errorf("Expected name %s, got %s", policy.Name, fetched.Name)
	}
	
	// List
	policies, total, err := repo.List(ctx, enterprise.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list policies: %v", err)
	}
	
	if total == 0 {
		t.Error("Should have at least one policy")
	}
	
	if len(policies) == 0 {
		t.Error("Should return policies")
	}
	
	// Update
	policy.Name = "Updated Policy"
	err = repo.Update(ctx, policy)
	if err != nil {
		t.Fatalf("Failed to update policy: %v", err)
	}
	
	updated, _ := repo.GetByID(ctx, policy.ID)
	if updated.Name != "Updated Policy" {
		t.Error("Policy name should be updated")
	}
	
	// Assign to device
	err = repo.AssignToDevice(ctx, device.ID, policy.ID)
	if err != nil {
		t.Fatalf("Failed to assign policy to device: %v", err)
	}
	
	// Unassign from device
	err = repo.UnassignFromDevice(ctx, device.ID, policy.ID)
	if err != nil {
		t.Fatalf("Failed to unassign policy from device: %v", err)
	}
	
	// Delete
	err = repo.Delete(ctx, policy.ID)
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}
	
	// Should not be found after delete
	_, err = repo.GetByID(ctx, policy.ID)
	if err == nil {
		t.Error("Should not find deleted policy")
	}
	
	// Cleanup
	deviceRepo.Delete(ctx, device.ID)
	enterpriseRepo.Delete(ctx, enterprise.ID)
}
