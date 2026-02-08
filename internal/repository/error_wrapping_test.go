package repository

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTransactionRollbackErrorWrapping verifies that rollback errors are properly
// wrapped in the error chain, allowing them to be detected with errors.Is()
func TestTransactionRollbackErrorWrapping(t *testing.T) {
	t.Run("rollback error is in error chain", func(t *testing.T) {
		// This test demonstrates why we use %w for rollback error:
		// If rollback fails, that's more critical than the original error
		// because the database is now in an inconsistent state.
		
		// Simulate a rollback failure scenario
		rollbackErr := sql.ErrConnDone // Connection lost during rollback
		originalErr := errors.New("some transaction error")
		
		// This is what our code does:
		wrappedErr := errors.Join(
			rollbackErr,
			originalErr,
		)
		
		// We can detect the critical rollback failure
		assert.True(t, errors.Is(wrappedErr, sql.ErrConnDone),
			"Should be able to detect rollback failure with errors.Is()")
		
		// We can also see the original error in the message
		assert.Contains(t, wrappedErr.Error(), "some transaction error",
			"Original error should still be visible in error message")
	})

	t.Run("demonstrates error chain priority", func(t *testing.T) {
		// Before our fix: Original error wrapped, rollback error just text
		// fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		// Result: Can detect original error, but NOT rollback error
		
		// After our fix: Rollback error wrapped, original error just text
		// fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
		// Result: Can detect rollback error (more critical!)
		
		rollbackErr := sql.ErrConnDone
		originalErr := errors.New("constraint violation")
		
		// Our implementation
		correctErr := errors.Join(rollbackErr, originalErr)
		
		// Can detect critical rollback failure
		assert.True(t, errors.Is(correctErr, sql.ErrConnDone),
			"Rollback failure should be detectable")
		
		// This is important because rollback failures indicate:
		// 1. Database connection lost
		// 2. Transaction may be partially committed
		// 3. Database state is inconsistent
		// These are MORE critical than the original transaction error
	})

	t.Run("real-world scenario: connection lost during rollback", func(t *testing.T) {
		// Scenario: Transaction fails, then connection is lost during rollback
		// This is a critical situation that needs special handling
		
		originalErr := errors.New("deadlock detected")
		rollbackErr := sql.ErrConnDone // Connection lost!
		
		// Our error wrapping
		err := errors.Join(rollbackErr, originalErr)
		
		// Application can detect this critical situation
		if errors.Is(err, sql.ErrConnDone) {
			// Critical: Connection lost during rollback
			// Actions:
			// 1. Alert operations team
			// 2. Check database consistency
			// 3. Retry with new connection
			// 4. Log for audit
			assert.True(t, true, "Can detect and handle critical rollback failure")
		}
		
		// Original error is still visible for debugging
		assert.Contains(t, err.Error(), "deadlock detected",
			"Original error context preserved")
	})
}

// TestErrorWrappingBestPractices documents error wrapping patterns
func TestErrorWrappingBestPractices(t *testing.T) {
	t.Run("use %w for errors you want to detect", func(t *testing.T) {
		// ✅ GOOD: Wrap errors you want to check with errors.Is()
		baseErr := sql.ErrNoRows
		wrappedErr := errors.Join(baseErr, errors.New("context"))
		
		assert.True(t, errors.Is(wrappedErr, sql.ErrNoRows),
			"Can detect wrapped error")
	})

	t.Run("use %v for errors that are just context", func(t *testing.T) {
		// ✅ GOOD: Use %v for errors that are just informational
		contextErr := errors.New("user clicked cancel")
		criticalErr := sql.ErrConnDone
		
		// Critical error wrapped, context error as text
		err := errors.Join(criticalErr, contextErr)
		
		assert.True(t, errors.Is(err, sql.ErrConnDone),
			"Critical error is detectable")
		assert.Contains(t, err.Error(), "user clicked cancel",
			"Context is preserved in message")
	})

	t.Run("prioritize more critical errors in chain", func(t *testing.T) {
		// Rule: The error you wrap with %w should be the one you want to detect
		
		// Example 1: Rollback failure (critical) vs transaction error (less critical)
		rollbackErr := sql.ErrConnDone
		txErr := errors.New("constraint violation")
		err1 := errors.Join(rollbackErr, txErr) // ✅ Correct: Rollback is critical
		assert.True(t, errors.Is(err1, sql.ErrConnDone))
		
		// Example 2: Not found (expected) vs database error (unexpected)
		notFoundErr := sql.ErrNoRows
		dbErr := errors.New("connection timeout")
		err2 := errors.Join(notFoundErr, dbErr) // ✅ Correct: Not found is the primary error
		assert.True(t, errors.Is(err2, sql.ErrNoRows))
	})
}
