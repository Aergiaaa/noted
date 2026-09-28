package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- router ---

func TestRouter_unknownRoute_returns404(t *testing.T) {
	for _, path := range []string{"/nope", "/healthz/", "/api/healthz"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
			}
		})
	}
}

func TestRouter_healthz_wrongMethod_returns405(t *testing.T) {
	for _, method := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/healthz", nil)
			rec := httptest.NewRecorder()

			newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestRouter_healthz_queryParams_ignored(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz?foo=bar", nil)
	rec := httptest.NewRecorder()

	newTestApp(Config{}).newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
