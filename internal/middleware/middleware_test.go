package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/stretchr/testify/assert"
)

type fakeUserStore struct {
	user *store.User
	err  error
}

func (f *fakeUserStore) CreateUser(*store.User) error                  { return nil }
func (f *fakeUserStore) GetUserByUsername(string) (*store.User, error) { return nil, nil }
func (f *fakeUserStore) GetUserByEmail(string) (*store.User, error)    { return nil, nil }
func (f *fakeUserStore) UpdateUserPassword(int, string) error          { return nil }
func (f *fakeUserStore) GetUserToken(scope, tokenPlainText string) (*store.User, error) {
	return f.user, f.err
}

func TestAuthenticate(t *testing.T) {
	realUser := &store.User{ID: 1, Username: "neo"}

	tests := []struct {
		name         string
		authHeader   string
		store        *fakeUserStore
		wantStatus   int
		wantNextUser *store.User // user seen by next handler; nil means next must NOT run
	}{
		{"no header falls back to anonymous", "", &fakeUserStore{}, http.StatusOK, store.AnonymousUser},
		{"wrong scheme is 401", "Token abc", &fakeUserStore{}, http.StatusUnauthorized, nil},
		{"single-part header is 401", "Bearer", &fakeUserStore{}, http.StatusUnauthorized, nil},
		{"lookup error is 401", "Bearer sometoken", &fakeUserStore{err: errors.New("db down")}, http.StatusUnauthorized, nil},
		{"expired or unknown token is 401", "Bearer sometoken", &fakeUserStore{user: nil}, http.StatusUnauthorized, nil},
		{"valid token injects the user", "Bearer sometoken", &fakeUserStore{user: realUser}, http.StatusOK, realUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			um := &UserMiddleware{UserStore: tt.store}

			var gotUser *store.User
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotUser = GetUser(r)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			um.Authenticate(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantNextUser != nil {
				assert.True(t, nextCalled, "next must run on success")
				assert.Equal(t, tt.wantNextUser, gotUser)
			} else {
				assert.False(t, nextCalled, "next must not run on auth failure")
			}
		})
	}
}

func TestAuthenticate_Cookie(t *testing.T) {
	realUser := &store.User{ID: 1, Username: "neo"}

	tests := []struct {
		name         string
		store        *fakeUserStore
		wantNextUser *store.User
	}{
		{"valid cookie authenticates", &fakeUserStore{user: realUser}, realUser},
		{"stale cookie falls back to anonymous", &fakeUserStore{user: nil}, store.AnonymousUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			um := &UserMiddleware{UserStore: tt.store}

			var gotUser *store.User
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotUser = GetUser(r)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "sometoken"})
			rec := httptest.NewRecorder()

			um.Authenticate(next).ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.True(t, nextCalled, "next must run (stale cookie is anonymous, not a hard fail)")
			assert.Equal(t, tt.wantNextUser, gotUser)
		})
	}
}

func TestRequireUserWeb(t *testing.T) {
	um := &UserMiddleware{}

	tests := []struct {
		name       string
		user       *store.User
		wantStatus int
		wantNext   bool
	}{
		{"anonymous redirects to login", store.AnonymousUser, http.StatusSeeOther, false},
		{"authenticated user passes", &store.User{ID: 1}, http.StatusOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := SetUser(httptest.NewRequest(http.MethodGet, "/", nil), tt.user)
			rec := httptest.NewRecorder()

			um.RequireUserWeb(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantNext, nextCalled)
			if !tt.wantNext {
				assert.Equal(t, "/login", rec.Header().Get("Location"))
			}
		})
	}
}

func TestRequireUser(t *testing.T) {
	um := &UserMiddleware{}

	tests := []struct {
		name       string
		user       *store.User
		wantStatus int
		wantNext   bool
	}{
		{"anonymous is rejected", store.AnonymousUser, http.StatusUnauthorized, false},
		{"authenticated user passes", &store.User{ID: 1}, http.StatusOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := SetUser(httptest.NewRequest(http.MethodGet, "/", nil), tt.user)
			rec := httptest.NewRecorder()

			um.RequireUser(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantNext, nextCalled)
		})
	}
}
