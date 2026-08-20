package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	READ_TIMEOUT_SEC  = 10 * time.Second
	WRITE_TIMEOUT_SEC = 30 * time.Second
	IDLE_TIMEOUT      = time.Minute
)

func createServer(app *App) *http.Server {
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", app.port),
		Handler:      app.routes(),
		IdleTimeout:  IDLE_TIMEOUT,
		ReadTimeout:  READ_TIMEOUT_SEC,
		WriteTimeout: WRITE_TIMEOUT_SEC,
	}
}

func (app *App) serve() error {
	s := createServer(app)

	log.Printf("Starting server on %s", s.Addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Printf("Shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return s.Shutdown(shutdownCtx)
	}
}
