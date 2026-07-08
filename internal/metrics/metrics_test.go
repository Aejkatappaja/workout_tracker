package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func routerWith(m *Metrics) *chi.Mux {
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/x/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/boom", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	return r
}

func serve(r http.Handler, method, target string) {
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, target, nil))
}

func TestMiddleware_RecordsRequest(t *testing.T) {
	m := New()
	serve(routerWith(m), http.MethodGet, "/x/42")
	assert.Equal(t, 1.0, testutil.ToFloat64(m.requests.WithLabelValues(http.MethodGet, "/x/{id}", "200")))
}

func TestMiddleware_BoundedCardinality(t *testing.T) {
	m := New()
	r := routerWith(m)
	serve(r, http.MethodGet, "/x/1")
	serve(r, http.MethodGet, "/x/2")

	// distinct ids collapse to a single {id} series (label is the route pattern)
	assert.Equal(t, 2.0, testutil.ToFloat64(m.requests.WithLabelValues(http.MethodGet, "/x/{id}", "200")))
	assert.Equal(t, 1, testutil.CollectAndCount(m.requests), "one series, not one per id")
}

func TestMiddleware_CountsErrors(t *testing.T) {
	m := New()
	serve(routerWith(m), http.MethodGet, "/boom")
	assert.Equal(t, 1.0, testutil.ToFloat64(m.requests.WithLabelValues(http.MethodGet, "/boom", "500")))
}

func TestHandler_ExposesMetrics(t *testing.T) {
	m := New()
	serve(routerWith(m), http.MethodGet, "/x/1")

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "go_goroutines", "Go runtime collector is registered")
}
