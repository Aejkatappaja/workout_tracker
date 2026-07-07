package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/app"
	"github.com/Aejkatappaja/go-gym/internal/routes"
)

func main() {
	defaultPort := 8080
	if p, err := strconv.Atoi(os.Getenv("PORT")); err == nil && p > 0 {
		defaultPort = p
	}
	var port int
	flag.IntVar(&port, "port", defaultPort, "HTTP server port")
	flag.Parse()

	application, err := app.NewApplication()
	if err != nil {
		panic(err)
	}
	defer func() { _ = application.DB.Close() }()

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      routes.SetupRoutes(application),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// cancelled on SIGINT / SIGTERM (container stop); also stops the recap loop.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if application.Recap != nil {
		go application.Recap.Run(ctx, time.Hour)
	}

	go func() {
		application.Logger.Info("listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			application.Logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	application.Logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		application.Logger.Error("graceful shutdown failed", "err", err)
	}
}
