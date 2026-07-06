package web

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/tokens"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUserStore struct {
	byName    map[string]*store.User
	createErr error
}

func (f *fakeUserStore) CreateUser(u *store.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	u.ID = 1
	f.byName[u.Username] = u
	return nil
}
func (f *fakeUserStore) GetUserByUsername(n string) (*store.User, error) { return f.byName[n], nil }
func (f *fakeUserStore) UpdateUser(*store.User) error                    { return nil }
func (f *fakeUserStore) GetUserToken(scope, t string) (*store.User, error) {
	return nil, nil
}

type fakeTokenStore struct{ deleted bool }

func (fakeTokenStore) Insert(*tokens.Token) error { return nil }
func (fakeTokenStore) CreateNewToken(userID int, ttl time.Duration, scope string) (*tokens.Token, error) {
	return &tokens.Token{PlainText: "test-token", UserID: userID, Expiry: time.Now().Add(ttl)}, nil
}
func (f *fakeTokenStore) DeleteAllTokensForUser(userID int, scope string) error {
	f.deleted = true
	return nil
}

func newTestHandler(users store.UserStore, toks store.TokenStore) *Handler {
	return NewHandler(users, toks, nil, log.New(io.Discard, "", 0))
}

func formPost(target, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// sessionCookie returns the Set-Cookie session cookie, or nil.
func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.SessionCookieName {
			return c
		}
	}
	return nil
}

func neoWithPassword(t *testing.T) *store.User {
	u := &store.User{ID: 1, Username: "neo", Email: "neo@x.io"}
	require.NoError(t, u.PasswordHash.Set("whiterabbit"))
	return u
}

func TestLogin(t *testing.T) {
	neo := neoWithPassword(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCookie bool
	}{
		{"valid credentials set a session cookie and redirect", "username=neo&password=whiterabbit", http.StatusSeeOther, true},
		{"wrong password is 401", "username=neo&password=nope", http.StatusUnauthorized, false},
		{"unknown user is 401", "username=ghost&password=whatever", http.StatusUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&fakeUserStore{byName: map[string]*store.User{"neo": neo}}, &fakeTokenStore{})
			rec := httptest.NewRecorder()

			h.Login(rec, formPost("/login", tt.body))

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusSeeOther {
				assert.Equal(t, "/app", rec.Header().Get("Location"))
			}
			c := sessionCookie(rec)
			if tt.wantCookie {
				require.NotNil(t, c)
				assert.NotEmpty(t, c.Value)
				assert.True(t, c.HttpOnly)
			} else {
				assert.Nil(t, c)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
		wantCookie bool
	}{
		{"valid registration logs in", "username=neo&email=neo@x.io&password=whiterabbit", nil, http.StatusSeeOther, true},
		{"duplicate is 409", "username=neo&email=neo@x.io&password=whiterabbit", &pgconn.PgError{Code: "23505"}, http.StatusConflict, false},
		{"short password is 400", "username=neo&email=neo@x.io&password=short", nil, http.StatusBadRequest, false},
		{"missing email is 400", "username=neo&password=whiterabbit", nil, http.StatusBadRequest, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&fakeUserStore{byName: map[string]*store.User{}, createErr: tt.createErr}, &fakeTokenStore{})
			rec := httptest.NewRecorder()

			h.Register(rec, formPost("/register", tt.body))

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantCookie, sessionCookie(rec) != nil)
		})
	}
}

func TestLogout(t *testing.T) {
	ts := &fakeTokenStore{}
	h := newTestHandler(&fakeUserStore{byName: map[string]*store.User{}}, ts)

	req := middleware.SetUser(httptest.NewRequest(http.MethodPost, "/logout", nil), &store.User{ID: 1})
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"))
	assert.True(t, ts.deleted, "logout must revoke the user's tokens")

	c := sessionCookie(rec)
	require.NotNil(t, c)
	assert.True(t, c.MaxAge < 0, "logout must clear the session cookie")
}
