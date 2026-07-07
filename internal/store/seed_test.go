package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedDemo(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	require.NoError(t, SeedDemo(db))

	users := NewPostgresUserStore(db)
	demo, err := users.GetUserByUsername(DemoUsername)
	require.NoError(t, err)
	require.NotNil(t, demo, "demo user must be created")

	workouts := NewPostgresWorkoutStore(db)
	list, err := workouts.ListWorkoutsByUser(demo.ID, 0, 1000)
	require.NoError(t, err)
	require.NotEmpty(t, list, "demo must have seeded workouts")
	seeded := len(list)

	// idempotent: a second call must not add anything
	require.NoError(t, SeedDemo(db))
	list2, err := workouts.ListWorkoutsByUser(demo.ID, 0, 1000)
	require.NoError(t, err)
	assert.Equal(t, seeded, len(list2), "second seed must be a no-op")
}
