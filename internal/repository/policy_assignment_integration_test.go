package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyAssignmentRepository(t *testing.T) {
	database := testutil.ConnectDB(t)
	testutil.EnsureTestEnterprise(t, database.Writer)
	t.Cleanup(func() { testutil.CleanupTestData(t, database.Writer) })
	enterpriseID := testutil.TestEnterpriseID

	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	device := &models.Device{
		EnterpriseID: enterpriseID,
		Platform:     models.PlatformWindows,
		DeviceID:     "pa-dev-" + uuid.New().String()[:8],
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(context.Background(), device))
	t.Cleanup(func() { deviceRepo.Delete(context.Background(), device.ID) })

	groupRepo, err := repository.NewGroupRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	group := &models.DeviceGroup{
		EnterpriseID: enterpriseID,
		Name:         "PA Test Group",
	}
	require.NoError(t, groupRepo.Create(context.Background(), group))
	t.Cleanup(func() { groupRepo.Delete(context.Background(), group.ID) })

	policyRepo, err := repository.NewPolicyRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	policy1 := &models.Policy{
		EnterpriseID: enterpriseID,
		Name:         "PA Policy 1",
		Platform:     models.PlatformWindows,
		PolicyType:   "wifi",
		PolicyConfig: models.JSONB{"ssid": "Corp"},
		IsActive:     true,
	}
	policy2 := &models.Policy{
		EnterpriseID: enterpriseID,
		Name:         "PA Policy 2",
		Platform:     models.PlatformWindows,
		PolicyType:   "security",
		PolicyConfig: models.JSONB{"rule": "lock"},
		IsActive:     true,
	}
	require.NoError(t, policyRepo.Create(context.Background(), policy1))
	require.NoError(t, policyRepo.Create(context.Background(), policy2))
	t.Cleanup(func() {
		policyRepo.Delete(context.Background(), policy1.ID)
		policyRepo.Delete(context.Background(), policy2.ID)
	})

	repo, err := repository.NewPolicyAssignmentRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("create device assignment", func(t *testing.T) {
		a := &models.PolicyAssignment{
			PolicyID:   policy1.ID,
			TargetType: models.TargetTypeDevice,
			TargetID:   device.ID,
			Priority:   1,
		}
		err := repo.Create(ctx, a)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, a.ID)
		assert.False(t, a.CreatedAt.IsZero())
	})

	t.Run("create group assignment", func(t *testing.T) {
		a := &models.PolicyAssignment{
			PolicyID:   policy1.ID,
			TargetType: models.TargetTypeGroup,
			TargetID:   group.ID,
			Priority:   10,
		}
		err := repo.Create(ctx, a)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, a.ID)
	})

	t.Run("create enterprise assignment", func(t *testing.T) {
		a := &models.PolicyAssignment{
			PolicyID:   policy2.ID,
			TargetType: models.TargetTypeEnterprise,
			TargetID:   enterpriseID,
			Priority:   100,
		}
		err := repo.Create(ctx, a)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, a.ID)
	})

	t.Run("list by target", func(t *testing.T) {
		assignments, err := repo.ListByTarget(ctx, models.TargetTypeDevice, device.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(assignments), 1)
		assert.Equal(t, policy1.ID, assignments[0].PolicyID)
	})

	t.Run("list by policy", func(t *testing.T) {
		assignments, err := repo.ListByPolicy(ctx, policy1.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(assignments), 2) // device + group
	})

	t.Run("get effective policies resolves priority", func(t *testing.T) {
		// Device has: direct assignment (priority 1), group assignment (priority 10),
		// enterprise assignment for policy2 (priority 100)
		groupIDs := []uuid.UUID{group.ID}
		effective, err := repo.GetEffectivePolicies(ctx, device.ID, groupIDs, enterpriseID)
		require.NoError(t, err)
		// Should have 2 policies (policy1 and policy2)
		assert.GreaterOrEqual(t, len(effective), 2)

		// For policy1, the device-level assignment (priority 1) should win
		// over the group-level (priority 10) via DISTINCT ON
		for _, e := range effective {
			if e.PolicyID == policy1.ID {
				assert.Equal(t, 1, e.Priority, "device-level priority should win")
			}
		}
	})

	t.Run("get effective policies with no groups", func(t *testing.T) {
		effective, err := repo.GetEffectivePolicies(ctx, device.ID, nil, enterpriseID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(effective), 1)
	})

	t.Run("delete assignment", func(t *testing.T) {
		a := &models.PolicyAssignment{
			PolicyID:   policy2.ID,
			TargetType: models.TargetTypeDevice,
			TargetID:   device.ID,
			Priority:   5,
		}
		require.NoError(t, repo.Create(ctx, a))

		err := repo.Delete(ctx, a.ID)
		require.NoError(t, err)
	})

	t.Run("delete nonexistent returns error", func(t *testing.T) {
		err := repo.Delete(ctx, uuid.New())
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("duplicate assignment is ignored", func(t *testing.T) {
		a := &models.PolicyAssignment{
			PolicyID:   policy1.ID,
			TargetType: models.TargetTypeDevice,
			TargetID:   device.ID,
			Priority:   1,
		}
		// ON CONFLICT DO NOTHING — RETURNING won't return a row, so Scan fails
		err := repo.Create(ctx, a)
		assert.Error(t, err) // sql.ErrNoRows from Scan
	})
}

func TestPolicyAssignmentRepository_Constructor(t *testing.T) {
	t.Run("nil returns error", func(t *testing.T) {
		_, err := repository.NewPolicyAssignmentRepository(nil, nil)
		assert.Error(t, err)
	})
	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := repository.NewPolicyAssignmentRepository("bad", "bad")
		assert.Error(t, err)
	})
}
