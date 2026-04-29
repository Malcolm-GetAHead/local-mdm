package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
)

func TestDeviceRepository_JSONBValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.ConnectDB(t)

	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()
	
	// Create enterprise for foreign key
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create enterprise repository: %v", err)
	}
	enterprise := testutil.NewTestEnterprise(t)
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("failed to create test enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	t.Run("create with oversized JSONB", func(t *testing.T) {
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "ios",
			DeviceID:     "test-device",
			SerialNumber: "SN123",
			Name:         "Test Device",
			Status:       "active",
			PlatformData: models.JSONB{"data": strings.Repeat("x", 2<<20)},
		}

		err := repo.Create(ctx, device)
		if err == nil {
			t.Error("expected error for oversized JSONB, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Errorf("expected size error, got: %v", err)
		}
	})

	t.Run("create with deeply nested JSONB", func(t *testing.T) {
		deepNested := models.JSONB{
			"l1": map[string]interface{}{
				"l2": map[string]interface{}{
					"l3": map[string]interface{}{
						"l4": map[string]interface{}{
							"l5": map[string]interface{}{
								"l6": map[string]interface{}{
									"l7": map[string]interface{}{
										"l8": map[string]interface{}{
											"l9": map[string]interface{}{
												"l10": map[string]interface{}{
													"l11": "too deep",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "ios",
			DeviceID:     "test-device-2",
			SerialNumber: "SN124",
			Name:         "Test Device 2",
			Status:       "active",
			PlatformData: deepNested,
		}

		err := repo.Create(ctx, device)
		if err == nil {
			t.Error("expected error for deeply nested JSONB, got nil")
		}
		if !strings.Contains(err.Error(), "nesting depth") {
			t.Errorf("expected depth error, got: %v", err)
		}
	})

	t.Run("create with valid JSONB", func(t *testing.T) {
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "ios",
			DeviceID:     "test-device-3",
			SerialNumber: "SN125",
			Name:         "Test Device 3",
			Status:       "active",
			PlatformData: models.JSONB{"model": "iPhone 14", "version": "16.0"},
		}

		err := repo.Create(ctx, device)
		if err != nil {
			t.Errorf("unexpected error for valid JSONB: %v", err)
		}
	})

	t.Run("update with invalid JSONB", func(t *testing.T) {
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "ios",
			DeviceID:     "test-device-4",
			SerialNumber: "SN126",
			Name:         "Test Device 4",
			Status:       "active",
			PlatformData: models.JSONB{"valid": "data"},
		}

		if err := repo.Create(ctx, device); err != nil {
			t.Fatalf("failed to create device: %v", err)
		}

		device.PlatformData = models.JSONB{"data": strings.Repeat("x", 2<<20)}
		err := repo.Update(ctx, device)
		if err == nil {
			t.Error("expected error for oversized JSONB in update, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Errorf("expected size error, got: %v", err)
		}
	})

	t.Run("create with nil JSONB", func(t *testing.T) {
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "ios",
			DeviceID:     "test-device-5",
			SerialNumber: "SN127",
			Name:         "Test Device 5",
			Status:       "active",
			PlatformData: nil,
		}

		err := repo.Create(ctx, device)
		if err != nil {
			t.Errorf("unexpected error for nil JSONB: %v", err)
		}
	})

	t.Run("error message contains field name", func(t *testing.T) {
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "ios",
			DeviceID:     "test-device-6",
			SerialNumber: "SN128",
			Name:         "Test Device 6",
			Status:       "active",
			PlatformData: models.JSONB{"data": strings.Repeat("x", 2<<20)},
		}

		err := repo.Create(ctx, device)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "platform_data") {
			t.Errorf("error should mention field name 'platform_data', got: %v", err)
		}
	})
}

func TestEnterpriseRepository_JSONBValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.ConnectDB(t)

	repo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	t.Run("create with oversized JSONB", func(t *testing.T) {
		enterprise := testutil.NewTestEnterprise(t)
		enterprise.Settings = models.JSONB{"data": strings.Repeat("x", 2<<20)}

		err := repo.Create(ctx, enterprise)
		if err == nil {
			t.Error("expected error for oversized JSONB, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Errorf("expected size error, got: %v", err)
		}
	})

	t.Run("create with valid JSONB", func(t *testing.T) {
		enterprise := testutil.NewTestEnterprise(t)
		enterprise.Settings = models.JSONB{"theme": "dark", "notifications": true}

		err := repo.Create(ctx, enterprise)
		if err != nil {
			t.Errorf("unexpected error for valid JSONB: %v", err)
		}
		t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })
	})

	t.Run("create with nil JSONB", func(t *testing.T) {
		enterprise := testutil.NewTestEnterprise(t)
		enterprise.Settings = nil

		err := repo.Create(ctx, enterprise)
		if err != nil {
			t.Errorf("unexpected error for nil JSONB: %v", err)
		}
		t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })
	})

	t.Run("update with invalid JSONB", func(t *testing.T) {
		enterprise := testutil.NewTestEnterprise(t)
		enterprise.Settings = models.JSONB{"valid": "data"}

		if err := repo.Create(ctx, enterprise); err != nil {
			t.Fatalf("failed to create enterprise: %v", err)
		}
		t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

		enterprise.Settings = models.JSONB{"data": strings.Repeat("x", 2<<20)}
		err := repo.Update(ctx, enterprise)
		if err == nil {
			t.Error("expected error for oversized JSONB in update, got nil")
		}
		if !strings.Contains(err.Error(), "settings") {
			t.Errorf("error should mention field name 'settings', got: %v", err)
		}
	})
}

func TestPolicyRepository_JSONBValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.ConnectDB(t)

	repo, err := NewPolicyRepository(db.Writer, db.Reader)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()
	
	// Create enterprise for foreign key
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create enterprise repository: %v", err)
	}
	enterprise := testutil.NewTestEnterprise(t)
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("failed to create test enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	t.Run("create with oversized JSONB", func(t *testing.T) {
		policy := &models.Policy{
			EnterpriseID: enterprise.ID,
			Name:         "Test Policy",
			Platform:     "ios",
			PolicyType:   "wifi",
			PolicyConfig: models.JSONB{"data": strings.Repeat("x", 2<<20)},
			IsActive:     true,
		}

		err := repo.Create(ctx, policy)
		if err == nil {
			t.Error("expected error for oversized JSONB, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Errorf("expected size error, got: %v", err)
		}
	})

	t.Run("create with valid JSONB", func(t *testing.T) {
		policy := &models.Policy{
			EnterpriseID: enterprise.ID,
			Name:         "Test Policy 2",
			Platform:     "ios",
			PolicyType:   "wifi",
			PolicyConfig: models.JSONB{"ssid": "TestNetwork", "security": "WPA2"},
			IsActive:     true,
		}

		err := repo.Create(ctx, policy)
		if err != nil {
			t.Errorf("unexpected error for valid JSONB: %v", err)
		}
	})

	t.Run("create with empty JSONB", func(t *testing.T) {
		policy := &models.Policy{
			EnterpriseID: enterprise.ID,
			Name:         "Test Policy 3",
			Platform:     "ios",
			PolicyType:   "restriction",
			PolicyConfig: models.JSONB{},
			IsActive:     true,
		}

		err := repo.Create(ctx, policy)
		if err != nil {
			t.Errorf("unexpected error for empty JSONB: %v", err)
		}
	})

	t.Run("update with deeply nested JSONB", func(t *testing.T) {
		policy := &models.Policy{
			EnterpriseID: enterprise.ID,
			Name:         "Test Policy 4",
			Platform:     "ios",
			PolicyType:   "wifi",
			PolicyConfig: models.JSONB{"valid": "data"},
			IsActive:     true,
		}

		if err := repo.Create(ctx, policy); err != nil {
			t.Fatalf("failed to create policy: %v", err)
		}

		deepNested := models.JSONB{
			"l1": map[string]interface{}{
				"l2": map[string]interface{}{
					"l3": map[string]interface{}{
						"l4": map[string]interface{}{
							"l5": map[string]interface{}{
								"l6": map[string]interface{}{
									"l7": map[string]interface{}{
										"l8": map[string]interface{}{
											"l9": map[string]interface{}{
												"l10": map[string]interface{}{
													"l11": "too deep",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		policy.PolicyConfig = deepNested

		err := repo.Update(ctx, policy)
		if err == nil {
			t.Error("expected error for deeply nested JSONB in update, got nil")
		}
		if !strings.Contains(err.Error(), "policy_config") {
			t.Errorf("error should mention field name 'policy_config', got: %v", err)
		}
		if !strings.Contains(err.Error(), "nesting depth") {
			t.Errorf("error should mention nesting depth, got: %v", err)
		}
	})
}
