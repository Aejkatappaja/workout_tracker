// Package routes
package routes

import (
	"github.com/Aejkatappaja/workout_tracker/internal/app"
	"github.com/Aejkatappaja/workout_tracker/internal/docs"
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(app.MiddleWare.Authenticate)

		r.Get("/workouts", app.MiddleWare.RequireUser(app.WorkoutHandler.HandleListWorkouts))
		r.Get("/workouts/{id}", app.MiddleWare.RequireUser(app.WorkoutHandler.HandleGetWorkoutByID))
		r.Post("/workouts", app.MiddleWare.RequireUser(app.WorkoutHandler.HandleCreateWorkout))
		r.Put("/workouts/{id}", app.MiddleWare.RequireUser(app.WorkoutHandler.HandleUpdatedWorkoutByID))
		r.Delete("/workouts/{id}", app.MiddleWare.RequireUser(app.WorkoutHandler.DeleteWorkout))

		// web routes that need the authenticated user in context
		r.Post("/logout", app.WebHandler.Logout)

		// browser UI (server-rendered HTMX), redirects anonymous users to /login
		r.Get("/", app.WebHandler.Root)
		r.Get("/app", app.MiddleWare.RequireUserWeb(app.WebHandler.Dashboard))
		r.Get("/app/workouts/new", app.MiddleWare.RequireUserWeb(app.WebHandler.NewForm))
		r.Get("/app/workouts/entry-row", app.MiddleWare.RequireUserWeb(app.WebHandler.EntryRow))
		r.Post("/app/workouts", app.MiddleWare.RequireUserWeb(app.WebHandler.Create))
		r.Get("/app/workouts/{id}", app.MiddleWare.RequireUserWeb(app.WebHandler.Detail))
		r.Get("/app/workouts/{id}/edit", app.MiddleWare.RequireUserWeb(app.WebHandler.EditForm))
		r.Post("/app/workouts/{id}", app.MiddleWare.RequireUserWeb(app.WebHandler.Update))
		r.Delete("/app/workouts/{id}", app.MiddleWare.RequireUserWeb(app.WebHandler.Delete))
	})

	r.Get("/health", app.HealthCheck)

	r.Get("/docs", docs.UI)
	r.Get("/openapi.yaml", docs.Spec)

	r.Post("/users", app.UserHandler.HandleRegisterUser)
	r.Post("/tokens/authentication", app.TokenHandler.HandleCreateToken)

	// browser UI: static assets and public auth pages
	r.Handle("/static/*", app.WebHandler.Static())
	r.Get("/login", app.WebHandler.LoginPage)
	r.Post("/login", app.WebHandler.Login)
	r.Get("/register", app.WebHandler.RegisterPage)
	r.Post("/register", app.WebHandler.Register)

	return r
}
