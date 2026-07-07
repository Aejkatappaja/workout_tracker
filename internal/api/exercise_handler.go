package api

import (
	"log"
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/utils"
)

type ExerciseHandler struct {
	exerciseStore store.ExerciseStore
	logger        *log.Logger
}

func NewExerciseHandler(exerciseStore store.ExerciseStore, logger *log.Logger) *ExerciseHandler {
	return &ExerciseHandler{exerciseStore: exerciseStore, logger: logger}
}

// HandleSearchExercises returns catalog exercises matching ?q= (prefix), for
// typeaheads and API clients. No `q` returns the first alphabetical matches.
func (h *ExerciseHandler) HandleSearchExercises(w http.ResponseWriter, r *http.Request) {
	exercises, err := h.exerciseStore.Search(r.URL.Query().Get("q"), 8)
	if err != nil {
		h.logger.Printf("ERROR: SearchExercises: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"exercises": exercises})
}
