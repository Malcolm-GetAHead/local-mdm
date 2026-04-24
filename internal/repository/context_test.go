package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
)

func TestDeviceRepository_List_ContextCancellation(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create test enterprise
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create enterprise repository: %v", err)
	}
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test Enterprise",
		Slug:      "test-ent-ctx-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(context.Background(), enterprise); err != nil {
		t.Fatalf("failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { enterpriseRepo.Delete(context.Background(), enterprise.ID) })

	// Create test devices
	deviceIDs := make([]uuid.UUID, 5)
	for i := 0; i < 5; i++ {
		device := &models.Device{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			EnterpriseID: enterprise.ID,
			Platform:     "ios",
			DeviceID:     uuid.New().String(),
			SerialNumber: uuid.New().String(),
			Name:         "Test Device",
		}
		if err := repo.Create(context.Background(), device); err != nil {
			t.Fatalf("failed to create device: %v", err)
		}
		deviceIDs[i] = device.ID
	}
	t.Cleanup(func() {
		for _, id := range deviceIDs {
			repo.Delete(context.Background(), id)
		}
	})

	t.Run("cancelled before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, _, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err == nil {
			t.Error("expected error from cancelled context, got nil")
		}
	})

	t.Run("cancelled with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond) // Ensure timeout

		_, _, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err == nil {
			t.Error("expected error from timed-out context, got nil")
		}
	})

	t.Run("not cancelled", func(t *testing.T) {
		// Use a fresh DB connection to avoid pool contamination from cancelled contexts
		freshDB := testutil.ConnectDB(t)
		freshRepo, err := NewDeviceRepository(freshDB.Writer, freshDB.Writer)
		if err != nil {
			t.Fatalf("failed to create fresh repository: %v", err)
		}

		devices, total, err := freshRepo.List(context.Background(), enterprise.ID, 10, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(devices) != 5 {
			t.Errorf("expected 5 devices, got %d", len(devices))
		}
		if total != 5 {
			t.Errorf("expected total 5, got %d", total)
		}
	})
}

func TestEnterpriseRepository_List_ContextCancellation(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create test enterprises
	entIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		enterprise := &models.Enterprise{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Name:      "Test Enterprise",
			Slug:      uuid.New().String(),
		}
		if err := repo.Create(context.Background(), enterprise); err != nil {
			t.Fatalf("failed to create enterprise: %v", err)
		}
		entIDs[i] = enterprise.ID
	}
	t.Cleanup(func() {
		for _, id := range entIDs {
			repo.Delete(context.Background(), id)
		}
	})

	t.Run("cancelled before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := repo.List(ctx, 10, 0)
		if err == nil {
			t.Error("expected error from cancelled context, got nil")
		}
	})

	t.Run("cancelled with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond)

		_, _, err := repo.List(ctx, 10, 0)
		if err == nil {
			t.Error("expected error from timed-out context, got nil")
		}
	})

	t.Run("not cancelled", func(t *testing.T) {
		freshDB := testutil.ConnectDB(t)
		freshRepo, err := NewEnterpriseRepository(freshDB.Writer, freshDB.Writer)
		if err != nil {
			t.Fatalf("failed to create fresh repository: %v", err)
		}
		ctx := context.Background()

		enterprises, total, err := freshRepo.List(ctx, 10, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(enterprises) < 3 {
			t.Errorf("expected at least 3 enterprises, got %d", len(enterprises))
		}
		if total < 3 {
			t.Errorf("expected total at least 3, got %d", total)
		}
	})
}

func TestPolicyRepository_List_ContextCancellation(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewPolicyRepository(db.Writer, db.Reader)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create test enterprise
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create enterprise repository: %v", err)
	}
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test Enterprise",
		Slug:      "test-pol-ctx-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(context.Background(), enterprise); err != nil {
		t.Fatalf("failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { enterpriseRepo.Delete(context.Background(), enterprise.ID) })

	// Create test policies
	policyIDs := make([]uuid.UUID, 4)
	for i := 0; i < 4; i++ {
		policy := &models.Policy{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			EnterpriseID: enterprise.ID,
			Name:         "Test Policy",
			Platform:     "ios",
			PolicyType:   "security",
			PolicyConfig: models.JSONB{},
		}
		if err := repo.Create(context.Background(), policy); err != nil {
			t.Fatalf("failed to create policy: %v", err)
		}
		policyIDs[i] = policy.ID
	}
	t.Cleanup(func() {
		for _, id := range policyIDs {
			repo.Delete(context.Background(), id)
		}
	})

	t.Run("cancelled before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err == nil {
			t.Error("expected error from cancelled context, got nil")
		}
	})

	t.Run("cancelled with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond)

		_, _, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err == nil {
			t.Error("expected error from timed-out context, got nil")
		}
	})

	t.Run("not cancelled", func(t *testing.T) {
		freshDB := testutil.ConnectDB(t)
		freshRepo, err := NewPolicyRepository(freshDB.Writer, freshDB.Reader)
		if err != nil {
			t.Fatalf("failed to create fresh repository: %v", err)
		}
		ctx := context.Background()

		policies, total, err := freshRepo.List(ctx, enterprise.ID, 10, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(policies) != 4 {
			t.Errorf("expected 4 policies, got %d", len(policies))
		}
		if total != 4 {
			t.Errorf("expected total 4, got %d", total)
		}
	})
}
