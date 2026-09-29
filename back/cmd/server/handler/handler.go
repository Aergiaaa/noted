// Package handler holds the HTTP handler layer: parse/validate, call one
// service method, map to status/JSON (see ARCHITECTURE.md). One file per
// endpoint (healthz.go, ...); route registration stays in the server's
// router.go.
package handler

// Handler carries the handler layer's dependencies (db, services) — empty
// until F3/F4 inject them via New. Constructors stay the wiring seam so
// router.go never changes when a dependency lands.
type Handler struct{}

// New builds a Handler. Args are added here when deps exist, not at call
// sites.
func New() *Handler {
	return &Handler{}
}
