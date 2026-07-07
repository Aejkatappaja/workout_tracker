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

type fakeAnalyticsStore struct {
	progress []store.ProgressPoint
	records  []store.PersonalRecord
	err      error
}

func (f fakeAnalyticsStore) ExerciseProgress(int, int) ([]store.ProgressPoint, error) {
	return f.progress, f.err
}
func (f fakeAnalyticsStore) PersonalRecords(int) ([]store.PersonalRecord, error) {
	return f.records, f.err
}
func (f fakeAnalyticsStore) WeeklyVolume(int, int) ([]store.VolumePoint, error) {
	return nil, f.err
}

func TestHandleExerciseProgress(t *testing.T) {
	user := &store.User{ID: 1}
	tests := []struct {
		name       string
		id         string
		store      fakeAnalyticsStore
		wantStatus int
	}{
		{"happy path", "3", fakeAnalyticsStore{progress: []store.ProgressPoint{{Day: "2026-07-01", E1RM: 100}}}, http.StatusOK},
		{"non-numeric id is 400", "abc", fakeAnalyticsStore{}, http.StatusBadRequest},
		{"store error is 500", "3", fakeAnalyticsStore{err: errors.New("db down")}, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAnalyticsHandler(tt.store)
			rec := httptest.NewRecorder()
			h.HandleExerciseProgress(rec, authedRequest(http.MethodGet, "/exercises/"+tt.id+"/progress", nil, tt.id, user))
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestHandlePersonalRecords(t *testing.T) {
	user := &store.User{ID: 1}

	h := NewAnalyticsHandler(fakeAnalyticsStore{records: []store.PersonalRecord{{Exercise: "bench press", E1RM: 116}}})
	rec := httptest.NewRecorder()
	h.HandlePersonalRecords(rec, authedRequest(http.MethodGet, "/records", nil, "", user))

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Records []store.PersonalRecord `json:"records"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Records, 1)
	assert.Equal(t, "bench press", body.Records[0].Exercise)
}

func TestHandlePersonalRecords_StoreError(t *testing.T) {
	h := NewAnalyticsHandler(fakeAnalyticsStore{err: errors.New("db down")})
	rec := httptest.NewRecorder()
	h.HandlePersonalRecords(rec, authedRequest(http.MethodGet, "/records", nil, "", &store.User{ID: 1}))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
