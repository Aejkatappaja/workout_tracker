package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Aejkatappaja/workout_tracker/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGetWorkoutByID_Authorization(t *testing.T) {
	owner := &store.User{ID: 1}
	other := &store.User{ID: 2}

	fs := newFakeWorkoutStore()
	fs.workouts[5] = &store.Workout{ID: 5, UserID: owner.ID, Title: "push day"}

	h := NewWorkoutHandler(fs, discardLogger())

	tests := []struct {
		name       string
		id         string
		user       *store.User
		wantStatus int
	}{
		{"owner reads own workout", "5", owner, http.StatusOK},
		{"other user is forbidden (IDOR)", "5", other, http.StatusForbidden},
		{"missing workout is 404", "999", owner, http.StatusNotFound},
		{"non-numeric id is 400", "abc", owner, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authedRequest(http.MethodGet, "/workouts/"+tt.id, nil, tt.id, tt.user)
			rec := httptest.NewRecorder()

			h.HandleGetWorkoutByID(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestHandleListWorkouts_OnlyOwn(t *testing.T) {
	owner := &store.User{ID: 1}
	other := &store.User{ID: 2}

	fs := newFakeWorkoutStore()
	fs.workouts[1] = &store.Workout{ID: 1, UserID: owner.ID, Title: "mine a"}
	fs.workouts[2] = &store.Workout{ID: 2, UserID: owner.ID, Title: "mine b"}
	fs.workouts[3] = &store.Workout{ID: 3, UserID: other.ID, Title: "not mine"}

	h := NewWorkoutHandler(fs, discardLogger())

	req := authedRequest(http.MethodGet, "/workouts", nil, "", owner)
	rec := httptest.NewRecorder()
	h.HandleListWorkouts(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Workouts []store.Workout `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Workouts, 2)
	for _, w := range body.Workouts {
		assert.Equal(t, owner.ID, w.UserID)
	}
}

func TestHandleCreateWorkout_OverridesClientUserID(t *testing.T) {
	user := &store.User{ID: 7}
	fs := newFakeWorkoutStore()
	h := NewWorkoutHandler(fs, discardLogger())

	// client tries to smuggle a foreign user_id in the body
	body := `{"title":"leg day","duration_minutes":45,"user_id":999}`
	req := authedRequest(http.MethodPost, "/workouts", strings.NewReader(body), "", user)
	rec := httptest.NewRecorder()

	h.HandleCreateWorkout(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, fs.workouts, 1)
	for _, w := range fs.workouts {
		assert.Equal(t, user.ID, w.UserID, "workout must be owned by the authenticated user, not the client-supplied id")
	}
}

func TestHandleUpdatedWorkoutByID_Authorization(t *testing.T) {
	owner := &store.User{ID: 1}
	other := &store.User{ID: 2}

	fs := newFakeWorkoutStore()
	fs.workouts[5] = &store.Workout{ID: 5, UserID: owner.ID, Title: "push day"}

	h := NewWorkoutHandler(fs, discardLogger())

	// another user cannot update someone else's workout
	req := authedRequest(http.MethodPut, "/workouts/5", strings.NewReader(`{"title":"hacked"}`), "5", other)
	rec := httptest.NewRecorder()
	h.HandleUpdatedWorkoutByID(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "push day", fs.workouts[5].Title, "workout must not be mutated by an unauthorized user")

	// non-numeric id is a client error, not a 500
	req = authedRequest(http.MethodPut, "/workouts/abc", strings.NewReader(`{"title":"x"}`), "abc", owner)
	rec = httptest.NewRecorder()
	h.HandleUpdatedWorkoutByID(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteWorkout_Authorization(t *testing.T) {
	owner := &store.User{ID: 1}
	other := &store.User{ID: 2}

	fs := newFakeWorkoutStore()
	fs.workouts[5] = &store.Workout{ID: 5, UserID: owner.ID}

	h := NewWorkoutHandler(fs, discardLogger())

	// other user forbidden, workout survives
	req := authedRequest(http.MethodDelete, "/workouts/5", nil, "5", other)
	rec := httptest.NewRecorder()
	h.DeleteWorkout(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, fs.workouts, int64(5))

	// owner deletes, gets 204 no content
	req = authedRequest(http.MethodDelete, "/workouts/5", nil, "5", owner)
	rec = httptest.NewRecorder()
	h.DeleteWorkout(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String(), "204 response must have no body")
	assert.NotContains(t, fs.workouts, int64(5))
}
