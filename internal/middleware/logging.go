package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

type ctxKey int

const loggerKey ctxKey = iota

// LoggerFrom returns the request-scoped logger stored by RequestLogger. It falls
// back to slog.Default when called outside a request (e.g. in tests).
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// RequestLogger attaches a request-scoped slog logger carrying the request id,
// method and path to the context, then logs one access line per request with the
// final status, response size and latency. Static asset requests are not logged
// to keep the access log signal-heavy, but still get a context logger.
func RequestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l := base.With(
				slog.String("req_id", chimw.GetReqID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
			ctx := context.WithValue(r.Context(), loggerKey, l)

			if strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r.WithContext(ctx))

			l.Info("request",
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}
