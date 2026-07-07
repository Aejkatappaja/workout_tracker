package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLogger_NormalizesImplicitStatus(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	// a handler that only sets a header (HTMX HX-Redirect) never calls
	// WriteHeader, so net/http sends an implicit 200 the wrapper sees as 0.
	handler := chimw.RequestID(RequestLogger(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("HX-Redirect", "/app")
	})))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/app/workouts", nil))

	var line struct {
		Msg    string `json:"msg"`
		Status int    `json:"status"`
		Method string `json:"method"`
		Path   string `json:"path"`
		ReqID  string `json:"req_id"`
	}
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line))
	assert.Equal(t, "request", line.Msg)
	assert.Equal(t, http.StatusOK, line.Status, "implicit 200 is reported, not 0")
	assert.Equal(t, http.MethodPost, line.Method)
	assert.Equal(t, "/app/workouts", line.Path)
	assert.NotEmpty(t, line.ReqID, "request id is captured")
}

func TestRequestLogger_SkipsStaticAssets(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := RequestLogger(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// the context logger is still present even though we skip the access line
		assert.NotNil(t, LoggerFrom(r.Context()))
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/static/app.css", nil))

	assert.Empty(t, buf.String(), "static asset requests are not access-logged")
}

func TestLoggerFrom_FallsBackToDefault(t *testing.T) {
	assert.NotNil(t, LoggerFrom(context.Background()), "no context logger still returns a usable logger")
}
