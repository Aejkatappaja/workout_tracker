package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type Envelope map[string]interface{}

// DayKey formats a moment as a civil date ("YYYY-MM-DD") in UTC. Go has no
// date-only type, so keying on the UTC day keeps the counts query, the heatmap
// grid and callers in agreement regardless of the server's local timezone.
func DayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func WriteJSON(w http.ResponseWriter, status int, data Envelope) {
	js, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	js = append(js, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(js)
}

func ReadIDParam(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	if idParam == "" {
		return 0, errors.New("invalid id parameter")
	}

	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		return 0, errors.New("invalid id parameter type")
	}

	return id, nil
}
