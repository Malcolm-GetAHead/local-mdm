package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
)

// Test Update method error paths
func TestDeviceRepository_Update_NotFound(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	device := &models.Device{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Updated Device",
	}

	err = repo.Update(context.Background(), device)
	if err == nil {
		t.Error("expected error for non-existent device")
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEnterpriseRepository_Update_NotFound(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Updated Enterprise",
		Slug:      "updated-slug",
	}

	err = repo.Update(context.Background(), enterprise)
	if err == nil {
		t.Error("expected error for non-existent enterprise")
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPolicyRepository_Update_NotFound(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewPolicyRepository(db.Writer, db.Reader)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	policy := &models.Policy{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		Name:         "Updated Policy",
		PolicyConfig: models.JSONB{},
	}

	err = repo.Update(context.Background(), policy)
	if err == nil {
		t.Error("expected error for non-existent policy")
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// Test Delete method error paths
func TestDeviceRepository_Delete_NotFound(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	err = repo.Delete(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error for non-existent device")
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEnterpriseRepository_Delete_NotFound(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	err = repo.Delete(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error for non-existent enterprise")
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPolicyRepository_Delete_NotFound(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewPolicyRepository(db.Writer, db.Reader)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	err = repo.Delete(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error for non-existent policy")
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// Test GetBySlug error path
func TestEnterpriseRepository_GetBySlug_NotFound(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	_, err = repo.GetBySlug(context.Background(), "non-existent-slug")
	if err == nil {
		t.Error("expected error for non-existent slug")
	}
}

// Test soft delete behavior
func TestDeviceRepository_Delete_SoftDelete(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create enterprise
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create enterprise repository: %v", err)
	}
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test Enterprise",
		Slug:      "test-ent-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(context.Background(), enterprise); err != nil {
		t.Fatalf("failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	// Create device
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

	// Delete device
	if err := repo.Delete(context.Background(), device.ID); err != nil {
		t.Fatalf("failed to delete device: %v", err)
	}

	// Try to delete again (should fail - already deleted)
	err = repo.Delete(context.Background(), device.ID)
	if err == nil {
		t.Error("expected error when deleting already-deleted device")
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Verify device not in list
	devices, total, err := repo.List(context.Background(), enterprise.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 devices after delete, got %d", total)
	}
	if len(devices) != 0 {
		t.Errorf("expected empty device list, got %d devices", len(devices))
	}
}

// Test List with empty results
func TestDeviceRepository_List_EmptyResults(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	devices, total, err := repo.List(context.Background(), uuid.New(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 total, got %d", total)
	}
	if len(devices) != 0 {
		t.Errorf("expected empty list, got %d devices", len(devices))
	}
}

func TestEnterpriseRepository_List_EmptyResults(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	// Create and delete an enterprise to test empty results
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Test Enterprise",
		Slug:      "test-empty-" + uuid.New().String()[:8],
	}
	if err := repo.Create(context.Background(), enterprise); err != nil {
		t.Fatalf("failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })
	if err := repo.Delete(context.Background(), enterprise.ID); err != nil {
		t.Fatalf("failed to delete enterprise: %v", err)
	}

	// Verify GetByID returns error for deleted enterprise
	_, err = repo.GetByID(context.Background(), enterprise.ID)
	if err == nil {
		t.Error("expected error when getting deleted enterprise")
	}
}

func TestPolicyRepository_List_EmptyResults(t *testing.T) {
	db := testutil.ConnectDB(t)
	repo, err := NewPolicyRepository(db.Writer, db.Reader)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	policies, total, err := repo.List(context.Background(), uuid.New(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 total, got %d", total)
	}
	if len(policies) != 0 {
		t.Errorf("expected empty list, got %d policies", len(policies))
	}
}
