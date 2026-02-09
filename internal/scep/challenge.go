package scep

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// Challenge represents a SCEP enrollment challenge
type Challenge struct {
	Password  string
	ExpiresAt time.Time
	DeviceID  string
	Used      bool
}

// ChallengeManager manages SCEP enrollment challenges
type ChallengeManager struct {
	challenges map[string]*Challenge
	mu         sync.RWMutex
}

// NewChallengeManager creates a new challenge manager
func NewChallengeManager() *ChallengeManager {
	return &ChallengeManager{
		challenges: make(map[string]*Challenge),
	}
}

// GenerateChallenge creates a new enrollment challenge
func (cm *ChallengeManager) GenerateChallenge(deviceID string, ttl time.Duration) (string, error) {
	password, err := generateSecurePassword(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.challenges[password] = &Challenge{
		Password:  password,
		ExpiresAt: time.Now().Add(ttl),
		DeviceID:  deviceID,
		Used:      false,
	}

	return password, nil
}

// ValidateChallenge validates and marks a challenge as used
func (cm *ChallengeManager) ValidateChallenge(password string) (string, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	challenge, exists := cm.challenges[password]
	if !exists {
		return "", false
	}

	if challenge.Used {
		return "", false
	}

	if time.Now().After(challenge.ExpiresAt) {
		delete(cm.challenges, password)
		return "", false
	}

	challenge.Used = true
	return challenge.DeviceID, true
}

// CleanupExpired removes expired challenges
func (cm *ChallengeManager) CleanupExpired() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for password, challenge := range cm.challenges {
		if now.After(challenge.ExpiresAt) {
			delete(cm.challenges, password)
		}
	}
}

// generateSecurePassword generates a cryptographically secure random password
func generateSecurePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
