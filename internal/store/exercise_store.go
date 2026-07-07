package store

import (
	"database/sql"
	"strings"
)

type Exercise struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	MuscleGroup string `json:"muscle_group"`
}

type ExerciseStore interface {
	Search(q string, limit int) ([]Exercise, error)
	List() ([]Exercise, error)
}

type PostgresExerciseStore struct {
	db *sql.DB
}

func NewPostgresExerciseStore(db *sql.DB) *PostgresExerciseStore {
	return &PostgresExerciseStore{db: db}
}

// Search returns catalog exercises whose name starts with q (prefix match backed
// by the pg_trgm GIN index), for the typeahead.
func (s *PostgresExerciseStore) Search(q string, limit int) ([]Exercise, error) {
	if limit <= 0 || limit > 25 {
		limit = 8
	}
	query := `
	SELECT id, name, muscle_group
	FROM exercises
	WHERE name ILIKE $1 || '%'
	ORDER BY name
	LIMIT $2`
	return s.queryExercises(query, strings.ToLower(strings.TrimSpace(q)), limit)
}

func (s *PostgresExerciseStore) List() ([]Exercise, error) {
	return s.queryExercises(`SELECT id, name, muscle_group FROM exercises ORDER BY name`)
}

func (s *PostgresExerciseStore) queryExercises(query string, args ...interface{}) ([]Exercise, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	exercises := []Exercise{}
	for rows.Next() {
		var e Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.MuscleGroup); err != nil {
			return nil, err
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}
