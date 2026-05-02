package service

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuditLogRepo struct {
	logs []*models.AuditLog
}

func (m *mockAuditLogRepo) List(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.AuditLog, int, error) {
	return m.logs, len(m.logs), nil
}
func (m *mockAuditLogRepo) Search(_ context.Context, _ uuid.UUID, _, _, _ string, _, _ int) ([]*models.AuditLog, int, error) {
	return m.logs, len(m.logs), nil
}

func TestAuditLogService_ListAndSearch(t *testing.T) {
	eid := uuid.New()
	logs := []*models.AuditLog{
		{ID: uuid.New(), EnterpriseID: &eid, Action: "device.create", CreatedAt: time.Now()},
	}
	repo := &mockAuditLogRepo{logs: logs}
	svc := NewAuditLogService(repo, slog.Default())
	ctx := context.Background()

	// List
	result, total, err := svc.List(ctx, eid, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)

	// Search
	result, total, err = svc.Search(ctx, eid, "device.create", "", "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
}
