package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/utils"
)

type WorkoutHandler struct {
	workoutStore store.WorkoutStore
}

func NewWorkoutHandler(workoutStore store.WorkoutStore) *WorkoutHandler {
	return &WorkoutHandler{workoutStore: workoutStore}
}

func (wh *WorkoutHandler) HandleListWorkouts(w http.ResponseWriter, r *http.Request) {
	currentUser := middleware.GetUser(r)
	workouts, err := wh.workoutStore.ListWorkoutsByUser(currentUser.ID)
	if err != nil {
		serverError(w, r, "list workouts", err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"workouts": workouts})
}

func (wh *WorkoutHandler) HandleGetWorkoutByID(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIDParam(r)
	if err != nil {
		clientError(w, http.StatusBadRequest, "invalid workout id")
		return
	}

	workout, err := wh.workoutStore.GetWorkoutByID(workoutID)
	if err != nil {
		serverError(w, r, "get workout by id", err)
		return
	}

	if workout == nil {
		clientError(w, http.StatusNotFound, "workout does not exist")
		return
	}

	currentUser := middleware.GetUser(r)
	if workout.UserID != currentUser.ID {
		clientError(w, http.StatusForbidden, "you are not authorized to view that workout")
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"workout": workout})
}

func (wh *WorkoutHandler) HandleCreateWorkout(w http.ResponseWriter, r *http.Request) {
	var workout store.Workout
	if err := json.NewDecoder(r.Body).Decode(&workout); err != nil {
		clientError(w, http.StatusBadRequest, "invalid request sent")
		return
	}

	currentUser := middleware.GetUser(r)
	workout.UserID = currentUser.ID

	createdWorkout, err := wh.workoutStore.CreateWorkout(&workout)
	if err != nil {
		serverError(w, r, "create workout", err)
		return
	}
	utils.WriteJSON(w, http.StatusCreated, utils.Envelope{"workout": createdWorkout})
}

func (wh *WorkoutHandler) HandleUpdatedWorkoutByID(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIDParam(r)
	if err != nil {
		clientError(w, http.StatusBadRequest, "invalid workout update id")
		return
	}

	existingWorkout, err := wh.workoutStore.GetWorkoutByID(workoutID)
	if err != nil {
		serverError(w, r, "get workout by id", err)
		return
	}
	if existingWorkout == nil {
		clientError(w, http.StatusNotFound, "workout does not exist")
		return
	}

	// GetWorkoutByID already loaded the owner, so check it here rather than
	// making a second GetWorkoutOwner round-trip, and before touching the body.
	currentUser := middleware.GetUser(r)
	if existingWorkout.UserID != currentUser.ID {
		clientError(w, http.StatusForbidden, "you are not authorized to update that workout")
		return
	}

	var updateWorkoutRequest struct {
		Title           *string              `json:"title"`
		Description     *string              `json:"description"`
		DurationMinutes *int                 `json:"duration_minutes"`
		CaloriesBurned  *int                 `json:"calories_burned"`
		Entries         []store.WorkoutEntry `json:"entries"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateWorkoutRequest); err != nil {
		clientError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if updateWorkoutRequest.Title != nil {
		existingWorkout.Title = *updateWorkoutRequest.Title
	}

	if updateWorkoutRequest.Description != nil {
		existingWorkout.Description = *updateWorkoutRequest.Description
	}

	if updateWorkoutRequest.DurationMinutes != nil {
		existingWorkout.DurationMinutes = *updateWorkoutRequest.DurationMinutes
	}

	if updateWorkoutRequest.CaloriesBurned != nil {
		existingWorkout.CaloriesBurned = *updateWorkoutRequest.CaloriesBurned
	}

	if updateWorkoutRequest.Entries != nil {
		existingWorkout.Entries = updateWorkoutRequest.Entries
	}

	if err := wh.workoutStore.UpdateWorkout(existingWorkout); err != nil {
		serverError(w, r, "update workout", err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"workout": existingWorkout})
}

func (wh *WorkoutHandler) DeleteWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIDParam(r)
	if err != nil {
		clientError(w, http.StatusBadRequest, "invalid workout id")
		return
	}

	currentUser := middleware.GetUser(r)

	workoutOwner, err := wh.workoutStore.GetWorkoutOwner(workoutID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			clientError(w, http.StatusNotFound, "workout does not exist")
			return
		}
		serverError(w, r, "get workout owner", err)
		return
	}

	if workoutOwner != currentUser.ID {
		clientError(w, http.StatusForbidden, "you are not authorized to delete that workout")
		return
	}

	err = wh.workoutStore.DeleteWorkoutByID(workoutID, currentUser.ID)
	if errors.Is(err, sql.ErrNoRows) {
		clientError(w, http.StatusNotFound, "workout does not exist")
		return
	}
	if err != nil {
		serverError(w, r, "delete workout", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
