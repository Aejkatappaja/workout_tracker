package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalytics(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	ws := NewPostgresWorkoutStore(db)
	es := NewPostgresExerciseStore(db)
	as := NewPostgresAnalyticsStore(db)
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

	var benchID int
	ex, err := es.Search("bench press", 5)
	require.NoError(t, err)
	for _, e := range ex {
		if e.Name == "bench press" {
			benchID = e.ID
		}
	}
	require.NotZero(t, benchID)

	// progression: one day, best e1RM (Epley) + total volume
	prog, err := as.ExerciseProgress(userID, benchID)
	require.NoError(t, err)
	require.Len(t, prog, 1)
	assert.InDelta(t, 100*(1+5.0/30), prog[0].E1RM, 0.01)
	assert.InDelta(t, 3*5*100.0, prog[0].Volume, 0.01)

	// PRs ranked by e1RM desc: squat (140) beats bench (100)
	prs, err := as.PersonalRecords(userID)
	require.NoError(t, err)
	require.Len(t, prs, 2)
	assert.Equal(t, "squat", prs[0].Exercise)
	assert.InDelta(t, 140*(1+5.0/30), prs[0].E1RM, 0.01)
	assert.Equal(t, "bench press", prs[1].Exercise)
}
