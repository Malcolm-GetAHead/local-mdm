package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

// Logger writes audit events to the database
type Logger struct {
	db *sql.DB
}

// Event represents an audit log entry
type Event struct {
	EnterpriseID uuid.UUID
	UserID       uuid.UUID
	Action       string
	ResourceType string
	ResourceID   uuid.UUID
	Details      map[string]interface{}
	IPAddress    net.IP
	UserAgent    string
}

// NewLogger creates a new audit logger
func NewLogger(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Log writes an audit event to the database
func (l *Logger) Log(ctx context.Context, event Event) error {
	if err := validateEvent(event); err != nil {
		return fmt.Errorf("invalid audit event: %w", err)
	}

	details, err := json.Marshal(event.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal details: %w", err)
	}

	query := `
		INSERT INTO audit_logs (
			enterprise_id, user_id, action, resource_type, resource_id,
			details, ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = l.db.ExecContext(
		ctx,
		query,
		nullUUID(event.EnterpriseID),
		nullUUID(event.UserID),
		event.Action,
		event.ResourceType,
		nullUUID(event.ResourceID),
		details,
		ipToString(event.IPAddress),
		nullString(event.UserAgent),
		time.Now().UTC(),
	)

	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// validateEvent ensures required fields are present
func validateEvent(event Event) error {
	if event.Action == "" {
		return fmt.Errorf("action is required")
	}
	if event.ResourceType == "" {
		return fmt.Errorf("resource_type is required")
	}
	if len(event.Action) > 100 {
		return fmt.Errorf("action exceeds 100 characters")
	}
	if len(event.ResourceType) > 50 {
		return fmt.Errorf("resource_type exceeds 50 characters")
	}
	return nil
}

// Helper functions for nullable fields
func nullUUID(id uuid.UUID) interface{} {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func ipToString(ip net.IP) interface{} {
	if ip == nil {
		return nil
	}
	return ip.String()
}
