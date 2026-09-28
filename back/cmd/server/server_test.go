package main

import (
	"testing"
	"time"
)

// --- newServer ---

func TestNewServer_wiresHandlerAndTimeouts(t *testing.T) {
	cfg := Config{Addr: "127.0.0.1:8080"}
	srv := newTestApp(cfg).newServer()

	if srv.Addr != cfg.Addr {
		t.Fatalf("Addr = %q, want %q", srv.Addr, cfg.Addr)
	}
	if srv.Handler == nil {
		t.Fatal("Handler is nil")
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != time.Minute {
		t.Fatalf("IdleTimeout = %v, want 1m", srv.IdleTimeout)
	}
	if srv.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout = %v, want 10s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 30*time.Second {
		t.Fatalf("WriteTimeout = %v, want 30s", srv.WriteTimeout)
	}
}
