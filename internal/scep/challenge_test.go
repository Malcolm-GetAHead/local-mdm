package scep

import (
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChallengeManager_GenerateChallenge(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	t.Cleanup(func() { db.Exec("DELETE FROM scep_challenges") })
	db.Exec("DELETE FROM scep_challenges")
	cm := NewChallengeManager(db)

	t.Run("generates unique challenges", func(t *testing.T) {
		c1, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)
		assert.NotEmpty(t, c1)

		c2, err := cm.GenerateChallenge("device2", 5*time.Minute)
		require.NoError(t, err)
		assert.NotEqual(t, c1, c2)
	})

	t.Run("generates challenge with correct length", func(t *testing.T) {
		c, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)
		assert.Len(t, c, 32)
	})
}

func TestChallengeManager_ValidateChallenge(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	t.Cleanup(func() { db.Exec("DELETE FROM scep_challenges") })
	db.Exec("DELETE FROM scep_challenges")
	cm := NewChallengeManager(db)

	t.Run("validates unused challenge", func(t *testing.T) {
		c, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)

		deviceID, valid := cm.ValidateChallenge(c)
		assert.True(t, valid)
		assert.Equal(t, "device1", deviceID)
	})

	t.Run("rejects used challenge", func(t *testing.T) {
		c, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)

		_, valid := cm.ValidateChallenge(c)
		assert.True(t, valid)

		_, valid = cm.ValidateChallenge(c)
		assert.False(t, valid)
	})

	t.Run("rejects expired challenge", func(t *testing.T) {
		// Insert directly with past expiry to avoid sleep
		password, err := generateSecurePassword(32)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO scep_challenges (password, device_id, expires_at) VALUES ($1, $2, $3)`,
			password, "device1", time.Now().Add(-1*time.Second))
		require.NoError(t, err)

		_, valid := cm.ValidateChallenge(password)
		assert.False(t, valid)
	})

	t.Run("rejects non-existent challenge", func(t *testing.T) {
		_, valid := cm.ValidateChallenge("invalid-challenge")
		assert.False(t, valid)
	})
}

func TestChallengeManager_CleanupExpired(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	t.Cleanup(func() { db.Exec("DELETE FROM scep_challenges") })
	db.Exec("DELETE FROM scep_challenges")
	cm := NewChallengeManager(db)

	// Insert expired challenge directly
	password, err := generateSecurePassword(32)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO scep_challenges (password, device_id, expires_at) VALUES ($1, $2, $3)`,
		password, "device1", time.Now().Add(-1*time.Second))
	require.NoError(t, err)

	// Create valid challenge
	validPw, err := cm.GenerateChallenge("device2", 5*time.Minute)
	require.NoError(t, err)

	cm.CleanupExpired()

	// Expired should be gone
	_, valid := cm.ValidateChallenge(password)
	assert.False(t, valid)

	// Valid should remain
	_, valid = cm.ValidateChallenge(validPw)
	assert.True(t, valid)
}

func TestGenerateSecurePassword(t *testing.T) {
	t.Run("generates password of correct length", func(t *testing.T) {
		p, err := generateSecurePassword(32)
		require.NoError(t, err)
		assert.Len(t, p, 32)
	})

	t.Run("generates unique passwords", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			p, err := generateSecurePassword(32)
			require.NoError(t, err)
			assert.False(t, seen[p])
			seen[p] = true
		}
	})
}
