package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Workout struct {
	ID              int            `json:"id"`
	UserID          int            `json:"user_id"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	DurationMinutes int            `json:"duration_minutes"`
	CaloriesBurned  int            `json:"calories_burned"`
	Entries         []WorkoutEntry `json:"entries"`
}

type WorkoutEntry struct {
	ID              int      `json:"id"`
	ExerciseID      int      `json:"exercise_id"`
	ExerciseName    string   `json:"exercise_name"` // input on write, resolved from the catalog on read
	MuscleGroup     string   `json:"muscle_group"`  // read-only, from the catalog
	Sets            int      `json:"sets"`
	Reps            *int     `json:"reps"`
	DurationSeconds *int     `json:"duration_seconds"`
	Weight          *float64 `json:"weight"`
	Notes           string   `json:"notes"`
	OrderIndex      int      `json:"order_index"`
}

type PostgresWorkoutStore struct {
	db *sql.DB
}

func NewPostgresWorkoutStore(db *sql.DB) *PostgresWorkoutStore {
	return &PostgresWorkoutStore{db: db}
}

type WorkoutStore interface {
	CreateWorkout(*Workout) (*Workout, error)
	GetWorkoutByID(id int64) (*Workout, error)
	UpdateWorkout(*Workout) error
	DeleteWorkoutByID(id int64, userID int) error
	GetWorkoutOwner(id int64) (int, error)
	ListWorkoutsByUser(userID int) ([]Workout, error)
	WorkoutCountsByDay(userID int, since time.Time) (map[string]int, error)
}

// insertWorkoutEntries inserts all entries in a single statement and assigns the
// generated ids back onto the entries slice (RETURNING order matches VALUES order).
func insertWorkoutEntries(tx *sql.Tx, workoutID int, entries []WorkoutEntry) error {
	if len(entries) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(entries))
	args := make([]interface{}, 0, len(entries)*8)
	for i := range entries {
		exerciseID, err := getOrCreateExercise(tx, entries[i].ExerciseName, entries[i].MuscleGroup)
		if err != nil {
			return err
		}
		entries[i].ExerciseID = exerciseID

		n := i * 8
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			n+1, n+2, n+3, n+4, n+5, n+6, n+7, n+8))
		args = append(args, workoutID, exerciseID, entries[i].Sets, entries[i].Reps,
			entries[i].DurationSeconds, entries[i].Weight, entries[i].Notes, entries[i].OrderIndex)
	}

	query := `
	INSERT INTO workout_entries (workout_id, exercise_id, sets, reps, duration_seconds, weight, notes, order_index)
	VALUES ` + strings.Join(placeholders, ", ") + `
	RETURNING id`

	rows, err := tx.Query(query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	i := 0
	for rows.Next() {
		if err := rows.Scan(&entries[i].ID); err != nil {
			return err
		}
		i++
	}
	return rows.Err()
}

var validMuscleGroups = map[string]bool{
	"chest": true, "back": true, "legs": true, "shoulders": true,
	"arms": true, "core": true, "cardio": true, "other": true,
}

// getOrCreateExercise resolves a free-text exercise name to a catalog row,
// inserting it (lower-cased, trimmed) if new, and returns its id. muscleGroup is
// only applied when the row is created; an existing exercise keeps its group.
// This keeps workout_entries normalized while letting the UI send plain names.
func getOrCreateExercise(tx *sql.Tx, name, muscleGroup string) (int, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return 0, errors.New("exercise name is required")
	}
	group := strings.ToLower(strings.TrimSpace(muscleGroup))
	if !validMuscleGroups[group] {
		group = "other"
	}
	var id int
	err := tx.QueryRow(
		`INSERT INTO exercises (name, muscle_group) VALUES ($1, $2)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, n, group).Scan(&id)
	return id, err
}

func (pg *PostgresWorkoutStore) CreateWorkout(workout *Workout) (*Workout, error) {
	tx, err := pg.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	query := `
  INSERT INTO workouts (user_id, title, description, duration_minutes, calories_burned)
  VALUES ($1, $2, $3, $4, $5)
  RETURNING id 
  `

	err = tx.QueryRow(query, workout.UserID, workout.Title, workout.Description, workout.DurationMinutes, workout.CaloriesBurned).Scan(&workout.ID)
	if err != nil {
		return nil, err
	}

	if err := insertWorkoutEntries(tx, workout.ID, workout.Entries); err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return workout, nil
}

// ListWorkoutsByUser returns the user's workouts without their entries (the list
// view only needs the summary; the detail view loads entries via GetWorkoutByID).
func (pg *PostgresWorkoutStore) ListWorkoutsByUser(userID int) ([]Workout, error) {
	query := `
	SELECT id, user_id, title, description, duration_minutes, calories_burned
	FROM workouts
	WHERE user_id = $1
	ORDER BY id DESC
	`

	rows, err := pg.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	workouts := []Workout{}
	for rows.Next() {
		var w Workout
		if err := rows.Scan(&w.ID, &w.UserID, &w.Title, &w.Description, &w.DurationMinutes, &w.CaloriesBurned); err != nil {
			return nil, err
		}
		workouts = append(workouts, w)
	}
	return workouts, rows.Err()
}

// WorkoutCountsByDay returns, for the user, the number of workouts created per
// day since the given date, keyed by "YYYY-MM-DD". Used to build the activity heatmap.
func (pg *PostgresWorkoutStore) WorkoutCountsByDay(userID int, since time.Time) (map[string]int, error) {
	query := `
	SELECT to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day, COUNT(*)
	FROM workouts
	WHERE user_id = $1 AND created_at >= $2
	GROUP BY day
	`

	rows, err := pg.db.Query(query, userID, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	counts := map[string]int{}
	for rows.Next() {
		var day string
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			return nil, err
		}
		counts[day] = n
	}
	return counts, rows.Err()
}

func (pg *PostgresWorkoutStore) GetWorkoutByID(id int64) (*Workout, error) {
	workout := &Workout{}
	query := `
	SELECT id, user_id, title, description, duration_minutes, calories_burned
	FROM workouts
	WHERE id = $1
	`

	err := pg.db.QueryRow(query, id).Scan(&workout.ID, &workout.UserID, &workout.Title, &workout.Description, &workout.DurationMinutes, &workout.CaloriesBurned)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	entryQuery := `
	SELECT e.id, e.exercise_id, x.name, x.muscle_group, e.sets, e.reps, e.duration_seconds, e.weight, e.notes, e.order_index
	FROM workout_entries e
	JOIN exercises x ON x.id = e.exercise_id
	WHERE e.workout_id = $1
	ORDER BY e.order_index
	`

	rows, err := pg.db.Query(entryQuery, id)
	if err != nil {
		fmt.Printf("Error querying workout entries: %v\n", err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var entry WorkoutEntry
		err = rows.Scan(&entry.ID, &entry.ExerciseID, &entry.ExerciseName, &entry.MuscleGroup, &entry.Sets, &entry.Reps, &entry.DurationSeconds, &entry.Weight, &entry.Notes, &entry.OrderIndex)
		if err != nil {
			return nil, err
		}
		workout.Entries = append(workout.Entries, entry)
	}

	return workout, nil
}

func (pg *PostgresWorkoutStore) UpdateWorkout(workout *Workout) error {
	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := `
  UPDATE workouts
  SET title = $1, description = $2, duration_minutes = $3, calories_burned = $4
  WHERE id = $5 AND user_id = $6
  `
	result, err := tx.Exec(query, workout.Title, workout.Description, workout.DurationMinutes, workout.CaloriesBurned, workout.ID, workout.UserID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	_, err = tx.Exec(`DELETE FROM workout_entries WHERE workout_id = $1`, workout.ID)
	if err != nil {
		return err
	}

	if err := insertWorkoutEntries(tx, workout.ID, workout.Entries); err != nil {
		return err
	}

	return tx.Commit()
}

func (pg *PostgresWorkoutStore) DeleteWorkoutByID(id int64, userID int) error {
	query := `
	DELETE FROM workouts
	WHERE id = $1 AND user_id = $2
	`
	result, err := pg.db.Exec(query, id, userID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (pg *PostgresWorkoutStore) GetWorkoutOwner(workoutID int64) (int, error) {
	var userID int

	query := `
	SELECT user_id
	FROM workouts
	WHERE id = $1
	`
	err := pg.db.QueryRow(query, workoutID).Scan(&userID)
	if err != nil {
		return 0, err
	}
	return userID, nil
}
