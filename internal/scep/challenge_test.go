package scep

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChallengeManager_GenerateChallenge(t *testing.T) {
	cm := NewChallengeManager()

	t.Run("generates unique challenges", func(t *testing.T) {
		challenge1, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)
		assert.NotEmpty(t, challenge1)

		challenge2, err := cm.GenerateChallenge("device2", 5*time.Minute)
		require.NoError(t, err)
		assert.NotEmpty(t, challenge2)

		assert.NotEqual(t, challenge1, challenge2, "challenges should be unique")
	})

	t.Run("generates challenge with correct length", func(t *testing.T) {
		challenge, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)
		assert.Len(t, challenge, 32)
	})
}

func TestChallengeManager_ValidateChallenge(t *testing.T) {
	cm := NewChallengeManager()

	t.Run("validates unused challenge", func(t *testing.T) {
		challenge, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)

		deviceID, valid := cm.ValidateChallenge(challenge)
		assert.True(t, valid)
		assert.Equal(t, "device1", deviceID)
	})

	t.Run("rejects used challenge", func(t *testing.T) {
		challenge, err := cm.GenerateChallenge("device1", 5*time.Minute)
		require.NoError(t, err)

		// Use challenge once
		_, valid := cm.ValidateChallenge(challenge)
		assert.True(t, valid)

		// Try to use again
		_, valid = cm.ValidateChallenge(challenge)
		assert.False(t, valid, "challenge should not be reusable")
	})

	t.Run("rejects expired challenge", func(t *testing.T) {
		challenge, err := cm.GenerateChallenge("device1", 1*time.Millisecond)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		_, valid := cm.ValidateChallenge(challenge)
		assert.False(t, valid, "expired challenge should be rejected")
	})

	t.Run("rejects non-existent challenge", func(t *testing.T) {
		_, valid := cm.ValidateChallenge("invalid-challenge")
		assert.False(t, valid)
	})
}

func TestChallengeManager_CleanupExpired(t *testing.T) {
	cm := NewChallengeManager()

	t.Run("removes expired challenges", func(t *testing.T) {
		// Create expired challenge
		challenge1, err := cm.GenerateChallenge("device1", 1*time.Millisecond)
		require.NoError(t, err)

		// Create valid challenge
		challenge2, err := cm.GenerateChallenge("device2", 5*time.Minute)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		cm.CleanupExpired()

		// Expired challenge should be gone
		_, valid := cm.ValidateChallenge(challenge1)
		assert.False(t, valid)

		// Valid challenge should still exist
		_, valid = cm.ValidateChallenge(challenge2)
		assert.True(t, valid)
	})
}

func TestGenerateSecurePassword(t *testing.T) {
	t.Run("generates password of correct length", func(t *testing.T) {
		password, err := generateSecurePassword(32)
		require.NoError(t, err)
		assert.Len(t, password, 32)
	})

	t.Run("generates unique passwords", func(t *testing.T) {
		passwords := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			password, err := generateSecurePassword(32)
			require.NoError(t, err)
			assert.False(t, passwords[password], "duplicate password generated")
			passwords[password] = true
		}
	})
}

func BenchmarkGenerateChallenge(b *testing.B) {
	cm := NewChallengeManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cm.GenerateChallenge("device1", 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateChallenge(b *testing.B) {
	cm := NewChallengeManager()
	challenge, _ := cm.GenerateChallenge("device1", 5*time.Minute)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.ValidateChallenge(challenge)
		// Regenerate for next iteration
		challenge, _ = cm.GenerateChallenge("device1", 5*time.Minute)
	}
}
