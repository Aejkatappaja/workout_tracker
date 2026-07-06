// Package web serves the server-rendered HTMX UI on top of the same stores as the JSON API.
package web

import (
	"embed"
	"log"
	"net/http"

	"github.com/Aejkatappaja/workout_tracker/internal/store"
)

//go:embed static
var staticFS embed.FS

type Handler struct {
	users  store.UserStore
	tokens store.TokenStore
	logger *log.Logger
}

func NewHandler(users store.UserStore, tokenStore store.TokenStore, logger *log.Logger) *Handler {
	return &Handler{users: users, tokens: tokenStore, logger: logger}
}

// Static serves the embedded css/js under /static/.
func (h *Handler) Static() http.Handler {
	return http.FileServer(http.FS(staticFS))
}
