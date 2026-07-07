package web

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// readOnly reports whether the caller is the demo account, which cannot write.
func (h *Handler) readOnly(r *http.Request) bool {
	return middleware.GetUser(r).Username == store.DemoUsername
}

const activityWeeks = 16

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	workouts, err := h.workouts.ListWorkoutsByUser(user.ID)
	if err != nil {
		h.serverError(w, r, "web dashboard list", err)
		return
	}

	since := time.Now().AddDate(0, 0, -activityWeeks*7)
	counts, err := h.workouts.WorkoutCountsByDay(user.ID, since)
	if err != nil {
		h.serverError(w, r, "web dashboard activity", err)
		return
	}

	stats := views.Stats{Sessions: len(workouts)}
	for _, wk := range workouts {
		stats.Minutes += wk.DurationMinutes
		stats.Calories += wk.CaloriesBurned
	}
	activity := views.BuildActivity(counts, activityWeeks, time.Now())

	h.render(w, r, http.StatusOK, views.Dashboard(user.Username, workouts, stats, activity, h.readOnly(r)))
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
		h.serverError(w, r, "web detail", err)
		return
	}
	if wk == nil {
		h.notFound(w, r)
		return
	}
	h.render(w, r, http.StatusOK, views.WorkoutDetail(middleware.GetUser(r).Username, *wk, h.readOnly(r)))
}

func (h *Handler) NewForm(w http.ResponseWriter, r *http.Request) {
	if h.readOnly(r) {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	user := middleware.GetUser(r)
	blank := store.Workout{Entries: []store.WorkoutEntry{{}}}
	h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, blank, "", "/app/workouts", "vim ~/workouts/new"))
}

func (h *Handler) EntryRow(w http.ResponseWriter, r *http.Request) {
	if h.readOnly(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	h.render(w, r, http.StatusOK, views.EntryRow(store.WorkoutEntry{}))
}

// ExerciseSearch returns the typeahead dropdown fragment for the exercise input.
// HTMX sends the input's value under its own name (entry_exercise).
func (h *Handler) ExerciseSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("entry_exercise"))
	if q == "" {
		h.render(w, r, http.StatusOK, views.ExerciseSuggest(nil, "", false))
		return
	}
	exercises, err := h.exercises.Search(q, 6)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("web exercise search", "err", err)
		h.render(w, r, http.StatusOK, views.ExerciseSuggest(nil, q, false))
		return
	}
	// exact catalog match -> no "create" affordance
	exact := false
	lq := strings.ToLower(q)
	for _, e := range exercises {
		if e.Name == lq {
			exact = true
			break
		}
	}
	h.render(w, r, http.StatusOK, views.ExerciseSuggest(exercises, q, exact))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if h.readOnly(r) {
		w.Header().Set("HX-Redirect", "/app")
		return
	}
	user := middleware.GetUser(r)
	wk := parseWorkoutForm(r)
	wk.UserID = user.ID

	if msg := validateWorkout(&wk); msg != "" {
		h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, wk, msg, "/app/workouts", "vim ~/workouts/new"))
		return
	}

	created, err := h.workouts.CreateWorkout(&wk)
	if err != nil {
		middleware.LoggerFrom(r.Context()).Error("web create workout", "err", err)
		h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, wk, saveErrMsg(err), "/app/workouts", "vim ~/workouts/new"))
		return
	}
	w.Header().Set("HX-Redirect", "/app/workouts/"+strconv.Itoa(created.ID))
}

func (h *Handler) EditForm(w http.ResponseWriter, r *http.Request) {
	if h.readOnly(r) {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	wk, err := h.loadOwnedWorkout(r)
	if err != nil {
		h.serverError(w, r, "web edit form", err)
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
	if h.readOnly(r) {
		w.Header().Set("HX-Redirect", "/app")
		return
	}
	id, ok, err := h.checkOwner(r)
	if err != nil {
		h.serverError(w, r, "web update lookup", err)
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
		middleware.LoggerFrom(r.Context()).Error("web update workout", "err", err)
		h.render(w, r, http.StatusOK, views.WorkoutForm(user.Username, wk, saveErrMsg(err), action, heading))
		return
	}
	w.Header().Set("HX-Redirect", action)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.readOnly(r) {
		w.Header().Set("HX-Redirect", "/app")
		return
	}
	id, ok, err := h.checkOwner(r)
	if err != nil {
		h.serverError(w, r, "web delete lookup", err)
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
		h.serverError(w, r, "web delete workout", err)
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
	groups := r.Form["entry_muscle_group"]
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
			MuscleGroup:     strings.TrimSpace(at(groups, i)), // set only when creating a new exercise
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
