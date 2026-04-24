package repository

import (
	"context"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReadExecutor_ReturnsReaderWithoutTx(t *testing.T) {
	db := testutil.ConnectDB(t)
	defer db.Close()

	ctx := context.Background()
	exec := getReadExecutor(ctx, db.Reader)
	assert.Equal(t, db.Reader, exec, "should return reader pool when no tx in context")
}

func TestGetReadExecutor_ReturnsTxWhenActive(t *testing.T) {
	db := testutil.ConnectDB(t)
	defer db.Close()

	tx, err := db.Writer.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(context.Background(), txKey{}, tx)
	exec := getReadExecutor(txCtx, db.Reader)
	assert.Equal(t, tx, exec, "should return tx when tx is in context, not reader")
}

func TestGetReadExecutor_PanicsOnUnsupportedType(t *testing.T) {
	assert.Panics(t, func() {
		getReadExecutor(context.Background(), "not-a-db")
	})
}

func TestNewDeviceRepository_AsymmetricNilArgs(t *testing.T) {
	t.Run("nil writer with valid reader", func(t *testing.T) {
		db := testutil.ConnectDB(t)
		defer db.Close()

		_, err := NewDeviceRepository(nil, db.Reader)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "writer")
	})

	t.Run("valid writer with nil reader", func(t *testing.T) {
		db := testutil.ConnectDB(t)
		defer db.Close()

		_, err := NewDeviceRepository(db.Writer, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reader")
	})
}
