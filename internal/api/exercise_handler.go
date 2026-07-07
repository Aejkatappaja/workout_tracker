package api

import (
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/utils"
)

type ExerciseHandler struct {
	exerciseStore store.ExerciseStore
}

func NewExerciseHandler(exerciseStore store.ExerciseStore) *ExerciseHandler {
	return &ExerciseHandler{exerciseStore: exerciseStore}
}

// HandleSearchExercises returns catalog exercises matching ?q= (prefix), for
// typeaheads and API clients. No `q` returns the first alphabetical matches.
func (h *ExerciseHandler) HandleSearchExercises(w http.ResponseWriter, r *http.Request) {
	exercises, err := h.exerciseStore.Search(r.URL.Query().Get("q"), 8)
	if err != nil {
		serverError(w, r, "search exercises", err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"exercises": exercises})
}
