// Package web serves the server-rendered HTMX UI on top of the same stores as the JSON API.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"log"
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/store"
)

//go:embed static
var staticFS embed.FS

// staticETags maps "/static/<file>" to a content hash computed once at startup.
// Files embedded with go:embed carry a zero modtime, so http.FileServer emits
// neither Last-Modified nor ETag and the browser re-downloads every asset on
// each navigation. A content ETag lets the browser cache and cheaply revalidate.
var staticETags = buildStaticETags()

func buildStaticETags() map[string]string {
	m := make(map[string]string)
	_ = fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := staticFS.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		m["/"+p] = `"` + hex.EncodeToString(sum[:16]) + `"`
		return nil
	})
	return m
}

type Handler struct {
	users    store.UserStore
	tokens   store.TokenStore
	workouts store.WorkoutStore
	logger   *log.Logger
}

func NewHandler(users store.UserStore, tokenStore store.TokenStore, workouts store.WorkoutStore, logger *log.Logger) *Handler {
	return &Handler{users: users, tokens: tokenStore, workouts: workouts, logger: logger}
}

// Static serves the embedded css/js under /static/ with caching headers.
// The ETag is honored by http.ServeContent, which answers If-None-Match with a
// 304, so a redeploy that changes an asset invalidates the cache automatically.
func (h *Handler) Static() http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if etag, ok := staticETags[r.URL.Path]; ok {
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		fileServer.ServeHTTP(w, r)
	})
}
