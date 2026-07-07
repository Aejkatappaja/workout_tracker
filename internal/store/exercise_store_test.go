package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExerciseCatalog(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	es := NewPostgresExerciseStore(db)

	all, err := es.List()
	require.NoError(t, err)
	assert.NotEmpty(t, all, "migration seeds a starter catalog")

	res, err := es.Search("bench", 8)
	require.NoError(t, err)
	require.NotEmpty(t, res)
	for _, e := range res {
		assert.True(t, strings.HasPrefix(e.Name, "bench"), "prefix search: %q", e.Name)
	}
}

func TestGetOrCreateExerciseViaWorkout(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	ws := NewPostgresWorkoutStore(db)
	es := NewPostgresExerciseStore(db)
	userID := createTestUser(t, db)

	// an unknown exercise name is added to the catalog (lower-cased) on write
	_, err := ws.CreateWorkout(&Workout{
		UserID:          userID,
		Title:           "novel",
		DurationMinutes: 10,
		Entries:         []WorkoutEntry{{ExerciseName: "Zercher Squat", Sets: 3, Reps: IntPtr(5), OrderIndex: 1}},
	})
	require.NoError(t, err)

	res, err := es.Search("zercher", 8)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "zercher squat", res[0].Name)

	// reusing the same name (any case) must not create a duplicate
	_, err = ws.CreateWorkout(&Workout{
		UserID:          userID,
		Title:           "again",
		DurationMinutes: 10,
		Entries:         []WorkoutEntry{{ExerciseName: "ZERCHER SQUAT", Sets: 3, Reps: IntPtr(5), OrderIndex: 1}},
	})
	require.NoError(t, err)

	res, err = es.Search("zercher", 8)
	require.NoError(t, err)
	assert.Len(t, res, 1, "case-insensitive dedup")
}
