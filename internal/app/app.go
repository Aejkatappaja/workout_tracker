// Package app
package app

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/Aejkatappaja/go-gym/internal/api"
	"github.com/Aejkatappaja/go-gym/internal/mail"
	"github.com/Aejkatappaja/go-gym/internal/middleware"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/web"
	"github.com/Aejkatappaja/go-gym/migrations"
)

type Application struct {
	Logger           *slog.Logger
	WorkoutHandler   *api.WorkoutHandler
	UserHandler      *api.UserHandler
	TokenHandler     *api.TokenHandler
	ExerciseHandler  *api.ExerciseHandler
	AnalyticsHandler *api.AnalyticsHandler
	MiddleWare       middleware.UserMiddleware
	WebHandler       *web.Handler
	DB               *sql.DB
}

func NewApplication() (*Application, error) {
	logger := newLogger()

	pgDB, err := store.Open()
	if err != nil {
		return nil, err
	}

	err = store.MigrateFS(pgDB, migrations.FS, ".")
	if err != nil {
		panic(err)
	}

	if err := store.SeedDemo(pgDB); err != nil {
		logger.Warn("seeding demo account", "err", err)
	}

	workoutStore := store.NewPostgresWorkoutStore(pgDB)
	userStore := store.NewPostgresUserStore(pgDB)
	tokenStore := store.NewPostgresTokenStore(pgDB)
	exerciseStore := store.NewPostgresExerciseStore(pgDB)
	analyticsStore := store.NewPostgresAnalyticsStore(pgDB)

	mailer := mail.New(logger, os.Getenv("RESEND_API_KEY"), os.Getenv("MAIL_FROM"))

	workoutHandler := api.NewWorkoutHandler(workoutStore)
	userHandler := api.NewUserHandler(userStore)
	tokenHandler := api.NewTokenHandler(tokenStore, userStore)
	exerciseHandler := api.NewExerciseHandler(exerciseStore)
	analyticsHandler := api.NewAnalyticsHandler(analyticsStore)
	middlewareHandler := middleware.UserMiddleware{UserStore: userStore}
	webHandler := web.NewHandler(userStore, tokenStore, workoutStore, exerciseStore, analyticsStore, mailer)

	app := &Application{
		Logger:           logger,
		WorkoutHandler:   workoutHandler,
		UserHandler:      userHandler,
		TokenHandler:     tokenHandler,
		ExerciseHandler:  exerciseHandler,
		AnalyticsHandler: analyticsHandler,
		MiddleWare:       middlewareHandler,
		WebHandler:       webHandler,
		DB:               pgDB,
	}

	return app, nil
}

func (a *Application) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := a.DB.PingContext(r.Context()); err != nil {
		middleware.LoggerFrom(r.Context()).Error("health check DB ping", "err", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, "database unavailable\n")
		return
	}
	_, _ = fmt.Fprint(w, "Status is available!\n")
}

// newLogger builds the process logger. LOG_FORMAT=json selects JSON output (for
// production log aggregation), otherwise human-readable text. LOG_LEVEL sets the
// minimum level (debug|info|warn|error, default info). It is also installed as the
// slog default so code without a request-scoped logger still logs consistently.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler = slog.NewTextHandler(os.Stdout, opts)
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}
