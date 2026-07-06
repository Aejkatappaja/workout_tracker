package web

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/utils"
	"github.com/Aejkatappaja/go-gym/internal/web/views"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxEntries = 100

// validateWorkout returns a user-facing message if the workout is invalid, or "".
// Mirrors the DB constraints so the user gets a clear error instead of a raw
// check_violation, and each exercise must track exactly one of reps or duration.
func validateWorkout(wk *store.Workout) string {
	if strings.TrimSpace(wk.Title) == "" {
		return "title is required"
	}
	if len(wk.Entries) == 0 {
		return "add at least one exercise"
	}
	if len(wk.Entries) > maxEntries {
		return fmt.Sprintf("too many exercises (max %d)", maxEntries)
	}
	for _, e := range wk.Entries {
		if (e.Reps != nil) == (e.DurationSeconds != nil) {
			return "each exercise needs exactly one of reps or duration"
		}
	}
	return ""
}

// checkOwner resolves the {id} param and verifies the caller owns that workout,
// using the cheap owner-only query (no entries loaded). Missing and not-owned
// are both reported as ok=false so the UI cannot probe which ids exist.
func (h *Handler) checkOwner(r *http.Request) (id int64, ok bool, err error) {
	id, perr := utils.ReadIDParam(r)
	if perr != nil {
		return 0, false, nil
	}
	owner, err := h.workouts.GetWorkoutOwner(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return id, false, nil
		}
		return 0, false, err
	}
	if owner != middleware.GetUser(r).ID {
		return id, false, nil
	}
	return id, true, nil
}

func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	workouts, err := h.workouts.ListWorkoutsByUser(user.ID)
	if err != nil {
		h.logger.Printf("ERROR: web dashboard: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.render(w, r, http.StatusOK, views.Dashboard(user.Username, workouts))
}

// loadOwnedWorkout fetches a workout by the {id} param and checks ownership.
// A missing workout and one owned by someone else are both returned as nil so
// the UI cannot be used to probe which ids exist.
func (h *Handler) loadOwnedWorkout(r *http.Request) (*store.Workout, error) {
	id, err := utils.ReadIDParam(r)
	if err != nil {
		return nil, nil
	}
	wk, err := h.workouts.GetWorkoutByID(id)
	if err != nil {
		return nil, err
	}
	if wk == nil || wk.UserID != middleware.GetUser(r).ID {
		return nil, nil
	}
	return wk, nil
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	wk, err := h.loadOwnedWorkout(r)
	if err != nil {
		h.logger.Printf("ERROR: web detail: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if wk == nil {
		h.notFound(w, r)
		return
	}
	h.render(w, r, http.StatusOK, views.WorkoutDetail(middleware.GetUser(r).Username, *wk))
}

func (h *Handler) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	blank := store.Workout{Entries: []store.WorkoutEntry{{}}}
	h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, blank, "", "/app/workouts", "vim ~/workouts/new"))
}

func (h *Handler) EntryRow(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, views.EntryRow(store.WorkoutEntry{}))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	wk := parseWorkoutForm(r)
	wk.UserID = user.ID

	if msg := validateWorkout(&wk); msg != "" {
		h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, wk, msg, "/app/workouts", "vim ~/workouts/new"))
		return
	}

	created, err := h.workouts.CreateWorkout(&wk)
	if err != nil {
		h.logger.Printf("ERROR: web create workout: %v", err)
		h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, wk, saveErrMsg(err), "/app/workouts", "vim ~/workouts/new"))
		return
	}
	w.Header().Set("HX-Redirect", "/app/workouts/"+strconv.Itoa(created.ID))
}

func (h *Handler) EditForm(w http.ResponseWriter, r *http.Request) {
	wk, err := h.loadOwnedWorkout(r)
	if err != nil {
		h.logger.Printf("ERROR: web edit form: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if wk == nil {
		h.notFound(w, r)
		return
	}
	user := middleware.GetUser(r)
	action := "/app/workouts/" + strconv.Itoa(wk.ID)
	h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, *wk, "", action, "vim ~/workouts/"+strconv.Itoa(wk.ID)))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok, err := h.checkOwner(r)
	if err != nil {
		h.logger.Printf("ERROR: web update lookup: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		h.notFound(w, r)
		return
	}

	user := middleware.GetUser(r)
	wk := parseWorkoutForm(r)
	wk.ID = int(id)
	wk.UserID = user.ID
	action := "/app/workouts/" + strconv.FormatInt(id, 10)
	heading := "vim ~/workouts/" + strconv.FormatInt(id, 10)

	if msg := validateWorkout(&wk); msg != "" {
		h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, wk, msg, action, heading))
		return
	}

	if err := h.workouts.UpdateWorkout(&wk); err != nil {
		h.logger.Printf("ERROR: web update workout: %v", err)
		h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, wk, saveErrMsg(err), action, heading))
		return
	}
	w.Header().Set("HX-Redirect", action)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok, err := h.checkOwner(r)
	if err != nil {
		h.logger.Printf("ERROR: web delete lookup: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := h.workouts.DeleteWorkoutByID(id, middleware.GetUser(r).ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.notFound(w, r)
			return
		}
		h.logger.Printf("ERROR: web delete workout: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Redirect", "/app")
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusNotFound, views.NotFound(middleware.GetUser(r).Username))
}

// parseWorkoutForm builds a Workout from the create/edit form. Entry inputs are
// submitted as parallel arrays (entry_exercise[], entry_sets[], ...); empty
// exercise names are skipped.
func parseWorkoutForm(r *http.Request) store.Workout {
	_ = r.ParseForm()

	wk := store.Workout{
		Title:           strings.TrimSpace(r.FormValue("title")),
		Description:     strings.TrimSpace(r.FormValue("description")),
		DurationMinutes: atoiOr0(r.FormValue("duration_minutes")),
		CaloriesBurned:  atoiOr0(r.FormValue("calories_burned")),
	}

	names := r.Form["entry_exercise"]
	sets := r.Form["entry_sets"]
	reps := r.Form["entry_reps"]
	durs := r.Form["entry_duration"]
	weights := r.Form["entry_weight"]
	notes := r.Form["entry_notes"]

	order := 1
	for i, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		wk.Entries = append(wk.Entries, store.WorkoutEntry{
			ExerciseName:    strings.TrimSpace(name),
			Sets:            atoiOr0(at(sets, i)),
			Reps:            atoiPtr(at(reps, i)),
			DurationSeconds: atoiPtr(at(durs, i)),
			Weight:          atofPtr(at(weights, i)),
			Notes:           strings.TrimSpace(at(notes, i)),
			OrderIndex:      order,
		})
		order++
	}
	return wk
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

func atoiOr0(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atoiPtr(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

func atofPtr(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// saveErrMsg maps a store error to a user-facing message.
func saveErrMsg(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check_violation
		return "each exercise needs exactly one of reps or duration (not both, not neither)"
	}
	return "could not save the workout, check the fields and try again"
}
