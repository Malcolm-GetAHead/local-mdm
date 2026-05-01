package scep

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"
)

// Challenge represents a SCEP enrollment challenge
type Challenge struct {
	Password  string
	ExpiresAt time.Time
	DeviceID  string
	Used      bool
}

// ChallengeStore is the interface for SCEP challenge operations.
type ChallengeStore interface {
	GenerateChallenge(ctx context.Context, deviceID string, ttl time.Duration) (string, error)
	ValidateChallenge(ctx context.Context, password string) (string, bool)
	CleanupExpired(ctx context.Context)
}

// ChallengeManager manages SCEP enrollment challenges in PostgreSQL.
type ChallengeManager struct {
	db *sql.DB
}

// NewChallengeManager creates a new PostgreSQL-backed challenge manager.
func NewChallengeManager(db *sql.DB) *ChallengeManager {
	return &ChallengeManager{db: db}
}

// GenerateChallenge creates a new enrollment challenge stored in PostgreSQL.
func (cm *ChallengeManager) GenerateChallenge(ctx context.Context, deviceID string, ttl time.Duration) (string, error) {
	password, err := generateSecurePassword(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}

	_, err = cm.db.ExecContext(ctx,
		`INSERT INTO scep_challenges (password, device_id, expires_at) VALUES ($1, $2, $3)`,
		password, deviceID, time.Now().Add(ttl),
	)
	if err != nil {
		return "", fmt.Errorf("failed to store challenge: %w", err)
	}

	return password, nil
}

// ValidateChallenge validates and marks a challenge as used atomically.
func (cm *ChallengeManager) ValidateChallenge(ctx context.Context, password string) (string, bool) {
	var deviceID string
	err := cm.db.QueryRowContext(ctx,
		`UPDATE scep_challenges SET used = true
		 WHERE password = $1 AND NOT used AND expires_at > NOW()
		 RETURNING device_id`, password,
	).Scan(&deviceID)
	if err != nil {
		return "", false
	}
	return deviceID, true
}

// CleanupExpired removes expired challenges from the database.
func (cm *ChallengeManager) CleanupExpired(ctx context.Context) {
	cm.db.ExecContext(ctx,
		`DELETE FROM scep_challenges WHERE expires_at < NOW()`)
}

// generateSecurePassword generates a cryptographically secure random password
func generateSecurePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
