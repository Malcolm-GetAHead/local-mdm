package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// setupBenchDB creates a test database connection for benchmarks
func setupBenchDB(b *testing.B) *db.DB {
	b.Helper()
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}
	cfg := config.DatabaseConfig{
		Host:            host,
		Port:            5432,
		User:            "postgres",
		Password:        password,
		Database:        "localmdm",
		SSLMode:         "disable",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}

	database, err := db.New(cfg)
	if err != nil {
		b.Fatalf("Failed to connect to database: %v", err)
	}

	return database
}

// createBenchEnterprise creates a test enterprise for benchmarks
func createBenchEnterprise(b *testing.B, database *db.DB) uuid.UUID {
	b.Helper()
	repo, _ := NewEnterpriseRepository(database, database)
	enterprise := &models.Enterprise{
		Name: "Bench Enterprise " + uuid.New().String()[:8],
		Slug: "bench-" + uuid.New().String()[:8],
	}
	_ = repo.Create(context.Background(), enterprise)
	return enterprise.ID
}

// createBenchDevice creates a test device for benchmarks
func createBenchDevice(b *testing.B, database *db.DB, enterpriseID uuid.UUID) *models.Device {
	b.Helper()
	repo, _ := NewDeviceRepository(database, database)
	device := &models.Device{
		EnterpriseID: enterpriseID,
		SerialNumber: uuid.New().String(),
		Platform:     "windows",
		Status:       "active",
	}
	_ = repo.Create(context.Background(), device)
	return device
}

// BenchmarkDeviceRepository_Create measures device creation performance
func BenchmarkDeviceRepository_Create(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		device := &models.Device{
			EnterpriseID: enterpriseID,
			SerialNumber: uuid.New().String(),
			Platform:     "windows",
			Status:       "active",
		}
		_ = repo.Create(ctx, device)
	}
}

// BenchmarkDeviceRepository_GetByID measures device retrieval performance
func BenchmarkDeviceRepository_GetByID(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)
	device := createBenchDevice(b, database, enterpriseID)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetByID(ctx, device.ID)
	}
}

// BenchmarkDeviceRepository_List measures pagination performance
func BenchmarkDeviceRepository_List(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)

	// Create 100 devices for realistic pagination
	for i := 0; i < 100; i++ {
		createBenchDevice(b, database, enterpriseID)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = repo.List(ctx, enterpriseID, 50, 0)
	}
}

// BenchmarkDeviceRepository_List_SmallPage measures small page performance
func BenchmarkDeviceRepository_List_SmallPage(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)

	for i := 0; i < 100; i++ {
		createBenchDevice(b, database, enterpriseID)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = repo.List(ctx, enterpriseID, 10, 0)
	}
}

// BenchmarkDeviceRepository_List_LargePage measures large page performance
func BenchmarkDeviceRepository_List_LargePage(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)

	for i := 0; i < 1000; i++ {
		createBenchDevice(b, database, enterpriseID)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = repo.List(ctx, enterpriseID, 100, 0)
	}
}

// BenchmarkDeviceRepository_Update measures device update performance
func BenchmarkDeviceRepository_Update(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)
	device := createBenchDevice(b, database, enterpriseID)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		device.Status = "inactive"
		_ = repo.Update(ctx, device)
		device.Status = "active"
	}
}

// BenchmarkEnterpriseRepository_Create measures enterprise creation performance
func BenchmarkEnterpriseRepository_Create(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewEnterpriseRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enterprise := &models.Enterprise{
			Name: uuid.New().String(),
			Slug: uuid.New().String(),
		}
		_ = repo.Create(ctx, enterprise)
	}
}

// BenchmarkEnterpriseRepository_GetByID measures enterprise retrieval performance
func BenchmarkEnterpriseRepository_GetByID(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewEnterpriseRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetByID(ctx, enterpriseID)
	}
}

// BenchmarkPolicyRepository_Create measures policy creation performance
func BenchmarkPolicyRepository_Create(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewPolicyRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy := &models.Policy{
			EnterpriseID: enterpriseID,
			Name:         uuid.New().String(),
			Platform:     "windows",
			PolicyConfig: models.JSONB{"setting": "value"},
		}
		_ = repo.Create(ctx, policy)
	}
}

// BenchmarkPolicyRepository_List measures policy pagination performance
func BenchmarkPolicyRepository_List(b *testing.B) {
	database := setupBenchDB(b)
	defer database.Close()

	repo, err := NewPolicyRepository(database.Writer, database.Reader)
	if err != nil {
		b.Fatal(err)
	}

	enterpriseID := createBenchEnterprise(b, database)

	// Create 50 policies
	for i := 0; i < 50; i++ {
		policy := &models.Policy{
			EnterpriseID: enterpriseID,
			Name:         uuid.New().String(),
			Platform:     "windows",
			PolicyConfig: models.JSONB{"setting": "value"},
		}
		_ = repo.Create(context.Background(), policy)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = repo.List(ctx, enterpriseID, 25, 0)
	}
}
