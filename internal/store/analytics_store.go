package store

import "database/sql"

// ProgressPoint is one day on an exercise's progression curve.
type ProgressPoint struct {
	Day    string  `json:"day"`    // YYYY-MM-DD (UTC)
	E1RM   float64 `json:"e1rm"`   // best estimated 1-rep max that day (Epley)
	Volume float64 `json:"volume"` // total sets*reps*weight that day
}

// PersonalRecord is a user's best estimated 1RM for an exercise.
type PersonalRecord struct {
	ExerciseID  int     `json:"exercise_id"`
	Exercise    string  `json:"exercise"`
	MuscleGroup string  `json:"muscle_group"`
	Weight      float64 `json:"weight"`
	Reps        int     `json:"reps"`
	E1RM        float64 `json:"e1rm"`
	Day         string  `json:"day"`
}

type AnalyticsStore interface {
	ExerciseProgress(userID, exerciseID int) ([]ProgressPoint, error)
	PersonalRecords(userID int) ([]PersonalRecord, error)
}

type PostgresAnalyticsStore struct {
	db *sql.DB
}

func NewPostgresAnalyticsStore(db *sql.DB) *PostgresAnalyticsStore {
	return &PostgresAnalyticsStore{db: db}
}

// ExerciseProgress returns the daily best e1RM and total volume for one exercise,
// oldest first. Only rep-based sets with a weight contribute (e1RM needs both).
func (s *PostgresAnalyticsStore) ExerciseProgress(userID, exerciseID int) ([]ProgressPoint, error) {
	query := `
	SELECT to_char(w.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day,
	       MAX(e.weight * (1 + e.reps / 30.0)) AS e1rm,
	       COALESCE(SUM(e.sets * e.reps * e.weight), 0) AS volume
	FROM workout_entries e
	JOIN workouts w ON w.id = e.workout_id
	WHERE w.user_id = $1 AND e.exercise_id = $2 AND e.reps IS NOT NULL AND e.weight IS NOT NULL
	GROUP BY day
	ORDER BY day`

	rows, err := s.db.Query(query, userID, exerciseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	points := []ProgressPoint{}
	for rows.Next() {
		var p ProgressPoint
		if err := rows.Scan(&p.Day, &p.E1RM, &p.Volume); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// PersonalRecords returns each exercise's best e1RM set (the PR), strongest first.
func (s *PostgresAnalyticsStore) PersonalRecords(userID int) ([]PersonalRecord, error) {
	query := `
	SELECT exercise_id, exercise, muscle_group, weight, reps, e1rm, day FROM (
	  SELECT DISTINCT ON (e.exercise_id)
	         e.exercise_id,
	         x.name AS exercise,
	         x.muscle_group,
	         e.weight,
	         e.reps,
	         e.weight * (1 + e.reps / 30.0) AS e1rm,
	         to_char(w.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day
	  FROM workout_entries e
	  JOIN workouts w ON w.id = e.workout_id
	  JOIN exercises x ON x.id = e.exercise_id
	  WHERE w.user_id = $1 AND e.reps IS NOT NULL AND e.weight IS NOT NULL
	  ORDER BY e.exercise_id, e1rm DESC
	) best
	ORDER BY e1rm DESC`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	records := []PersonalRecord{}
	for rows.Next() {
		var pr PersonalRecord
		if err := rows.Scan(&pr.ExerciseID, &pr.Exercise, &pr.MuscleGroup, &pr.Weight, &pr.Reps, &pr.E1RM, &pr.Day); err != nil {
			return nil, err
		}
		records = append(records, pr)
	}
	return records, rows.Err()
}
