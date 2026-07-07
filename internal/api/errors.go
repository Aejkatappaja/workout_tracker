package api

import (
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/utils"
)

// clientError writes a JSON error envelope for a 4xx the caller can act on.
func clientError(w http.ResponseWriter, status int, msg string) {
	utils.WriteJSON(w, status, utils.Envelope{"error": msg})
}

// serverError logs the underlying cause against the request and returns a generic
// 500, so internal details never reach the client.
func serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	middleware.LoggerFrom(r.Context()).Error(op, "err", err)
	utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
}
