package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCreateToken(t *testing.T) {
	valid := &store.User{ID: 1, Username: "neo"}
	require.NoError(t, valid.PasswordHash.Set("whiterabbit"))

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"valid credentials issue a token", `{"username":"neo","password":"whiterabbit"}`, http.StatusCreated},
		{"wrong password is 401", `{"username":"neo","password":"wrong-password"}`, http.StatusUnauthorized},
		{"unknown username is 401 not 500", `{"username":"ghost","password":"whatever"}`, http.StatusUnauthorized},
		{"malformed json is 400", `{`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			us := &fakeUserStore{users: map[string]*store.User{"neo": valid}}
			h := NewTokenHandler(fakeTokenStore{}, us)

			req := httptest.NewRequest(http.MethodPost, "/tokens/authentication", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandleCreateToken(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
