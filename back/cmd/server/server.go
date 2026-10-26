package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// shutdownTimeout is a var so tests can force the timeout-error branch
// without waiting the full 25s. Kept just under compose's
// stop_grace_period (30s) so db close + logs finish before Docker would
// SIGKILL.
var shutdownTimeout = 25 * time.Second

// ReadTimeout/WriteTimeout cap the whole request/response, not just the
// headers: a future import (large upload) or export (long stream)
// endpoint must lift them or the transfer gets cut off mid-flight —
// healthz and the F2 routes stay well inside 10s/30s.
func (a *app) newServer() *http.Server {
	return &http.Server{
		Addr:              a.conf.Addr,
		Handler:           a.newRouter(),
		IdleTimeout:       time.Minute,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
}

func (a *app) serve(ctx context.Context) error {
	s := a.newServer()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("listening", "addr", a.conf.Addr)

	errCh := make(chan error, 1)
	go listenAndServeWithErrCh(s, errCh)

	return listenCtxAndCh(s, ctx, errCh)
}

func listenCtxAndCh(server *http.Server, ctx context.Context, errCh chan error) error {
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(sctx); err != nil {
			return err
		}

		slog.Info("graceful shutdown complete")
		return <-errCh
	}
}

// listenAndServeWithErrCh sends exactly one result: the listen error, or
// nil after a clean ErrServerClosed (produced by Shutdown).
func listenAndServeWithErrCh(server *http.Server, errCh chan error) {
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errCh <- err
		return
	}

	errCh <- nil
}
