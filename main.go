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

	go func() {
		application.Logger.Printf("listening on port %d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			application.Logger.Fatalf("server error: %v", err)
		}
	}()

	// graceful shutdown on SIGINT / SIGTERM (container stop)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	application.Logger.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		application.Logger.Printf("graceful shutdown failed: %v", err)
	}
}
