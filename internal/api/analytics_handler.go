package api

import (
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/utils"
)

type AnalyticsHandler struct {
	analytics store.AnalyticsStore
}

func NewAnalyticsHandler(analytics store.AnalyticsStore) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics}
}

// HandleExerciseProgress returns the caller's progression curve for one exercise.
func (h *AnalyticsHandler) HandleExerciseProgress(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	id, err := utils.ReadIDParam(r)
	if err != nil {
		clientError(w, http.StatusBadRequest, "invalid exercise id")
		return
	}

	progress, err := h.analytics.ExerciseProgress(user.ID, int(id))
	if err != nil {
		serverError(w, r, "exercise progress", err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"progress": progress})
}

// HandlePersonalRecords returns the caller's best e1RM per exercise.
func (h *AnalyticsHandler) HandlePersonalRecords(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	records, err := h.analytics.PersonalRecords(user.ID)
	if err != nil {
		serverError(w, r, "personal records", err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"records": records})
}
