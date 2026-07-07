package web

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeExerciseStore struct{ got *store.Exercise }

func (f *fakeExerciseStore) Search(string, int) ([]store.Exercise, error) { return nil, nil }
func (f *fakeExerciseStore) List() ([]store.Exercise, error)              { return nil, nil }
func (f *fakeExerciseStore) Get(int) (*store.Exercise, error)             { return f.got, nil }

type fakeAnalyticsStore struct {
	records  []store.PersonalRecord
	progress []store.ProgressPoint
}

func (f *fakeAnalyticsStore) PersonalRecords(int) ([]store.PersonalRecord, error) {
	return f.records, nil
}
func (f *fakeAnalyticsStore) ExerciseProgress(int, int) ([]store.ProgressPoint, error) {
	return f.progress, nil
}

func progressHandler(ex store.ExerciseStore, an store.AnalyticsStore) *Handler {
	return NewHandler(nil, nil, nil, ex, an, log.New(io.Discard, "", 0), nil)
}

func TestProgress(t *testing.T) {
	an := &fakeAnalyticsStore{records: []store.PersonalRecord{
		{ExerciseID: 3, Exercise: "bench press", MuscleGroup: "chest", Weight: 100, Reps: 5, E1RM: 116, Day: "2026-07-01"},
	}}
	h := progressHandler(&fakeExerciseStore{}, an)

	rec := httptest.NewRecorder()
	h.Progress(rec, webReq(http.MethodGet, "/app/progress", "", &store.User{ID: 1, Username: "neo"}, ""))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "bench press")
	assert.Contains(t, body, "/app/exercises/3", "PR card links to the exercise progress page")
}

func TestExerciseProgress(t *testing.T) {
	ex := &fakeExerciseStore{got: &store.Exercise{ID: 3, Name: "bench press", MuscleGroup: "chest"}}
	an := &fakeAnalyticsStore{progress: []store.ProgressPoint{
		{Day: "2026-06-01", E1RM: 100, Volume: 1500},
		{Day: "2026-07-01", E1RM: 116, Volume: 1740},
	}}
	h := progressHandler(ex, an)

	rec := httptest.NewRecorder()
	h.ExerciseProgress(rec, webReq(http.MethodGet, "/app/exercises/3", "", &store.User{ID: 1, Username: "neo"}, "3"))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "bench press")
	assert.Contains(t, body, "chart-line", "renders the line chart")
}

func TestExerciseProgress_NotFound(t *testing.T) {
	h := progressHandler(&fakeExerciseStore{got: nil}, &fakeAnalyticsStore{})

	rec := httptest.NewRecorder()
	h.ExerciseProgress(rec, webReq(http.MethodGet, "/app/exercises/9", "", &store.User{ID: 1}, "9"))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
