// Package app
package app

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Aejkatappaja/go-gym/internal/api"
	"github.com/Aejkatappaja/go-gym/internal/mail"
	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/web"
	"github.com/Aejkatappaja/go-gym/migrations"
)

type Application struct {
	Logger          *log.Logger
	WorkoutHandler  *api.WorkoutHandler
	UserHandler     *api.UserHandler
	TokenHandler    *api.TokenHandler
	ExerciseHandler *api.ExerciseHandler
	MiddleWare      middleware.UserMiddleware
	WebHandler      *web.Handler
	DB              *sql.DB
}

func NewApplication() (*Application, error) {
	pgDB, err := store.Open()
	if err != nil {
		return nil, err
	}

	err = store.MigrateFS(pgDB, migrations.FS, ".")
	if err != nil {
		panic(err)
	}

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	if err := store.SeedDemo(pgDB); err != nil {
		logger.Printf("WARN: seeding demo account: %v", err)
	}

	workoutStore := store.NewPostgresWorkoutStore(pgDB)
	userStore := store.NewPostgresUserStore(pgDB)
	tokenStore := store.NewPostgresTokenStore(pgDB)
	exerciseStore := store.NewPostgresExerciseStore(pgDB)

	mailer := mail.New(logger, os.Getenv("RESEND_API_KEY"), os.Getenv("MAIL_FROM"))

	workoutHandler := api.NewWorkoutHandler(workoutStore, logger)
	userHandler := api.NewUserHandler(userStore, logger)
	tokenHandler := api.NewTokenHandler(tokenStore, userStore, logger)
	exerciseHandler := api.NewExerciseHandler(exerciseStore, logger)
	middlewareHandler := middleware.UserMiddleware{UserStore: userStore}
	webHandler := web.NewHandler(userStore, tokenStore, workoutStore, exerciseStore, logger, mailer)

	app := &Application{
		Logger:          logger,
		WorkoutHandler:  workoutHandler,
		UserHandler:     userHandler,
		TokenHandler:    tokenHandler,
		ExerciseHandler: exerciseHandler,
		MiddleWare:      middlewareHandler,
		WebHandler:      webHandler,
		DB:              pgDB,
	}

	return app, nil
}

func (a *Application) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := a.DB.PingContext(r.Context()); err != nil {
		a.Logger.Printf("ERROR: health check DB ping: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, "database unavailable\n")
		return
	}
	_, _ = fmt.Fprint(w, "Status is available!\n")
}
