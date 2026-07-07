package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecapStore_WithLock(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	rs := NewPostgresRecapStore(db)
	ctx := context.Background()

	const key = int64(999001)
	inner := false
	ran, err := rs.WithLock(ctx, key, func() error {
		inner = true
		// a second attempt (different pool connection) must be refused while held
		ran2, err := rs.WithLock(ctx, key, func() error { return nil })
		require.NoError(t, err)
		assert.False(t, ran2, "the lock is not re-entrant across connections")
		return nil
	})
	require.NoError(t, err)
	assert.True(t, ran)
	assert.True(t, inner)

	// released after fn returns, so it can be taken again
	ran, err = rs.WithLock(ctx, key, func() error { return nil })
	require.NoError(t, err)
	assert.True(t, ran, "lock is available again after release")
}

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
