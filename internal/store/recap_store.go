package store

import (
	"database/sql"
	"errors"
	"time"
)

// RecapCandidate is a user eligible for a weekly recap email.
type RecapCandidate struct {
	UserID   int
	Username string
	Email    string
}

// RecapSummary is one user's activity over a week.
type RecapSummary struct {
	Sessions     int
	Volume       float64
	BestExercise string  // empty when the week had no weighted sets
	BestE1RM     float64 // Epley estimate for BestExercise
}

type RecapStore interface {
	// DueForRecap returns non-demo users who trained since cutoff and have not
	// received a recap since cutoff.
	DueForRecap(cutoff time.Time) ([]RecapCandidate, error)
	// WeeklyRecap aggregates a user's activity since the given time.
	WeeklyRecap(userID int, since time.Time) (RecapSummary, error)
	// MarkRecapSent claims the recap for a user: it stamps last_recap_sent_at and
	// reports whether this call won the claim (false if another already sent since
	// cutoff), which makes concurrent/duplicate sends safe.
	MarkRecapSent(userID int, at, cutoff time.Time) (bool, error)
}

type PostgresRecapStore struct {
	db *sql.DB
}

func NewPostgresRecapStore(db *sql.DB) *PostgresRecapStore {
	return &PostgresRecapStore{db: db}
}

func (s *PostgresRecapStore) DueForRecap(cutoff time.Time) ([]RecapCandidate, error) {
	query := `
	SELECT DISTINCT u.id, u.username, u.email
	FROM users u
	JOIN workouts w ON w.user_id = u.id
	WHERE u.username <> $2
	  AND w.created_at >= $1
	  AND (u.last_recap_sent_at IS NULL OR u.last_recap_sent_at < $1)
	ORDER BY u.id`

	rows, err := s.db.Query(query, cutoff, DemoUsername)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	candidates := []RecapCandidate{}
	for rows.Next() {
		var c RecapCandidate
		if err := rows.Scan(&c.UserID, &c.Username, &c.Email); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

func (s *PostgresRecapStore) WeeklyRecap(userID int, since time.Time) (RecapSummary, error) {
	var out RecapSummary

	totals := `
	SELECT COUNT(DISTINCT w.id),
	       COALESCE(SUM(e.sets * e.reps * e.weight) FILTER (WHERE e.reps IS NOT NULL AND e.weight IS NOT NULL), 0)
	FROM workouts w
	LEFT JOIN workout_entries e ON e.workout_id = w.id
	WHERE w.user_id = $1 AND w.created_at >= $2`
	if err := s.db.QueryRow(totals, userID, since).Scan(&out.Sessions, &out.Volume); err != nil {
		return out, err
	}

	best := `
	SELECT x.name, MAX(e.weight * (1 + e.reps / 30.0)) AS e1rm
	FROM workout_entries e
	JOIN workouts w ON w.id = e.workout_id
	JOIN exercises x ON x.id = e.exercise_id
	WHERE w.user_id = $1 AND w.created_at >= $2 AND e.reps IS NOT NULL AND e.weight IS NOT NULL
	GROUP BY x.name
	ORDER BY e1rm DESC
	LIMIT 1`
	err := s.db.QueryRow(best, userID, since).Scan(&out.BestExercise, &out.BestE1RM)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	return out, nil
}

func (s *PostgresRecapStore) MarkRecapSent(userID int, at, cutoff time.Time) (bool, error) {
	res, err := s.db.Exec(`
	UPDATE users SET last_recap_sent_at = $1
	WHERE id = $2 AND (last_recap_sent_at IS NULL OR last_recap_sent_at < $3)`,
		at, userID, cutoff)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
