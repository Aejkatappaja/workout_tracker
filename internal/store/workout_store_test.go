package store

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("pgx", "host=localhost user=postgres password=postgres dbname=postgres port=5433 sslmode=disable")
	if err != nil {
		t.Fatalf("opening test db %v", err)
	}

	// run the migration for our test db
	err = Migrate(db, "../../migrations/")
	if err != nil {
		t.Fatalf("migrating test db error: %v", err)
	}
	_, err = db.Exec(`TRUNCATE users, workouts, workout_entries CASCADE`)
	if err != nil {
		t.Fatalf("truncating tables %v", err)
	}

	return db
}

func createTestUser(t *testing.T, db *sql.DB) int {
	var id int
	err := db.QueryRow(
		`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		"tester", "tester@example.com", "hash",
	).Scan(&id)
	if err != nil {
		t.Fatalf("creating test user %v", err)
	}
	return id
}

func TestCreateWorkout(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	store := NewPostgresWorkoutStore(db)

	userID := createTestUser(t, db)

	tests := []struct {
		name    string
		workout *Workout
		wantErr bool
	}{
		{
			name: "valid workout",
			workout: &Workout{
				UserID:          userID,
				Title:           "push day",
				Description:     "upper body day",
				DurationMinutes: 60,
				CaloriesBurned:  200,
				Entries: []WorkoutEntry{
					{
						ExerciseName: "Bench press",
						Sets:         3,
						Reps:         IntPtr(10),
						Weight:       FloatPtr(135.5),
						Notes:        "warm up properly",
						OrderIndex:   1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "workout with invalid entries",
			workout: &Workout{
				UserID:          userID,
				Title:           "full body",
				Description:     "complete workout",
				DurationMinutes: 90,
				CaloriesBurned:  500,
				Entries: []WorkoutEntry{
					{
						ExerciseName: "Plan",
						Sets:         3,
						Reps:         IntPtr(60),
						Notes:        "keep form",
						OrderIndex:   1,
					},
					{
						ExerciseName:    "squats",
						Sets:            4,
						Reps:            IntPtr(12),
						DurationSeconds: IntPtr(60),
						Weight:          FloatPtr(185.0),
						Notes:           "full depth",
						OrderIndex:      2,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdWorkout, err := store.CreateWorkout(tt.workout)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.workout.Title, createdWorkout.Title)
			assert.Equal(t, tt.workout.Description, createdWorkout.Description)
			assert.Equal(t, tt.workout.DurationMinutes, createdWorkout.DurationMinutes)

			retrieved, err := store.GetWorkoutByID(int64(createdWorkout.ID))
			require.NoError(t, err)

			assert.Equal(t, createdWorkout.ID, retrieved.ID)
			assert.Equal(t, len(tt.workout.Entries), len(retrieved.Entries))

			for i := range retrieved.Entries {
				assert.Equal(t, tt.workout.Entries[i].ExerciseName, retrieved.Entries[i].ExerciseName)
				assert.Equal(t, tt.workout.Entries[i].Sets, retrieved.Entries[i].Sets)
				assert.Equal(t, tt.workout.Entries[i].OrderIndex, retrieved.Entries[i].OrderIndex)
			}
		})
	}
}

func TestGetWorkoutByID(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	st := NewPostgresWorkoutStore(db)
	userID := createTestUser(t, db)

	created, err := st.CreateWorkout(&Workout{
		UserID:          userID,
		Title:           "push day",
		Description:     "upper body",
		DurationMinutes: 60,
		CaloriesBurned:  200,
		Entries: []WorkoutEntry{
			{ExerciseName: "bench press", Sets: 3, Reps: IntPtr(10), OrderIndex: 1},
		},
	})
	require.NoError(t, err)
	require.NotZero(t, created.Entries[0].ID, "batch insert must assign entry ids")

	got, err := st.GetWorkoutByID(int64(created.ID))
	require.NoError(t, err)
	assert.Equal(t, userID, got.UserID, "user_id must be selected by GetWorkoutByID")
	assert.Equal(t, "push day", got.Title)
	assert.Len(t, got.Entries, 1)

	missing, err := st.GetWorkoutByID(999999)
	require.NoError(t, err)
	assert.Nil(t, missing, "missing workout returns (nil, nil)")
}

func TestUpdateWorkout(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	st := NewPostgresWorkoutStore(db)
	userID := createTestUser(t, db)

	created, err := st.CreateWorkout(&Workout{
		UserID:          userID,
		Title:           "push day",
		DurationMinutes: 60,
		Entries: []WorkoutEntry{
			{ExerciseName: "bench press", Sets: 3, Reps: IntPtr(10), OrderIndex: 1},
			{ExerciseName: "dips", Sets: 3, Reps: IntPtr(12), OrderIndex: 2},
		},
	})
	require.NoError(t, err)

	created.Title = "pull day"
	created.Entries = []WorkoutEntry{
		{ExerciseName: "barbell row", Sets: 4, Reps: IntPtr(8), OrderIndex: 1},
	}
	require.NoError(t, st.UpdateWorkout(created))

	got, err := st.GetWorkoutByID(int64(created.ID))
	require.NoError(t, err)
	assert.Equal(t, "pull day", got.Title)
	assert.Len(t, got.Entries, 1, "old entries must be replaced, not appended")
	assert.Equal(t, "barbell row", got.Entries[0].ExerciseName)
}

func TestDeleteWorkoutByID(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	st := NewPostgresWorkoutStore(db)
	userID := createTestUser(t, db)

	created, err := st.CreateWorkout(&Workout{
		UserID:          userID,
		Title:           "leg day",
		DurationMinutes: 45,
		Entries: []WorkoutEntry{
			{ExerciseName: "squat", Sets: 5, Reps: IntPtr(5), OrderIndex: 1},
		},
	})
	require.NoError(t, err)

	require.NoError(t, st.DeleteWorkoutByID(int64(created.ID)))

	got, err := st.GetWorkoutByID(int64(created.ID))
	require.NoError(t, err)
	assert.Nil(t, got)

	var entryCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM workout_entries WHERE workout_id = $1`, created.ID).Scan(&entryCount)
	require.NoError(t, err)
	assert.Equal(t, 0, entryCount, "entries must be removed by cascade")

	err = st.DeleteWorkoutByID(999999)
	assert.ErrorIs(t, err, sql.ErrNoRows, "deleting a missing workout returns ErrNoRows")
}

func TestGetWorkoutOwner(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	st := NewPostgresWorkoutStore(db)
	userID := createTestUser(t, db)

	created, err := st.CreateWorkout(&Workout{
		UserID:          userID,
		Title:           "push day",
		DurationMinutes: 60,
		Entries: []WorkoutEntry{
			{ExerciseName: "bench press", Sets: 3, Reps: IntPtr(10), OrderIndex: 1},
		},
	})
	require.NoError(t, err)

	owner, err := st.GetWorkoutOwner(int64(created.ID))
	require.NoError(t, err)
	assert.Equal(t, userID, owner)

	_, err = st.GetWorkoutOwner(999999)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func IntPtr(i int) *int {
	return &i
}

func FloatPtr(i float64) *float64 {
	return &i
}
