package auth

import (
	"log/slog"
	"sync"
	"time"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                          // Failing, reject requests
	StateHalfOpen                      // Testing if service recovered
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures  int
	timeout      time.Duration
	state        CircuitState
	failures     int
	lastFailTime time.Time
	mu           sync.RWMutex
	logger       *slog.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, timeout time.Duration, logger *slog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       StateClosed,
		logger:      logger,
	}
}

// Call executes the function if the circuit is closed or half-open
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.AllowRequest() {
		return ErrCircuitOpen
	}

	err := fn()
	cb.RecordResult(err)
	return err
}

// AllowRequest checks if a request should be allowed
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.RLock()
	state := cb.state
	lastFail := cb.lastFailTime
	cb.mu.RUnlock()

	if state == StateClosed {
		return true
	}

	if state == StateOpen {
		// Check if timeout has elapsed
		if time.Since(lastFail) > cb.timeout {
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			if cb.logger != nil {
				cb.logger.Info("Circuit breaker half-open - testing service recovery")
			}
			return true
		}
		return false
	}

	// StateHalfOpen - allow one request to test
	return true
}

// RecordResult records the result of a request
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()

		if cb.failures >= cb.maxFailures && cb.state != StateOpen {
			cb.state = StateOpen
			if cb.logger != nil {
				cb.logger.Warn("Circuit breaker opened - service unavailable",
					"failures", cb.failures,
					"max_failures", cb.maxFailures,
					"last_fail_time", cb.lastFailTime,
				)
			}
		}
	} else {
		// Success - reset
		oldState := cb.state
		cb.failures = 0
		if cb.state == StateHalfOpen {
			cb.state = StateClosed
			if cb.logger != nil {
				cb.logger.Info("Circuit breaker closed - service recovered")
			}
		}
		// Log if we recovered from failures in closed state
		if oldState == StateClosed && cb.failures > 0 && cb.logger != nil {
			cb.logger.Debug("Circuit breaker recovered from failures", "previous_failures", cb.failures)
		}
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
}
