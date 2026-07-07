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
	if currentUser == nil || currentUser == store.AnonymousUser {
		utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "you must be logged in"})
		return
	}

	workouts, err := wh.workoutStore.ListWorkoutsByUser(currentUser.ID)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("list workouts", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"workouts": workouts})
}

func (wh *WorkoutHandler) HandleGetWorkoutByID(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIDParam(r)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("read id param", "err", err)
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "invalid workout id"})
		return
	}

	workout, err := wh.workoutStore.GetWorkoutByID(workoutID)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("get workout by id", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if workout == nil {
		utils.WriteJSON(w, http.StatusNotFound, utils.Envelope{"error": "workout does not exist"})
		return
	}

	currentUser := middleware.GetUser(r)
	if currentUser == nil || currentUser == store.AnonymousUser {
		utils.WriteJSON(w, http.StatusUnauthorized, utils.Envelope{"error": "you must be logged in"})
		return
	}

	if workout.UserID != currentUser.ID {
		utils.WriteJSON(w, http.StatusForbidden, utils.Envelope{"error": "you are not authorized to view that workout"})
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"workout": workout})
}

func (wh *WorkoutHandler) HandleCreateWorkout(w http.ResponseWriter, r *http.Request) {
	var workout store.Workout
	err := json.NewDecoder(r.Body).Decode(&workout)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("decode workout body", "err", err)
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request sent"})

		return
	}

	currentUser := middleware.GetUser(r)
	if currentUser == nil || currentUser == store.AnonymousUser {
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "you must be logged in"})
		return
	}

	workout.UserID = currentUser.ID

	createdWorkout, err := wh.workoutStore.CreateWorkout(&workout)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("create workout", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "failed to create workout"})
		return
	}
	utils.WriteJSON(w, http.StatusCreated, utils.Envelope{"workout": createdWorkout})
}

func (wh *WorkoutHandler) HandleUpdatedWorkoutByID(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIDParam(r)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("read id param", "err", err)
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "invalid workout update id"})
		return
	}

	existingWorkout, err := wh.workoutStore.GetWorkoutByID(workoutID)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("get workout by id", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if existingWorkout == nil {
		http.NotFound(w, r)
		return
	}

	var updateWorkoutRequest struct {
		Title           *string              `json:"title"`
		Description     *string              `json:"description"`
		DurationMinutes *int                 `json:"duration_minutes"`
		CaloriesBurned  *int                 `json:"calories_burned"`
		Entries         []store.WorkoutEntry `json:"entries"`
	}

	err = json.NewDecoder(r.Body).Decode(&updateWorkoutRequest)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("decode workout body", "err", err)
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
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

	currentUser := middleware.GetUser(r)
	if currentUser == nil || currentUser == store.AnonymousUser {
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "you must be logged in"})
		return
	}

	workoutOwner, err := wh.workoutStore.GetWorkoutOwner(int64(workoutID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJSON(w, http.StatusNotFound, utils.Envelope{"error": "workout does not exist"})
			return
		}

		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if workoutOwner != currentUser.ID {

		utils.WriteJSON(w, http.StatusForbidden, utils.Envelope{"error": "you are not authorized to update that workout"})
		return
	}

	err = wh.workoutStore.UpdateWorkout(existingWorkout)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("update workout", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.Envelope{"workout": existingWorkout})
}

func (wh *WorkoutHandler) DeleteWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIDParam(r)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("read id param", "err", err)
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "invalid workout id"})
		return
	}

	currentUser := middleware.GetUser(r)
	if currentUser == nil || currentUser == store.AnonymousUser {
		utils.WriteJSON(w, http.StatusBadRequest, utils.Envelope{"error": "you must be logged in"})
		return
	}

	workoutOwner, err := wh.workoutStore.GetWorkoutOwner(workoutID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJSON(w, http.StatusNotFound, utils.Envelope{"error": "workout does not exist"})
			return
		}

		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if workoutOwner != currentUser.ID {
		utils.WriteJSON(w, http.StatusForbidden, utils.Envelope{"error": "you are not authorized to delete that workout"})
		return
	}

	err = wh.workoutStore.DeleteWorkoutByID(workoutID, currentUser.ID)
	if errors.Is(err, sql.ErrNoRows) {
		utils.WriteJSON(w, http.StatusNotFound, utils.Envelope{"error": "workout does not exist"})
		return
	}

	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("delete workout", "err", err)
		utils.WriteJSON(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
