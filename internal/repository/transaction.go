package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// txKey is the context key for storing transaction
type txKey struct{}

// IsolationLevel represents transaction isolation levels
type IsolationLevel string

const (
	// IsolationDefault uses the database's default isolation level
	IsolationDefault IsolationLevel = ""
	// IsolationReadCommitted prevents dirty reads
	IsolationReadCommitted IsolationLevel = "READ COMMITTED"
	// IsolationSerializable provides full isolation
	IsolationSerializable IsolationLevel = "SERIALIZABLE"
)

// Transactor provides transaction management capabilities
type Transactor interface {
	// WithTransaction executes a function within a database transaction
	// If the function returns an error, the transaction is rolled back
	// Otherwise, the transaction is committed
	WithTransaction(ctx context.Context, fn func(context.Context) error) error
	
	// WithTransactionIsolation executes a function within a transaction with specified isolation level
	WithTransactionIsolation(ctx context.Context, isolation IsolationLevel, fn func(context.Context) error) error
}

// transactor implements the Transactor interface
type transactor struct {
	db executor
}

// NewTransactor creates a new Transactor
// Accepts either *sql.DB or any type that embeds *sql.DB
func NewTransactor(db interface{}) (Transactor, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	
	// Handle both *sql.DB and wrapped types
	switch v := db.(type) {
	case *sql.DB:
		return &transactor{db: v}, nil
	case executor:
		return &transactor{db: v}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %T", db)
	}
}

// WithTransaction executes fn within a transaction with default isolation level
func (t *transactor) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return t.WithTransactionIsolation(ctx, IsolationDefault, fn)
}

// WithTransactionIsolation executes fn within a transaction with specified isolation level
func (t *transactor) WithTransactionIsolation(ctx context.Context, isolation IsolationLevel, fn func(context.Context) error) error {
	// Check if we're already in a transaction
	if tx := getTx(ctx); tx != nil {
		// Already in a transaction, just execute the function
		return fn(ctx)
	}

	// Prepare transaction options
	opts := &sql.TxOptions{}
	if isolation != IsolationDefault {
		opts.Isolation = toSQLIsolation(isolation)
	}

	// Begin new transaction
	var tx *sql.Tx
	var err error
	
	switch db := t.db.(type) {
	case *sql.DB:
		tx, err = db.BeginTx(ctx, opts)
	case interface{ BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) }:
		tx, err = db.BeginTx(ctx, opts)
	default:
		return fmt.Errorf("database does not support transactions")
	}
	
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Ensure transaction is finalized
	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic
			_ = tx.Rollback()
			panic(p) // Re-throw panic after rollback
		}
	}()

	// Create context with transaction
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Execute function with retry for serialization errors
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = fn(txCtx)
		
		if err == nil {
			break
		}
		
		// Retry on serialization failure (only for SERIALIZABLE isolation)
		if isolation == IsolationSerializable && isSerializationError(err) && attempt < maxRetries-1 {
			// Exponential backoff
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			continue
		}
		
		break
	}

	if err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// toSQLIsolation converts IsolationLevel to sql.IsolationLevel
func toSQLIsolation(level IsolationLevel) sql.IsolationLevel {
	switch level {
	case IsolationReadCommitted:
		return sql.LevelReadCommitted
	case IsolationSerializable:
		return sql.LevelSerializable
	default:
		return sql.LevelDefault
	}
}

// isSerializationError checks if an error is a serialization failure
func isSerializationError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "serialization") ||
		strings.Contains(errStr, "deadlock") ||
		strings.Contains(errStr, "could not serialize")
}

// getTx retrieves the transaction from context, if any
func getTx(ctx context.Context) *sql.Tx {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// getExecutor returns either the transaction or the database connection
// This allows repository methods to work both inside and outside transactions
func getExecutor(ctx context.Context, db interface{}) executor {
	if tx := getTx(ctx); tx != nil {
		return tx
	}
	
	// Handle both *sql.DB and wrapped types
	switch v := db.(type) {
	case *sql.DB:
		return v
	case executor:
		return v
	default:
		panic(fmt.Sprintf("unsupported database type: %T", db))
	}
}

// getReadExecutor returns the transaction if one is active (so reads see
// uncommitted writes within the same tx), otherwise returns the reader pool.
func getReadExecutor(ctx context.Context, reader interface{}) executor {
	if tx := getTx(ctx); tx != nil {
		return tx
	}

	switch v := reader.(type) {
	case *sql.DB:
		return v
	case executor:
		return v
	default:
		panic(fmt.Sprintf("unsupported database type: %T", reader))
	}
}

// executor is an interface that both *sql.DB and *sql.Tx implement
type executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// resolveExecutor converts an interface{} to an executor, returning an error
// if the type is nil or unsupported. Used by repository constructors.
func resolveExecutor(db interface{}, name string) (executor, error) {
	if db == nil {
		return nil, fmt.Errorf("%s cannot be nil", name)
	}
	switch v := db.(type) {
	case *sql.DB:
		return v, nil
	case executor:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported %s type: %T", name, db)
	}
}
