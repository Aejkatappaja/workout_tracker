package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeExerciseStore struct {
	results []store.Exercise
	err     error
}

func (f fakeExerciseStore) Search(string, int) ([]store.Exercise, error) { return f.results, f.err }
func (f fakeExerciseStore) List() ([]store.Exercise, error)              { return f.results, f.err }
func (f fakeExerciseStore) Get(int) (*store.Exercise, error)             { return nil, f.err }

func TestHandleSearchExercises(t *testing.T) {
	h := NewExerciseHandler(fakeExerciseStore{results: []store.Exercise{{ID: 1, Name: "bench press", MuscleGroup: "chest"}}})
	rec := httptest.NewRecorder()
	h.HandleSearchExercises(rec, httptest.NewRequest(http.MethodGet, "/exercises?q=ben", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Exercises []store.Exercise `json:"exercises"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Exercises, 1)
	assert.Equal(t, "bench press", body.Exercises[0].Name)
}

func TestHandleSearchExercises_StoreError(t *testing.T) {
	h := NewExerciseHandler(fakeExerciseStore{err: errors.New("db down")})
	rec := httptest.NewRecorder()
	h.HandleSearchExercises(rec, httptest.NewRequest(http.MethodGet, "/exercises?q=x", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
