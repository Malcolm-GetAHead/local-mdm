package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
)

func TestTransactionCommit(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	ctx := context.Background()

	// Create enterprise and device in a transaction
	var enterpriseID, deviceID uuid.UUID
	slug := "test-enterprise-" + uuid.New().String()[:8]
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create enterprise
		enterprise := &models.Enterprise{
			Name: "Test Enterprise",
			Slug: slug,
		}
		if err := enterpriseRepo.Create(txCtx, enterprise); err != nil {
			return err
		}
		enterpriseID = enterprise.ID

		// Create device
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "macos",
			DeviceID:     "test-device-id",
			SerialNumber: "TEST123-" + uuid.New().String()[:8],
			Name:         "Test Device",
			Status:       "active",
		}
		if err := deviceRepo.Create(txCtx, device); err != nil {
			return err
		}
		deviceID = device.ID

		return nil
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterpriseID) })

	// Verify both records exist
	enterprise, err := enterpriseRepo.GetByID(ctx, enterpriseID)
	if err != nil {
		t.Fatalf("Failed to get enterprise: %v", err)
	}
	if enterprise.Name != "Test Enterprise" {
		t.Errorf("Expected enterprise name 'Test Enterprise', got '%s'", enterprise.Name)
	}

	device, err := deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	if device.Name != "Test Device" {
		t.Errorf("Expected device name 'Test Device', got '%s'", device.Name)
	}
}

func TestTransactionRollback(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	ctx := context.Background()

	// Create enterprise first (outside transaction)
	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: "test-enterprise-rollback-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	// Try to create device in transaction that will fail
	testError := errors.New("intentional error")
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create device
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "macos",
			DeviceID:     "test-device-id",
			SerialNumber: "ROLLBACK123",
			Name:         "Test Device",
			Status:       "active",
		}
		if err := deviceRepo.Create(txCtx, device); err != nil {
			return err
		}

		// Return error to trigger rollback
		return testError
	})

	if err != testError {
		t.Fatalf("Expected error %v, got %v", testError, err)
	}

	// Verify device was NOT created
	_, err = deviceRepo.GetBySerial(ctx, enterprise.ID, "ROLLBACK123")
	if err == nil {
		t.Fatal("Expected device to not exist after rollback, but it does")
	}
}

func TestTransactionRollbackOnPanic(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	ctx := context.Background()

	// Create enterprise first
	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: "test-enterprise-panic-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	// Try to create device in transaction that will panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic, but didn't get one")
		}
	}()

	_ = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create device
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "macos",
			DeviceID:     "test-device-id",
			SerialNumber: "PANIC123",
			Name:         "Test Device",
			Status:       "active",
		}
		if err := deviceRepo.Create(txCtx, device); err != nil {
			return err
		}

		// Panic to trigger rollback
		panic("intentional panic")
	})

	// This code should not be reached due to panic
	// But if it is, verify device was NOT created
	_, err = deviceRepo.GetBySerial(ctx, enterprise.ID, "PANIC123")
	if err == nil {
		t.Fatal("Expected device to not exist after panic rollback, but it does")
	}
}

func TestNestedTransactions(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	ctx := context.Background()

	// Nested transactions should use the same transaction
	var enterpriseID, deviceID uuid.UUID
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create enterprise
		enterprise := &models.Enterprise{
			Name: "Test Enterprise",
			Slug: "test-enterprise-nested-" + uuid.New().String()[:8],
		}
		if err := enterpriseRepo.Create(txCtx, enterprise); err != nil {
			return err
		}
		enterpriseID = enterprise.ID

		// Nested transaction (should reuse same tx)
		return transactor.WithTransaction(txCtx, func(nestedCtx context.Context) error {
			device := &models.Device{
				EnterpriseID: enterprise.ID,
				Platform:     "macos",
				DeviceID:     "test-device-id",
				SerialNumber: "NESTED123",
				Name:         "Test Device",
				Status:       "active",
			}
			if err := deviceRepo.Create(nestedCtx, device); err != nil {
				return err
			}
			deviceID = device.ID
			return nil
		})
	})

	if err != nil {
		t.Fatalf("Nested transaction failed: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterpriseID) })

	// Verify both records exist
	_, err = enterpriseRepo.GetByID(ctx, enterpriseID)
	if err != nil {
		t.Fatalf("Failed to get enterprise: %v", err)
	}

	_, err = deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
}

func TestTransactionWithMultipleOperations(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}
	policyRepo, err := NewPolicyRepository(db.Writer, db.Reader)
	if err != nil {
		t.Fatalf("Failed to create policy repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create policy repository: %v", err)

	}

	ctx := context.Background()

	// Complex transaction with multiple operations
	var enterpriseID, deviceID, policyID uuid.UUID
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create enterprise
		enterprise := &models.Enterprise{
			Name: "Test Enterprise",
			Slug: "test-enterprise-multi-" + uuid.New().String()[:8],
		}
		if err := enterpriseRepo.Create(txCtx, enterprise); err != nil {
			return err
		}
		enterpriseID = enterprise.ID

		// Create device
		device := &models.Device{
			EnterpriseID: enterprise.ID,
			Platform:     "macos",
			DeviceID:     "test-device-id",
			SerialNumber: "MULTI123",
			Name:         "Test Device",
			Status:       "active",
		}
		if err := deviceRepo.Create(txCtx, device); err != nil {
			return err
		}
		deviceID = device.ID

		// Create policy
		policy := &models.Policy{
			EnterpriseID: enterprise.ID,
			Name:         "Test Policy",
			Platform:     "macos",
			PolicyType:   "security",
			PolicyConfig: models.JSONB{"test": true},
			IsActive:     true,
		}
		if err := policyRepo.Create(txCtx, policy); err != nil {
			return err
		}
		policyID = policy.ID

		// Assign policy to device
		if err := policyRepo.AssignToDevice(txCtx, device.ID, policy.ID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Multi-operation transaction failed: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterpriseID) })

	// Verify all records exist
	_, err = enterpriseRepo.GetByID(ctx, enterpriseID)
	if err != nil {
		t.Fatalf("Failed to get enterprise: %v", err)
	}

	_, err = deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}

	_, err = policyRepo.GetByID(ctx, policyID)
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}
}

func TestGetExecutor(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	// Without transaction, should return db.Writer
	exec := getExecutor(ctx, db.Writer)
	if exec != db.Writer {
		t.Error("Expected executor to be db.Writer when no transaction in context")
	}

	// With transaction, should return tx
	tx, err := db.Writer.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, txKey{}, tx)
	exec = getExecutor(txCtx, db.Writer)
	if exec != tx {
		t.Error("Expected executor to be tx when transaction in context")
	}
}

func TestGetTx(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	// Without transaction
	tx := getTx(ctx)
	if tx != nil {
		t.Error("Expected nil when no transaction in context")
	}

	// With transaction
	dbTx, err := db.Writer.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer dbTx.Rollback()

	txCtx := context.WithValue(ctx, txKey{}, dbTx)
	tx = getTx(txCtx)
	if tx != dbTx {
		t.Error("Expected to get transaction from context")
	}
}

func TestTransactionUpdateOperations(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	ctx := context.Background()

	// Create enterprise and device first
	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: "test-update-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     "macos",
		DeviceID:     "test-device-id",
		SerialNumber: "UPDATE123-" + uuid.New().String()[:8],
		Name:         "Original Name",
		Status:       "active",
	}
	if err := deviceRepo.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	// Update in transaction
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		device.Name = "Updated Name"
		if err := deviceRepo.Update(txCtx, device); err != nil {
			return err
		}

		enterprise.Name = "Updated Enterprise"
		if err := enterpriseRepo.Update(txCtx, enterprise); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify updates
	updatedDevice, err := deviceRepo.GetByID(ctx, device.ID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	if updatedDevice.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", updatedDevice.Name)
	}

	updatedEnterprise, err := enterpriseRepo.GetByID(ctx, enterprise.ID)
	if err != nil {
		t.Fatalf("Failed to get enterprise: %v", err)
	}
	if updatedEnterprise.Name != "Updated Enterprise" {
		t.Errorf("Expected name 'Updated Enterprise', got '%s'", updatedEnterprise.Name)
	}
}

func TestTransactionUpdateRollback(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	ctx := context.Background()

	// Create enterprise and device
	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: "test-update-rollback-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     "macos",
		DeviceID:     "test-device-id",
		SerialNumber: "ROLLBACK-UPDATE-" + uuid.New().String()[:8],
		Name:         "Original Name",
		Status:       "active",
	}
	if err := deviceRepo.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	originalName := device.Name

	// Try to update in transaction that fails
	testError := errors.New("update failed")
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		device.Name = "Should Not Persist"
		if err := deviceRepo.Update(txCtx, device); err != nil {
			return err
		}
		return testError
	})

	if err != testError {
		t.Fatalf("Expected error %v, got %v", testError, err)
	}

	// Verify update was rolled back
	device, err = deviceRepo.GetByID(ctx, device.ID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	if device.Name != originalName {
		t.Errorf("Expected name '%s', got '%s' - update should have been rolled back", originalName, device.Name)
	}
}

func TestTransactionDeleteOperations(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	policyRepo, err := NewPolicyRepository(db.Writer, db.Reader)
	if err != nil {
		t.Fatalf("Failed to create policy repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create policy repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	ctx := context.Background()

	// Create test data
	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: "test-delete-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     "macos",
		DeviceID:     "test-device-id",
		SerialNumber: "DELETE123-" + uuid.New().String()[:8],
		Name:         "Test Device",
		Status:       "active",
	}
	if err := deviceRepo.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	policy := &models.Policy{
		EnterpriseID: enterprise.ID,
		Name:         "Test Policy",
		Platform:     "macos",
		PolicyType:   "security",
		PolicyConfig: models.JSONB{"test": true},
		IsActive:     true,
	}
	if err := policyRepo.Create(ctx, policy); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Delete in transaction
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := deviceRepo.Delete(txCtx, device.ID); err != nil {
			return err
		}
		if err := policyRepo.Delete(txCtx, policy.ID); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify deletions (soft delete)
	_, err = deviceRepo.GetByID(ctx, device.ID)
	if err == nil {
		t.Error("Expected device to be deleted")
	}

	_, err = policyRepo.GetByID(ctx, policy.ID)
	if err == nil {
		t.Error("Expected policy to be deleted")
	}
}

func TestTransactionDeleteRollback(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	ctx := context.Background()

	// Create test data
	enterprise := &models.Enterprise{
		Name: "Test Enterprise",
		Slug: "test-delete-rollback-" + uuid.New().String()[:8],
	}
	if err := enterpriseRepo.Create(ctx, enterprise); err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     "macos",
		DeviceID:     "test-device-id",
		SerialNumber: "DELETE-ROLLBACK-" + uuid.New().String()[:8],
		Name:         "Test Device",
		Status:       "active",
	}
	if err := deviceRepo.Create(ctx, device); err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	// Try to delete in transaction that fails
	testError := errors.New("delete failed")
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := deviceRepo.Delete(txCtx, device.ID); err != nil {
			return err
		}
		return testError
	})

	if err != testError {
		t.Fatalf("Expected error %v, got %v", testError, err)
	}

	// Verify device still exists
	_, err = deviceRepo.GetByID(ctx, device.ID)
	if err != nil {
		t.Errorf("Device should still exist after rollback: %v", err)
	}
}

func TestTransactionErrorPaths(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	deviceRepo, err := NewDeviceRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create device repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create device repository: %v", err)

	}

	ctx := context.Background()

	t.Run("update_nonexistent_device", func(t *testing.T) {
		err := transactor.WithTransaction(ctx, func(txCtx context.Context) error {
			device := &models.Device{
				Name: "Nonexistent",
			}
			device.ID = uuid.New()
			return deviceRepo.Update(txCtx, device)
		})

		if err == nil {
			t.Error("Expected error when updating nonexistent device")
		}
	})

	t.Run("delete_nonexistent_device", func(t *testing.T) {
		err := transactor.WithTransaction(ctx, func(txCtx context.Context) error {
			return deviceRepo.Delete(txCtx, uuid.New())
		})

		if err == nil {
			t.Error("Expected error when deleting nonexistent device")
		}
	})

	t.Run("get_nonexistent_device", func(t *testing.T) {
		err := transactor.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := deviceRepo.GetByID(txCtx, uuid.New())
			return err
		})

		if err == nil {
			t.Error("Expected error when getting nonexistent device")
		}
	})
}

func TestNewTransactorWithInvalidType(t *testing.T) {
	_, err := NewTransactor("invalid type")
	if err == nil {
		t.Error("Expected error when creating transactor with invalid type")
	}
	if err != nil && !strings.Contains(err.Error(), "unsupported database type") {
		t.Errorf("Expected 'unsupported database type' error, got: %v", err)
	}
}

func TestNewTransactorWithNil(t *testing.T) {
	_, err := NewTransactor(nil)
	if err == nil {
		t.Error("Expected error when creating transactor with nil")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot be nil") {
		t.Errorf("Expected 'cannot be nil' error, got: %v", err)
	}
}

func TestNewTransactorWithExecutor(t *testing.T) {
	db := testutil.ConnectDB(t)
	
	// Start a transaction to get an executor
	tx, err := db.Writer.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()
	
	// Create transactor with transaction (implements executor interface)
	transactor, err := NewTransactor(tx)
	if err != nil {
		t.Fatalf("Failed to create transactor with executor: %v", err)
	}
	
	if transactor == nil {
		t.Error("Expected non-nil transactor")
	}
}

func TestNewRepositoryWithInvalidType(t *testing.T) {
	t.Run("device_repository", func(t *testing.T) {
		_, err := NewDeviceRepository("invalid type", "invalid type")
		if err == nil {
			t.Error("Expected error when creating device repository with invalid type")
		}
		if err != nil && !strings.Contains(err.Error(), "unsupported writer type") {
			t.Errorf("Expected 'unsupported writer type' error, got: %v", err)
		}
	})

	t.Run("enterprise_repository", func(t *testing.T) {
		_, err := NewEnterpriseRepository("invalid type", "invalid type")
		if err == nil {
			t.Error("Expected error when creating enterprise repository with invalid type")
		}
		if err != nil && !strings.Contains(err.Error(), "unsupported writer type") {
			t.Errorf("Expected 'unsupported writer type' error, got: %v", err)
		}
	})

	t.Run("policy_repository", func(t *testing.T) {
		_, err := NewPolicyRepository("invalid type", "invalid type")
		if err == nil {
			t.Error("Expected error when creating policy repository with invalid type")
		}
		if err != nil && !strings.Contains(err.Error(), "unsupported writer type") {
			t.Errorf("Expected 'unsupported writer type' error, got: %v", err)
		}
	})
}

func TestNewRepositoryWithNil(t *testing.T) {
	t.Run("device_repository", func(t *testing.T) {
		_, err := NewDeviceRepository(nil, nil)
		if err == nil {
			t.Error("Expected error when creating device repository with nil")
		}
		if err != nil && !strings.Contains(err.Error(), "cannot be nil") {
			t.Errorf("Expected 'cannot be nil' error, got: %v", err)
		}
	})

	t.Run("enterprise_repository", func(t *testing.T) {
		_, err := NewEnterpriseRepository(nil, nil)
		if err == nil {
			t.Error("Expected error when creating enterprise repository with nil")
		}
		if err != nil && !strings.Contains(err.Error(), "cannot be nil") {
			t.Errorf("Expected 'cannot be nil' error, got: %v", err)
		}
	})

	t.Run("policy_repository", func(t *testing.T) {
		_, err := NewPolicyRepository(nil, nil)
		if err == nil {
			t.Error("Expected error when creating policy repository with nil")
		}
		if err != nil && !strings.Contains(err.Error(), "cannot be nil") {
			t.Errorf("Expected 'cannot be nil' error, got: %v", err)
		}
	})
}

func TestNewRepositoryWithExecutor(t *testing.T) {
	db := testutil.ConnectDB(t)
	
	// Start a transaction to get an executor
	tx, err := db.Writer.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()
	
	t.Run("device_repository", func(t *testing.T) {
		repo, err := NewDeviceRepository(tx, tx)
		if err != nil {
			t.Fatalf("Failed to create device repository with executor: %v", err)
		}
		if repo == nil {
			t.Error("Expected non-nil repository")
		}
	})
	
	t.Run("enterprise_repository", func(t *testing.T) {
		repo, err := NewEnterpriseRepository(tx, tx)
		if err != nil {
			t.Fatalf("Failed to create enterprise repository with executor: %v", err)
		}
		if repo == nil {
			t.Error("Expected non-nil repository")
		}
	})
	
	t.Run("policy_repository", func(t *testing.T) {
		repo, err := NewPolicyRepository(tx, tx)
		if err != nil {
			t.Fatalf("Failed to create policy repository with executor: %v", err)
		}
		if repo == nil {
			t.Error("Expected non-nil repository")
		}
	})
}

func TestGetExecutorWithInvalidType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when getting executor with invalid type")
		}
	}()

	ctx := context.Background()
	getExecutor(ctx, "invalid type")
}

func TestTransactionWithCancelledContext(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}


	if err != nil {


		t.Fatalf("Failed to create transactor: %v", err)


	}
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	if err != nil {

		t.Fatalf("Failed to create enterprise repository: %v", err)

	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to start transaction with cancelled context
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		enterprise := &models.Enterprise{
			Name: "Test Enterprise",
			Slug: "test-cancelled-" + uuid.New().String()[:8],
		}
		return enterpriseRepo.Create(txCtx, enterprise)
	})

	// Should fail due to cancelled context
	if err == nil {
		t.Error("Expected error when using cancelled context")
	}
}

func TestTransactionWithTimeout(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to timeout
	time.Sleep(10 * time.Millisecond)

	// Try to start transaction - should fail due to timeout
	err = transactor.WithTransaction(ctx, func(txCtx context.Context) error {
		return nil
	})

	if err == nil {
		t.Error("Expected error due to context timeout")
	}
}

func TestTransactionIsolationLevels(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}
	
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		isolation IsolationLevel
	}{
		{"default isolation", IsolationDefault},
		{"read committed", IsolationReadCommitted},
		{"serializable", IsolationSerializable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entID uuid.UUID
			err := transactor.WithTransactionIsolation(ctx, tt.isolation, func(txCtx context.Context) error {
				enterprise := &models.Enterprise{
					Name: "Test Enterprise " + tt.name,
					Slug: "test-" + uuid.New().String()[:8],
				}
				if err := enterpriseRepo.Create(txCtx, enterprise); err != nil {
					return err
				}
				entID = enterprise.ID
				return nil
			})

			if err != nil {
				t.Errorf("Transaction with %s failed: %v", tt.name, err)
			}
			if entID != uuid.Nil {
				t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", entID) })
			}
		})
	}
}

func TestSerializableTransactionRetry(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}
	
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	ctx := context.Background()

	// Create an enterprise
	enterprise := &models.Enterprise{
		Name: "Serializable Test",
		Slug: "serializable-" + uuid.New().String()[:8],
	}
	
	err = enterpriseRepo.Create(ctx, enterprise)
	if err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	// Test serializable transaction
	err = transactor.WithTransactionIsolation(ctx, IsolationSerializable, func(txCtx context.Context) error {
		// Read enterprise
		_, err := enterpriseRepo.GetByID(txCtx, enterprise.ID)
		return err
	})

	if err != nil {
		t.Errorf("Serializable transaction failed: %v", err)
	}
}

func TestIsSerializationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "serialization error",
			err:      errors.New("could not serialize access"),
			expected: true,
		},
		{
			name:     "deadlock error",
			err:      errors.New("deadlock detected"),
			expected: true,
		},
		{
			name:     "serialization failure",
			err:      errors.New("SERIALIZATION FAILURE"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "connection error",
			err:      errors.New("connection refused"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSerializationError(tt.err)
			if result != tt.expected {
				t.Errorf("isSerializationError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestToSQLIsolation(t *testing.T) {
	tests := []struct {
		name     string
		level    IsolationLevel
		expected sql.IsolationLevel
	}{
		{
			name:     "default",
			level:    IsolationDefault,
			expected: sql.LevelDefault,
		},
		{
			name:     "read committed",
			level:    IsolationReadCommitted,
			expected: sql.LevelReadCommitted,
		},
		{
			name:     "serializable",
			level:    IsolationSerializable,
			expected: sql.LevelSerializable,
		},
		{
			name:     "unknown level",
			level:    IsolationLevel("UNKNOWN"),
			expected: sql.LevelDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toSQLIsolation(tt.level)
			if result != tt.expected {
				t.Errorf("toSQLIsolation(%v) = %v, want %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestTransactionIsolationWithError(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}

	ctx := context.Background()

	// Test that error is properly returned
	testError := errors.New("intentional test error")
	err = transactor.WithTransactionIsolation(ctx, IsolationSerializable, func(txCtx context.Context) error {
		return testError
	})

	if err == nil {
		t.Error("Expected error to be returned")
	}
	if err != testError {
		t.Errorf("Expected error %v, got %v", testError, err)
	}
}

func TestNestedTransactionWithIsolation(t *testing.T) {
	db := testutil.ConnectDB(t)

	transactor, err := NewTransactor(db.Writer)
	if err != nil {
		t.Fatalf("Failed to create transactor: %v", err)
	}
	
	enterpriseRepo, err := NewEnterpriseRepository(db.Writer, db.Writer)
	if err != nil {
		t.Fatalf("Failed to create enterprise repository: %v", err)
	}

	ctx := context.Background()

	// Test nested transaction (should reuse outer transaction)
	var outerID, innerID uuid.UUID
	err = transactor.WithTransactionIsolation(ctx, IsolationSerializable, func(txCtx context.Context) error {
		enterprise := &models.Enterprise{
			Name: "Outer Transaction",
			Slug: "outer-" + uuid.New().String()[:8],
		}
		
		if err := enterpriseRepo.Create(txCtx, enterprise); err != nil {
			return err
		}
		outerID = enterprise.ID

		// Nested transaction should reuse the same transaction
		return transactor.WithTransactionIsolation(txCtx, IsolationSerializable, func(nestedCtx context.Context) error {
			enterprise2 := &models.Enterprise{
				Name: "Inner Transaction",
				Slug: "inner-" + uuid.New().String()[:8],
			}
			if err := enterpriseRepo.Create(nestedCtx, enterprise2); err != nil {
				return err
			}
			innerID = enterprise2.ID
			return nil
		})
	})

	if err != nil {
		t.Errorf("Nested transaction failed: %v", err)
	}
	t.Cleanup(func() {
		db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", outerID)
		db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", innerID)
	})
}

