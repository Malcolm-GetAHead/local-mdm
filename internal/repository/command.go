package repository

import (
	"context"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CommandRepository provides data access for device management commands.
type CommandRepository interface {
	Create(ctx context.Context, cmd *models.DeviceCommand) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.DeviceCommand, error)
	ListPending(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error)
	ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*models.DeviceCommand, int, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkCompleted(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error
}

type commandRepository struct {
	writer executor
	reader executor
}

// NewCommandRepository creates a new command repository instance.
func NewCommandRepository(writer, reader interface{}) (CommandRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &commandRepository{writer: w, reader: r}, nil
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

	exec := getExecutor(ctx, r.writer)
	return exec.QueryRowContext(ctx, query,
		cmd.ID, cmd.DeviceID, cmd.CommandType, cmd.CommandData, cmd.Status,
	).Scan(&cmd.CreatedAt, &cmd.UpdatedAt)
}

func (r *commandRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DeviceCommand, error) {
	query := `
		SELECT id, device_id, command_type, command_data, status, sent_at, completed_at, COALESCE(error_message, ''), created_at, updated_at
		FROM device_commands WHERE id = $1`

	exec := getReadExecutor(ctx, r.reader)
	cmd := &models.DeviceCommand{}
	err := exec.QueryRowContext(ctx, query, id).Scan(
		&cmd.ID, &cmd.DeviceID, &cmd.CommandType, &cmd.CommandData,
		&cmd.Status, &cmd.SentAt, &cmd.CompletedAt, &cmd.ErrorMessage,
		&cmd.CreatedAt, &cmd.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("command not found: %w", apperrors.ErrNotFound)
	}
	return cmd, nil
}

func (r *commandRepository) ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*models.DeviceCommand, int, error) {
	vLimit, vOffset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	countQuery := `SELECT COUNT(*) FROM device_commands WHERE device_id = $1`
	exec := getReadExecutor(ctx, r.reader)
	var total int
	if err := exec.QueryRowContext(ctx, countQuery, deviceID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count commands: %w", err)
	}

	query := `
		SELECT id, device_id, command_type, command_data, status, sent_at, completed_at, COALESCE(error_message, ''), created_at, updated_at
		FROM device_commands WHERE device_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := exec.QueryContext(ctx, query, deviceID, vLimit, vOffset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list commands: %w", err)
	}
	defer rows.Close()

	var cmds []*models.DeviceCommand
	for rows.Next() {
		cmd := &models.DeviceCommand{}
		if err := rows.Scan(
			&cmd.ID, &cmd.DeviceID, &cmd.CommandType, &cmd.CommandData,
			&cmd.Status, &cmd.SentAt, &cmd.CompletedAt, &cmd.ErrorMessage,
			&cmd.CreatedAt, &cmd.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan command: %w", err)
		}
		cmds = append(cmds, cmd)
	}
	return cmds, total, rows.Err()
}

func (r *commandRepository) ListPending(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error) {
	query := `
		SELECT id, device_id, command_type, command_data, status, created_at, updated_at
		FROM device_commands
		WHERE device_id = $1 AND status = 'pending'
		ORDER BY created_at ASC`

	exec := getReadExecutor(ctx, r.reader)
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

	exec := getExecutor(ctx, r.writer)
	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update command status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("command not found: %w", apperrors.ErrNotFound)
	}
	return nil
}
