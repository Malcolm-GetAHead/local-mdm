package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CommandRepository provides data access for device management commands.
type CommandRepository interface {
	Create(ctx context.Context, cmd *models.DeviceCommand) error
	ListPending(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkCompleted(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error
}

type commandRepository struct {
	db executor
}

// NewCommandRepository creates a new command repository instance.
func NewCommandRepository(db interface{}) (CommandRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	switch v := db.(type) {
	case *sql.DB:
		return &commandRepository{db: v}, nil
	case executor:
		return &commandRepository{db: v}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %T", db)
	}
}

func (r *commandRepository) Create(ctx context.Context, cmd *models.DeviceCommand) error {
	if cmd.ID == uuid.Nil {
		cmd.ID = uuid.New()
	}
	if cmd.Status == "" {
		cmd.Status = models.CommandStatusPending
	}

	query := `
		INSERT INTO device_commands (id, device_id, command_type, command_data, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`

	exec := getExecutor(ctx, r.db)
	return exec.QueryRowContext(ctx, query,
		cmd.ID, cmd.DeviceID, cmd.CommandType, cmd.CommandData, cmd.Status,
	).Scan(&cmd.CreatedAt, &cmd.UpdatedAt)
}

func (r *commandRepository) ListPending(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error) {
	query := `
		SELECT id, device_id, command_type, command_data, status, created_at, updated_at
		FROM device_commands
		WHERE device_id = $1 AND status = 'pending'
		ORDER BY created_at ASC`

	exec := getExecutor(ctx, r.db)
	rows, err := exec.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending commands: %w", err)
	}
	defer rows.Close()

	var cmds []*models.DeviceCommand
	for rows.Next() {
		cmd := &models.DeviceCommand{}
		if err := rows.Scan(
			&cmd.ID, &cmd.DeviceID, &cmd.CommandType, &cmd.CommandData,
			&cmd.Status, &cmd.CreatedAt, &cmd.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}
		cmds = append(cmds, cmd)
	}
	return cmds, rows.Err()
}

func (r *commandRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	return r.updateStatus(ctx, id, models.CommandStatusSent, "")
}

func (r *commandRepository) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	return r.updateStatus(ctx, id, models.CommandStatusCompleted, "")
}

func (r *commandRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	return r.updateStatus(ctx, id, models.CommandStatusFailed, errMsg)
}

func (r *commandRepository) updateStatus(ctx context.Context, id uuid.UUID, status, errMsg string) error {
	now := time.Now()
	var query string
	var args []interface{}

	switch status {
	case models.CommandStatusSent:
		query = `UPDATE device_commands SET status = $1, sent_at = $2 WHERE id = $3`
		args = []interface{}{status, now, id}
	case models.CommandStatusCompleted:
		query = `UPDATE device_commands SET status = $1, completed_at = $2 WHERE id = $3`
		args = []interface{}{status, now, id}
	case models.CommandStatusFailed:
		query = `UPDATE device_commands SET status = $1, completed_at = $2, error_message = $3 WHERE id = $4`
		args = []interface{}{status, now, errMsg, id}
	default:
		return fmt.Errorf("unsupported status: %s", status)
	}

	exec := getExecutor(ctx, r.db)
	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update command status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("command not found")
	}
	return nil
}
