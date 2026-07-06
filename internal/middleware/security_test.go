package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'")
	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"), "no HSTS over plain HTTP")

	// HSTS only once the request is HTTPS (direct or via proxy)
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Forwarded-Proto", "https")
	h.ServeHTTP(rec2, req2)
	assert.NotEmpty(t, rec2.Header().Get("Strict-Transport-Security"))
}

func TestBodyLimit(t *testing.T) {
	read := func(limit int64, body string) error {
		var err error
		BodyLimit(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err = io.ReadAll(r.Body)
		})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		return err
	}

	assert.Error(t, read(10, strings.Repeat("a", 100)), "body over the limit must error")
	assert.NoError(t, read(10, "ok"), "body under the limit is fine")
}
