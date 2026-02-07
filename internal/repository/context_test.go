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
	db := testutil.SetupTestDB(t)
	repo, err := NewDeviceRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create test enterprise
	enterpriseRepo, err := NewEnterpriseRepository(db)
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

	// Create test devices
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
	}

	t.Run("cancelled before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		devices, total, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if devices != nil {
			t.Errorf("expected nil devices, got %v", devices)
		}
		if total != 0 {
			t.Errorf("expected 0 total, got %d", total)
		}
	})

	t.Run("cancelled with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond) // Ensure timeout

		devices, total, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
		if devices != nil {
			t.Errorf("expected nil devices, got %v", devices)
		}
		if total != 0 {
			t.Errorf("expected 0 total, got %d", total)
		}
	})

	t.Run("not cancelled", func(t *testing.T) {
		ctx := context.Background()

		devices, total, err := repo.List(ctx, enterprise.ID, 10, 0)
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
	db := testutil.SetupTestDB(t)
	repo, err := NewEnterpriseRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create test enterprises
	for i := 0; i < 3; i++ {
		enterprise := &models.Enterprise{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Name:      "Test Enterprise",
			Slug:      uuid.New().String(),
		}
		if err := repo.Create(context.Background(), enterprise); err != nil {
			t.Fatalf("failed to create enterprise: %v", err)
		}
	}

	t.Run("cancelled before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		enterprises, total, err := repo.List(ctx, 10, 0)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if enterprises != nil {
			t.Errorf("expected nil enterprises, got %v", enterprises)
		}
		if total != 0 {
			t.Errorf("expected 0 total, got %d", total)
		}
	})

	t.Run("cancelled with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond)

		enterprises, total, err := repo.List(ctx, 10, 0)
		if err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
		if enterprises != nil {
			t.Errorf("expected nil enterprises, got %v", enterprises)
		}
		if total != 0 {
			t.Errorf("expected 0 total, got %d", total)
		}
	})

	t.Run("not cancelled", func(t *testing.T) {
		ctx := context.Background()

		enterprises, total, err := repo.List(ctx, 10, 0)
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
	db := testutil.SetupTestDB(t)
	repo, err := NewPolicyRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create test enterprise
	enterpriseRepo, err := NewEnterpriseRepository(db)
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

	// Create test policies
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
	}

	t.Run("cancelled before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		policies, total, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if policies != nil {
			t.Errorf("expected nil policies, got %v", policies)
		}
		if total != 0 {
			t.Errorf("expected 0 total, got %d", total)
		}
	})

	t.Run("cancelled with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond)

		policies, total, err := repo.List(ctx, enterprise.ID, 10, 0)
		if err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
		if policies != nil {
			t.Errorf("expected nil policies, got %v", policies)
		}
		if total != 0 {
			t.Errorf("expected 0 total, got %d", total)
		}
	})

	t.Run("not cancelled", func(t *testing.T) {
		ctx := context.Background()

		policies, total, err := repo.List(ctx, enterprise.ID, 10, 0)
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
