package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// shutdownTimeout bounds graceful shutdown. Package var (not const) so tests
// can force the timeout-error branch without waiting the full 25s. Kept just
// under compose's stop_grace_period (30s) so db close + logs finish before
// Docker would SIGKILL.
var shutdownTimeout = 25 * time.Second

// newServer wires the handler + timeouts in one place so serve() and tests
// share the same constructor. ReadTimeout/WriteTimeout cap the whole
// request/response, not just the headers: a future import (large upload)
// or export (long stream) endpoint must lift them or the transfer gets
// cut off mid-flight — healthz and the F2 routes stay well inside 10s/30s.
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

// serve listens until ctx is cancelled or SIGINT/SIGTERM arrives, then
// shuts down gracefully within shutdownTimeout. The listener reports its
// result through errCh so the select can race it against cancellation.
func (a *app) serve(ctx context.Context) error {
	s := a.newServer()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("listening on %s", a.conf.Addr)

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
		log.Print("graceful shutdown complete")
		return <-errCh
	}
}

// listenAndServeWithErrCh sends exactly one result to errCh: the listen error,
// or nil after a clean ErrServerClosed (produced by Shutdown).
func listenAndServeWithErrCh(server *http.Server, errCh chan error) {
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errCh <- err
		return
	}
	errCh <- nil
}
