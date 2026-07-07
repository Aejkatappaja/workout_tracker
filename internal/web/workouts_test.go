package web

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkoutStore struct {
	byID      map[int64]*store.Workout
	created   *store.Workout
	deletedID int64
}

func (f *fakeWorkoutStore) CreateWorkout(wk *store.Workout) (*store.Workout, error) {
	wk.ID = 1
	if f.byID == nil {
		f.byID = map[int64]*store.Workout{}
	}
	f.byID[1] = wk
	f.created = wk
	return wk, nil
}
func (f *fakeWorkoutStore) GetWorkoutByID(id int64) (*store.Workout, error) { return f.byID[id], nil }
func (f *fakeWorkoutStore) UpdateWorkout(*store.Workout) error              { return nil }
func (f *fakeWorkoutStore) DeleteWorkoutByID(id int64, userID int) error {
	f.deletedID = id
	return nil
}
func (f *fakeWorkoutStore) WorkoutCountsByDay(userID int, since time.Time) (map[string]int, error) {
	return map[string]int{}, nil
}
func (f *fakeWorkoutStore) GetWorkoutOwner(id int64) (int, error) {
	if wk := f.byID[id]; wk != nil {
		return wk.UserID, nil
	}
	return 0, nil
}
func (f *fakeWorkoutStore) ListWorkoutsByUser(userID int) ([]store.Workout, error) {
	out := []store.Workout{}
	for _, wk := range f.byID {
		if wk.UserID == userID {
			out = append(out, *wk)
		}
	}
	return out, nil
}

func handlerWith(ws store.WorkoutStore) *Handler {
	return NewHandler(nil, nil, ws, nil, log.New(io.Discard, "", 0), nil)
}

// webReq builds a request with an optional form body, chi {id} param and user context.
func webReq(method, target, body string, user *store.User, id string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	if user != nil {
		r = middleware.SetUser(r, user)
	}
	return r
}

func TestDashboard(t *testing.T) {
	owner := &store.User{ID: 1, Username: "neo"}
	ws := &fakeWorkoutStore{byID: map[int64]*store.Workout{
		1: {ID: 1, UserID: 1, Title: "push day", DurationMinutes: 60},
		2: {ID: 2, UserID: 2, Title: "not mine", DurationMinutes: 30},
	}}
	h := handlerWith(ws)

	rec := httptest.NewRecorder()
	h.Dashboard(rec, webReq(http.MethodGet, "/app", "", owner, ""))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "push day")
	assert.NotContains(t, body, "not mine")
}

func TestWebCreate(t *testing.T) {
	owner := &store.User{ID: 7, Username: "neo"}
	ws := &fakeWorkoutStore{}
	h := handlerWith(ws)

	body := "title=push+day&duration_minutes=60&entry_exercise=bench&entry_sets=3&entry_reps=10&entry_duration=&entry_weight=135.5&entry_notes=warmup"
	rec := httptest.NewRecorder()
	h.Create(rec, webReq(http.MethodPost, "/app/workouts", body, owner, ""))

	assert.Equal(t, "/app/workouts/1", rec.Header().Get("HX-Redirect"))
	require.NotNil(t, ws.created)
	assert.Equal(t, owner.ID, ws.created.UserID, "workout must be owned by the authed user")
	require.Len(t, ws.created.Entries, 1)
	assert.Equal(t, "bench", ws.created.Entries[0].ExerciseName)
	require.NotNil(t, ws.created.Entries[0].Reps)
	assert.Equal(t, 10, *ws.created.Entries[0].Reps)
	assert.Nil(t, ws.created.Entries[0].DurationSeconds, "blank duration must be nil")
}

func TestWebDetail_NotOwned(t *testing.T) {
	owner := &store.User{ID: 1}
	ws := &fakeWorkoutStore{byID: map[int64]*store.Workout{
		5: {ID: 5, UserID: 2, Title: "someone else's"},
	}}
	h := handlerWith(ws)

	rec := httptest.NewRecorder()
	h.Detail(rec, webReq(http.MethodGet, "/app/workouts/5", "", owner, "5"))

	assert.Equal(t, http.StatusNotFound, rec.Code, "another user's workout must not be viewable")
	assert.NotContains(t, rec.Body.String(), "someone else's")
}

func TestReadOnlyDemoBlocksWrite(t *testing.T) {
	ws := &fakeWorkoutStore{}
	h := handlerWith(ws)
	demo := &store.User{ID: 1, Username: store.DemoUsername}

	rec := httptest.NewRecorder()
	body := "title=x&duration_minutes=30&entry_exercise=a&entry_sets=3&entry_reps=5&entry_duration="
	h.Create(rec, webReq(http.MethodPost, "/app/workouts", body, demo, ""))

	assert.Equal(t, "/app", rec.Header().Get("HX-Redirect"), "demo write bounces to dashboard")
	assert.Nil(t, ws.created, "demo must not create workouts")
}

func TestWebDelete(t *testing.T) {
	owner := &store.User{ID: 1}
	ws := &fakeWorkoutStore{byID: map[int64]*store.Workout{
		5: {ID: 5, UserID: 1, Title: "mine"},
	}}
	h := handlerWith(ws)

	rec := httptest.NewRecorder()
	h.Delete(rec, webReq(http.MethodDelete, "/app/workouts/5", "", owner, "5"))

	assert.Equal(t, "/app", rec.Header().Get("HX-Redirect"))
	assert.Equal(t, int64(5), ws.deletedID)

	// deleting another user's workout is a no-op 404
	other := &store.User{ID: 99}
	ws2 := &fakeWorkoutStore{byID: map[int64]*store.Workout{5: {ID: 5, UserID: 1}}}
	h2 := handlerWith(ws2)
	rec2 := httptest.NewRecorder()
	h2.Delete(rec2, webReq(http.MethodDelete, "/app/workouts/5", "", other, "5"))
	assert.Equal(t, http.StatusNotFound, rec2.Code)
	assert.Equal(t, int64(0), ws2.deletedID, "must not delete another user's workout")
}
