package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("starts in closed state", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second, nil)
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("allows requests when closed", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second, nil)
		assert.True(t, cb.AllowRequest())
	})

	t.Run("opens after max failures", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second, nil)
		testErr := errors.New("test error")

		// Record 3 failures
		for i := 0; i < 3; i++ {
			cb.RecordResult(testErr)
		}

		assert.Equal(t, StateOpen, cb.State())
	})

	t.Run("rejects requests when open", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second, nil)
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < 3; i++ {
			cb.RecordResult(testErr)
		}

		assert.False(t, cb.AllowRequest())
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond, nil)
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < 3; i++ {
			cb.RecordResult(testErr)
		}
		assert.Equal(t, StateOpen, cb.State())

		// Wait for timeout
		time.Sleep(150 * time.Millisecond)

		// Should allow request (half-open)
		assert.True(t, cb.AllowRequest())
		assert.Equal(t, StateHalfOpen, cb.State())
	})

	t.Run("closes on success in half-open state", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond, nil)
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < 3; i++ {
			cb.RecordResult(testErr)
		}

		// Wait for timeout
		time.Sleep(150 * time.Millisecond)
		cb.AllowRequest() // Transition to half-open

		// Record success
		cb.RecordResult(nil)

		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("reopens on failure in half-open state", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond, nil)
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < 3; i++ {
			cb.RecordResult(testErr)
		}

		// Wait for timeout
		time.Sleep(150 * time.Millisecond)
		cb.AllowRequest() // Transition to half-open

		// Record failure
		cb.RecordResult(testErr)

		assert.Equal(t, StateOpen, cb.State())
	})

	t.Run("Call executes function when closed", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second, nil)
		executed := false

		err := cb.Call(func() error {
			executed = true
			return nil
		})

		assert.NoError(t, err)
		assert.True(t, executed)
	})

	t.Run("Call returns ErrCircuitOpen when open", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second, nil)
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < 3; i++ {
			cb.RecordResult(testErr)
		}

		executed := false
		err := cb.Call(func() error {
			executed = true
			return nil
		})

		assert.Equal(t, ErrCircuitOpen, err)
		assert.False(t, executed)
	})

	t.Run("Reset closes the circuit", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second, nil)
		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < 3; i++ {
			cb.RecordResult(testErr)
		}
		assert.Equal(t, StateOpen, cb.State())

		// Reset
		cb.Reset()

		assert.Equal(t, StateClosed, cb.State())
		assert.True(t, cb.AllowRequest())
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		cb := NewCircuitBreaker(10, 1*time.Second, nil)
		done := make(chan bool)

		// Multiple goroutines recording results
		for i := 0; i < 10; i++ {
			go func(i int) {
				defer func() { done <- true }()
				for j := 0; j < 100; j++ {
					if i%2 == 0 {
						cb.RecordResult(nil)
					} else {
						cb.RecordResult(errors.New("error"))
					}
					cb.AllowRequest()
					_ = cb.State()
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should not panic
		assert.NotNil(t, cb)
	})
}

func TestCircuitBreakerIntegration(t *testing.T) {
	t.Run("protects service from repeated failures", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 1*time.Second, nil)
		callCount := 0
		failingService := func() error {
			callCount++
			return errors.New("service unavailable")
		}

		// Make 10 calls - only first 5 should execute
		for i := 0; i < 10; i++ {
			_ = cb.Call(failingService)
		}

		// Circuit should be open after 5 failures
		assert.Equal(t, StateOpen, cb.State())
		// Only 5 calls should have been made (before circuit opened)
		assert.Equal(t, 5, callCount)
	})

	t.Run("allows recovery after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond, nil)
		callCount := 0

		// Fail 3 times to open circuit
		for i := 0; i < 3; i++ {
			_ = cb.Call(func() error {
				callCount++
				return errors.New("error")
			})
		}
		assert.Equal(t, StateOpen, cb.State())
		assert.Equal(t, 3, callCount)

		// Wait for timeout
		time.Sleep(150 * time.Millisecond)

		// Next call should be allowed (half-open)
		err := cb.Call(func() error {
			callCount++
			return nil // Success
		})

		assert.NoError(t, err)
		assert.Equal(t, 4, callCount)
		assert.Equal(t, StateClosed, cb.State())
	})
}
