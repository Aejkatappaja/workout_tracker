// Package routes
package routes

import (
	"net"
	"net/http"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/app"
	"github.com/Aejkatappaja/go-gym/internal/docs"
	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

// ipKey rate-limits by client IP. RemoteAddr is resolved from X-Forwarded-For by
// RealIP behind a trusted proxy, otherwise it is the direct peer.
func ipKey(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host, nil
}

func SetupRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimw.RealIP, chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.BodyLimit(1 << 20)) // 1 MiB

	// throttle credential endpoints against brute force / stuffing.
	// keys off RemoteAddr, which RealIP resolves from X-Forwarded-For behind a trusted proxy.
	authLimit := httprate.LimitBy(10, time.Minute, ipKey)

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

	r.With(authLimit).Post("/users", app.UserHandler.HandleRegisterUser)
	r.With(authLimit).Post("/tokens/authentication", app.TokenHandler.HandleCreateToken)

	// browser UI: static assets and public auth pages
	r.Handle("/static/*", app.WebHandler.Static())
	r.Get("/login", app.WebHandler.LoginPage)
	r.With(authLimit).Post("/login", app.WebHandler.Login)
	r.With(authLimit).Get("/demo", app.WebHandler.DemoLogin)
	r.Get("/register", app.WebHandler.RegisterPage)
	r.With(authLimit).Post("/register", app.WebHandler.Register)

	return r
}
