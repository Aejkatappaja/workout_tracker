package api

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/tokens"
	"github.com/go-chi/chi/v5"
)

func discardLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// authedRequest builds a request with the chi "id" URL param and the given user
// injected into the context, mirroring what the router + Authenticate middleware
// do in production.
func authedRequest(method, target string, body io.Reader, id string, user *store.User) *http.Request {
	req := httptest.NewRequest(method, target, body)

	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}

	if user != nil {
		req = middleware.SetUser(req, user)
	}

	return req
}

type fakeWorkoutStore struct {
	workouts map[int64]*store.Workout
	nextID   int
}

func newFakeWorkoutStore() *fakeWorkoutStore {
	return &fakeWorkoutStore{workouts: map[int64]*store.Workout{}, nextID: 1}
}

func (f *fakeWorkoutStore) CreateWorkout(w *store.Workout) (*store.Workout, error) {
	w.ID = f.nextID
	f.nextID++
	stored := *w
	f.workouts[int64(w.ID)] = &stored
	return w, nil
}

func (f *fakeWorkoutStore) GetWorkoutByID(id int64) (*store.Workout, error) {
	w, ok := f.workouts[id]
	if !ok {
		return nil, nil
	}
	// return a copy so handler mutations don't leak before authorization,
	// matching how the real store returns a fresh struct per query.
	cp := *w
	return &cp, nil
}

func (f *fakeWorkoutStore) UpdateWorkout(w *store.Workout) error {
	stored := *w
	f.workouts[int64(w.ID)] = &stored
	return nil
}

func (f *fakeWorkoutStore) DeleteWorkoutByID(id int64, userID int) error {
	delete(f.workouts, id)
	return nil
}

func (f *fakeWorkoutStore) ListWorkoutsByUser(userID int) ([]store.Workout, error) {
	out := []store.Workout{}
	for _, w := range f.workouts {
		if w.UserID == userID {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (f *fakeWorkoutStore) WorkoutCountsByDay(userID int, since time.Time) (map[string]int, error) {
	return map[string]int{}, nil
}

func (f *fakeWorkoutStore) GetWorkoutOwner(id int64) (int, error) {
	w, ok := f.workouts[id]
	if !ok {
		return 0, nil
	}
	return w.UserID, nil
}

type fakeUserStore struct {
	users     map[string]*store.User
	createErr error
}

func (f *fakeUserStore) CreateUser(u *store.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.users[u.Username] = u
	return nil
}

func (f *fakeUserStore) GetUserByUsername(username string) (*store.User, error) {
	return f.users[username], nil
}

func (f *fakeUserStore) UpdateUser(*store.User) error { return nil }

func (f *fakeUserStore) GetUserToken(scope, tokenPlainText string) (*store.User, error) {
	return nil, nil
}

type fakeTokenStore struct{}

func (fakeTokenStore) Insert(*tokens.Token) error { return nil }

func (fakeTokenStore) CreateNewToken(userID int, ttl time.Duration, scope string) (*tokens.Token, error) {
	return &tokens.Token{PlainText: "test-token", UserID: userID}, nil
}

func (fakeTokenStore) DeleteAllTokensForUser(userID int, scope string) error { return nil }
