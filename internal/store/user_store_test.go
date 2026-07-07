package store

import (
	"testing"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserStore_CRUDAndLookups(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	us := NewPostgresUserStore(db)

	u := &User{Username: "neo", Email: "neo@example.com", Bio: "the one"}
	require.NoError(t, u.PasswordHash.Set("whiterabbit"))
	require.NoError(t, us.CreateUser(u))
	require.NotZero(t, u.ID)

	byName, err := us.GetUserByUsername("neo")
	require.NoError(t, err)
	require.NotNil(t, byName)
	assert.Equal(t, u.ID, byName.ID)

	byEmail, err := us.GetUserByEmail("neo@example.com")
	require.NoError(t, err)
	require.NotNil(t, byEmail)
	assert.Equal(t, u.ID, byEmail.ID)

	// unknown lookups return (nil, nil), never an error
	missing, err := us.GetUserByUsername("nobody")
	require.NoError(t, err)
	assert.Nil(t, missing)

	ok, err := byName.PasswordHash.Matches("whiterabbit")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestUserStore_UpdateUserPassword(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	us := NewPostgresUserStore(db)

	u := &User{Username: "neo", Email: "neo@example.com"}
	require.NoError(t, u.PasswordHash.Set("whiterabbit"))
	require.NoError(t, us.CreateUser(u))

	require.NoError(t, us.UpdateUserPassword(u.ID, "newpass123"))

	reloaded, err := us.GetUserByUsername("neo")
	require.NoError(t, err)
	old, _ := reloaded.PasswordHash.Matches("whiterabbit")
	assert.False(t, old, "the old password no longer matches after a reset")
	fresh, _ := reloaded.PasswordHash.Matches("newpass123")
	assert.True(t, fresh, "the new password matches")
}

func TestGetUserToken(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	us := NewPostgresUserStore(db)
	ts := NewPostgresTokenStore(db)

	u := &User{Username: "trin", Email: "trin@example.com"}
	require.NoError(t, u.PasswordHash.Set("dozer"))
	require.NoError(t, us.CreateUser(u))

	tok, err := ts.CreateNewToken(u.ID, time.Hour, tokens.ScopeAuth)
	require.NoError(t, err)

	// a valid, in-scope, unexpired token resolves to its user
	got, err := us.GetUserToken(tokens.ScopeAuth, tok.PlainText)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, u.ID, got.ID)

	// the same token under a different scope does not resolve
	got, err = us.GetUserToken(tokens.ScopePasswordReset, tok.PlainText)
	require.NoError(t, err)
	assert.Nil(t, got, "lookup is scoped")

	// an expired token does not resolve
	expired, err := ts.CreateNewToken(u.ID, -time.Hour, tokens.ScopeAuth)
	require.NoError(t, err)
	got, err = us.GetUserToken(tokens.ScopeAuth, expired.PlainText)
	require.NoError(t, err)
	assert.Nil(t, got, "expired token does not resolve")

	// revoking the scope drops the token
	require.NoError(t, ts.DeleteAllTokensForUser(u.ID, tokens.ScopeAuth))
	got, err = us.GetUserToken(tokens.ScopeAuth, tok.PlainText)
	require.NoError(t, err)
	assert.Nil(t, got, "revoked token does not resolve")
}
