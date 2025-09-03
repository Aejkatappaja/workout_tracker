// Package routes
package routes

import (
	"github.com/aejkatappaja/project/internal/app"
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(app.MiddleWare.Authenticate)

		r.Get("/workouts/{id}", app.MiddleWare.RequireUser(app.WorkoutHandler.HandleGetWorkoutByID))
		r.Post("/workouts", app.MiddleWare.RequireUser(app.WorkoutHandler.HandleCreateWorkout))
		r.Put("/workouts/{id}", app.MiddleWare.RequireUser(app.WorkoutHandler.HandleUpdatedWorkoutByID))
		r.Delete("/workouts/{id}", app.MiddleWare.RequireUser(app.WorkoutHandler.DeleteWorkout))
	})

	r.Get("/health", app.HealthCheck)

	r.Post("/users", app.UserHandler.HandleRegisterUser)
	r.Post("/tokens/authentication", app.TokenHandler.HandleCreateToken)

	return r
}
