package store

import (
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"math/rand"
	"time"
)

// DemoUsername is the read-only showcase account seeded on startup.
const DemoUsername = "demo"

type seedExercise struct {
	name     string
	sets     int
	reps     *int
	duration *int // seconds; exactly one of reps/duration is set
	weight   *float64
}

var strengthPool = []seedExercise{
	{"Bench Press", 3, ptr(10), nil, fptr(60)},
	{"Back Squat", 5, ptr(5), nil, fptr(90)},
	{"Deadlift", 3, ptr(5), nil, fptr(120)},
	{"Overhead Press", 3, ptr(8), nil, fptr(40)},
	{"Pull Up", 4, ptr(8), nil, nil},
	{"Barbell Row", 4, ptr(10), nil, fptr(50)},
	{"Bicep Curl", 3, ptr(12), nil, fptr(14)},
}

var cardioPool = []seedExercise{
	{"Plank", 3, nil, ptr(60), nil},
	{"Running", 1, nil, ptr(1500), nil},
	{"Rowing", 1, nil, ptr(900), nil},
	{"Jump Rope", 3, nil, ptr(180), nil},
	{"Cycling", 1, nil, ptr(1800), nil},
}

var sessionTitles = []string{"Push Day", "Pull Day", "Leg Day", "Full Body", "Cardio", "Upper Body"}

// SeedDemo creates the read-only demo account with backdated workouts if it does
// not already exist. Idempotent: safe to call on every startup.
func SeedDemo(db *sql.DB) error {
	users := NewPostgresUserStore(db)

	existing, err := users.GetUserByUsername(DemoUsername)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	demo := &User{Username: DemoUsername, Email: "demo@go-gym.local", Bio: "Read-only showcase account."}
	// unusable password: the demo is entered via a button, never by password.
	if err := demo.PasswordHash.Set(randomToken(24)); err != nil {
		return err
	}
	if err := users.CreateUser(demo); err != nil {
		return err
	}

	return seedWorkouts(db, demo.ID)
}

func seedWorkouts(db *sql.DB, userID int) error {
	rng := rand.New(rand.NewSource(42))

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	insertWorkout := `
	INSERT INTO workouts (user_id, title, description, duration_minutes, calories_burned, created_at)
	VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	insertEntry := `
	INSERT INTO workout_entries (workout_id, exercise_id, sets, reps, duration_seconds, weight, notes, order_index)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	now := time.Now()
	for daysAgo := 0; daysAgo < 84; daysAgo++ {
		// train roughly 4 days a week
		if rng.Float64() > 0.55 {
			continue
		}
		title := sessionTitles[rng.Intn(len(sessionTitles))]
		cardio := title == "Cardio"
		pool := strengthPool
		if cardio {
			pool = cardioPool
		}

		createdAt := now.AddDate(0, 0, -daysAgo).Add(-time.Duration(rng.Intn(6)) * time.Hour)
		duration := 30 + rng.Intn(45)
		calories := 150 + rng.Intn(350)

		var workoutID int
		if err := tx.QueryRow(insertWorkout, userID, title, "Seeded demo session.", duration, calories, createdAt).Scan(&workoutID); err != nil {
			return err
		}

		n := 3 + rng.Intn(3) // 3..5 exercises
		for i := 0; i < n; i++ {
			e := pool[rng.Intn(len(pool))]
			exerciseID, err := getOrCreateExercise(tx, e.name)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(insertEntry, workoutID, exerciseID, e.sets, e.reps, e.duration, e.weight, "", i+1); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func ptr(i int) *int          { return &i }
func fptr(f float64) *float64 { return &f }

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}
