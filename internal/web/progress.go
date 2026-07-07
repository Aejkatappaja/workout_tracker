package web

import (
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/utils"
	"github.com/Aejkatappaja/go-gym/internal/web/views"
)

// Progress lists the user's personal records (best e1RM per exercise).
func (h *Handler) Progress(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	records, err := h.analytics.PersonalRecords(user.ID)
	if err != nil {
		h.logger.Printf("ERROR: web PersonalRecords: %v", err)
		records = nil
	}
	h.render(w, r, http.StatusOK, views.ProgressPage(user.Username, records, h.readOnly(r)))
}

// ExerciseProgress shows one exercise's e1RM progression line chart.
func (h *Handler) ExerciseProgress(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	id, err := utils.ReadIDParam(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ex, err := h.exercises.Get(int(id))
	if err != nil {
		h.logger.Printf("ERROR: web exercise get: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if ex == nil {
		http.NotFound(w, r)
		return
	}

	points, err := h.analytics.ExerciseProgress(user.ID, int(id))
	if err != nil {
		h.logger.Printf("ERROR: web ExerciseProgress: %v", err)
		points = nil
	}

	h.render(w, r, http.StatusOK, views.ExerciseProgressPage(user.Username, ex.Name, views.BuildLineChart(points), h.readOnly(r)))
}
