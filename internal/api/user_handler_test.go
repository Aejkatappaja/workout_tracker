package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestHandleRegisterUser(t *testing.T) {
	const validBody = `{"username":"neo","email":"neo@matrix.io","password":"whiterabbit"}`

	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
	}{
		{"valid registration", validBody, nil, http.StatusCreated},
		{"duplicate username or email", validBody, &pgconn.PgError{Code: "23505"}, http.StatusConflict},
		{"password too short", `{"username":"neo","email":"neo@matrix.io","password":"short"}`, nil, http.StatusBadRequest},
		{"invalid email", `{"username":"neo","email":"not-an-email","password":"whiterabbit"}`, nil, http.StatusBadRequest},
		{"missing username", `{"email":"neo@matrix.io","password":"whiterabbit"}`, nil, http.StatusBadRequest},
		{"malformed json", `{`, nil, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			us := &fakeUserStore{users: map[string]*store.User{}, createErr: tt.createErr}
			h := NewUserHandler(us)

			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandleRegisterUser(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
