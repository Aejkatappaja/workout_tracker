package store

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigurePool checks the pool cap without opening a real connection:
// sql.Open is lazy, and Stats reports the configured limit immediately. This
// guards the fix for the "too many clients" exhaustion found under load test.
func TestConfigurePool(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://user:pass@localhost:5432/db")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	configurePool(db)
	assert.Equal(t, 25, db.Stats().MaxOpenConnections)
}
