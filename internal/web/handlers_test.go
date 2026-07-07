package web

import (
	"context"
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
	byEmail   map[string]*store.User
	tokenUser *store.User // returned by GetUserToken (reset flow)
	createErr error
	newPass   string // last plaintext passed to UpdateUserPassword
	pwUpdated bool
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
func (f *fakeUserStore) GetUserByEmail(e string) (*store.User, error)    { return f.byEmail[e], nil }
func (f *fakeUserStore) UpdateUserPassword(_ int, plaintext string) error {
	f.newPass = plaintext
	f.pwUpdated = true
	return nil
}
func (f *fakeUserStore) GetUserToken(scope, t string) (*store.User, error) {
	return f.tokenUser, nil
}

type fakeMailer struct {
	sent    bool
	to      string
	subject string
}

func (m *fakeMailer) Send(_ context.Context, to, subject, _, _ string) error {
	m.sent = true
	m.to = to
	m.subject = subject
	return nil
}

type fakeTokenStore struct {
	deleted bool
	created int
}

func (f *fakeTokenStore) CreateNewToken(userID int, ttl time.Duration, scope string) (*tokens.Token, error) {
	f.created++
	return &tokens.Token{PlainText: "test-token", UserID: userID, Expiry: time.Now().Add(ttl)}, nil
}
func (f *fakeTokenStore) DeleteAllTokensForUser(userID int, scope string) error {
	f.deleted = true
	return nil
}

func newTestHandler(users store.UserStore, toks store.TokenStore) *Handler {
	return NewHandler(users, toks, nil, nil, nil, &fakeMailer{})
}

func newTestHandlerMail(users store.UserStore, toks store.TokenStore, m *fakeMailer) *Handler {
	return NewHandler(users, toks, nil, nil, nil, m)
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

func TestForgot(t *testing.T) {
	neo := &store.User{ID: 1, Username: "neo", Email: "neo@x.io"}
	tests := []struct {
		name        string
		email       string
		known       bool
		wantCreated int
	}{
		{"known email creates a reset token", "neo@x.io", true, 1},
		{"unknown email creates nothing", "ghost@x.io", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			byEmail := map[string]*store.User{}
			if tt.known {
				byEmail["neo@x.io"] = neo
			}
			ts := &fakeTokenStore{}
			h := newTestHandlerMail(&fakeUserStore{byEmail: byEmail}, ts, &fakeMailer{})
			rec := httptest.NewRecorder()

			h.Forgot(rec, formPost("/forgot", "email="+tt.email))

			// identical response either way (no account enumeration)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantCreated, ts.created)
		})
	}
}

func TestReset(t *testing.T) {
	neo := &store.User{ID: 1, Username: "neo", Email: "neo@x.io"}

	t.Run("valid token sets the password and revokes tokens", func(t *testing.T) {
		us := &fakeUserStore{tokenUser: neo}
		ts := &fakeTokenStore{}
		h := newTestHandler(us, ts)
		rec := httptest.NewRecorder()

		h.Reset(rec, formPost("/reset", "token=abc&password=newpass1234"))

		assert.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/login", rec.Header().Get("Location"))
		assert.True(t, us.pwUpdated, "password must be updated")
		assert.Equal(t, "newpass1234", us.newPass)
		assert.True(t, ts.deleted, "tokens must be revoked")
	})

	t.Run("invalid token is 400 and changes nothing", func(t *testing.T) {
		us := &fakeUserStore{tokenUser: nil}
		h := newTestHandler(us, &fakeTokenStore{})
		rec := httptest.NewRecorder()

		h.Reset(rec, formPost("/reset", "token=bad&password=newpass1234"))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, us.pwUpdated)
	})

	t.Run("short password is 400", func(t *testing.T) {
		us := &fakeUserStore{tokenUser: neo}
		h := newTestHandler(us, &fakeTokenStore{})
		rec := httptest.NewRecorder()

		h.Reset(rec, formPost("/reset", "token=abc&password=short"))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, us.pwUpdated)
	})
}
