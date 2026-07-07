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
	if user == nil || user == store.AnonymousUser {
		utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "you must be logged in"})
		return
	}
	id, err := utils.ReadIDParam(r)
	if err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "invalid exercise id"})
		return
	}

	progress, err := h.analytics.ExerciseProgress(user.ID, int(id))
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("exercise progress", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"progress": progress})
}

// HandlePersonalRecords returns the caller's best e1RM per exercise.
func (h *AnalyticsHandler) HandlePersonalRecords(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || user == store.AnonymousUser {
		utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "you must be logged in"})
		return
	}

	records, err := h.analytics.PersonalRecords(user.ID)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("personal records", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"records": records})
}
