// Package web serves the server-rendered HTMX UI on top of the same stores as the JSON API.
package web

import (
	"embed"
	"log"
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/store"
)

//go:embed static
var staticFS embed.FS

type Handler struct {
	users    store.UserStore
	tokens   store.TokenStore
	workouts store.WorkoutStore
	logger   *log.Logger
}

func NewHandler(users store.UserStore, tokenStore store.TokenStore, workouts store.WorkoutStore, logger *log.Logger) *Handler {
	return &Handler{users: users, tokens: tokenStore, workouts: workouts, logger: logger}
}

// Static serves the embedded css/js under /static/.
func (h *Handler) Static() http.Handler {
	return http.FileServer(http.FS(staticFS))
}
