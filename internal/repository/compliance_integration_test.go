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

func TestComplianceRepository(t *testing.T) {
	database := testutil.ConnectDB(t)
	testutil.EnsureTestEnterprise(t, database.Writer)
	t.Cleanup(func() { testutil.CleanupTestData(t, database.Writer) })
	enterpriseID := testutil.TestEnterpriseID

	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	device := &models.Device{
		EnterpriseID: enterpriseID,
		Platform:     models.PlatformWindows,
		DeviceID:     "comp-dev-" + uuid.New().String()[:8],
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(context.Background(), device))
	t.Cleanup(func() { deviceRepo.Delete(context.Background(), device.ID) })

	policyRepo, err := repository.NewPolicyRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	policy := &models.Policy{
		EnterpriseID: enterpriseID,
		Name:         "comp-policy-" + uuid.New().String()[:8],
		Platform:     models.PlatformWindows,
		PolicyType:   "security",
		PolicyConfig: models.JSONB{"rule": "test"},
		IsActive:     true,
	}
	require.NoError(t, policyRepo.Create(context.Background(), policy))
	t.Cleanup(func() { policyRepo.Delete(context.Background(), policy.ID) })

	repo, err := repository.NewComplianceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("upsert creates new result", func(t *testing.T) {
		result := &models.ComplianceResult{
			DeviceID: device.ID,
			PolicyID: policy.ID,
			Status:   models.ComplianceStatusCompliant,
			Details:  models.JSONB{"check": "passed"},
		}
		err := repo.Upsert(ctx, result)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.ID)
		assert.False(t, result.EvaluatedAt.IsZero())
	})

	t.Run("upsert updates existing result", func(t *testing.T) {
		result := &models.ComplianceResult{
			DeviceID: device.ID,
			PolicyID: policy.ID,
			Status:   models.ComplianceStatusNonCompliant,
			Details:  models.JSONB{"check": "failed"},
		}
		err := repo.Upsert(ctx, result)
		require.NoError(t, err)

		results, err := repo.GetByDevice(ctx, device.ID)
		require.NoError(t, err)
		// Should have exactly one result (upsert replaced the first)
		found := false
		for _, r := range results {
			if r.PolicyID == policy.ID {
				assert.Equal(t, models.ComplianceStatusNonCompliant, r.Status)
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("get by device returns results", func(t *testing.T) {
		results, err := repo.GetByDevice(ctx, device.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1)
		assert.Equal(t, device.ID, results[0].DeviceID)
	})

	t.Run("get by device with no results returns empty", func(t *testing.T) {
		results, err := repo.GetByDevice(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("get summary returns enterprise counts", func(t *testing.T) {
		summary, err := repo.GetSummary(ctx, enterpriseID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, summary.Total, 1)
		assert.GreaterOrEqual(t, summary.NonCompliant, 1) // from upsert update above
	})

	t.Run("get summary for empty enterprise", func(t *testing.T) {
		summary, err := repo.GetSummary(ctx, uuid.New())
		require.NoError(t, err)
		assert.Equal(t, 0, summary.Total)
	})

	t.Run("delete by device removes all results", func(t *testing.T) {
		err := repo.DeleteByDevice(ctx, device.ID)
		require.NoError(t, err)

		results, err := repo.GetByDevice(ctx, device.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("delete by device with no results is no-op", func(t *testing.T) {
		err := repo.DeleteByDevice(ctx, uuid.New())
		require.NoError(t, err)
	})
}

func TestComplianceRepository_Constructor(t *testing.T) {
	t.Run("nil returns error", func(t *testing.T) {
		_, err := repository.NewComplianceRepository(nil, nil)
		assert.Error(t, err)
	})
	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewComplianceRepository("bad", "bad")
		assert.Error(t, err)
	})
}
