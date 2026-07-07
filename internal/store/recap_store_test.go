package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecapStore(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	ws := NewPostgresWorkoutStore(db)
	rs := NewPostgresRecapStore(db)
	userID := createTestUser(t, db)

	_, err := ws.CreateWorkout(&Workout{
		UserID:          userID,
		Title:           "session",
		DurationMinutes: 30,
		Entries: []WorkoutEntry{
			{ExerciseName: "bench press", Sets: 3, Reps: IntPtr(5), Weight: FloatPtr(100), OrderIndex: 1},
			{ExerciseName: "squat", Sets: 3, Reps: IntPtr(5), Weight: FloatPtr(140), OrderIndex: 2},
		},
	})
	require.NoError(t, err)

	now := time.Now()
	cutoff := now.Add(-7 * 24 * time.Hour)

	// the fresh workout makes the user due, with no prior recap.
	due, err := rs.DueForRecap(cutoff)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, "tester", due[0].Username)
	assert.Equal(t, "tester@example.com", due[0].Email)

	sum, err := rs.WeeklyRecap(userID, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Sessions)
	assert.InDelta(t, 3*5*100.0+3*5*140.0, sum.Volume, 0.01) // 3600
	assert.Equal(t, "squat", sum.BestExercise)               // 140 beats 100
	assert.InDelta(t, 140*(1+5.0/30), sum.BestE1RM, 0.01)

	// claiming stamps the send and wins once.
	ok, err := rs.MarkRecapSent(userID, now, cutoff)
	require.NoError(t, err)
	assert.True(t, ok)

	// after a claim the user is no longer due, and a second claim loses.
	due, err = rs.DueForRecap(cutoff)
	require.NoError(t, err)
	assert.Empty(t, due)

	ok, err = rs.MarkRecapSent(userID, now, cutoff)
	require.NoError(t, err)
	assert.False(t, ok)
}
