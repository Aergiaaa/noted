package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/v5/middleware"
)

func (a *App) routes() http.Handler {
	s := chi.NewMux()

	s.Use(
		middleware.SetHeader("X-Frame-Options", "DENY"),
		middleware.SetHeader("X-Content-Type-Options", "nosniff"),
		middleware.SetHeader("Referrer-Policy", "strict-origin-when-cross-origin"),
	)

	// public page
	s.Route("/auth", func(s chi.Router) {

		s.Get("/login", a.renderLogin)

		s.Get("/logout", a.handleLogout)
		s.Post("/verify", a.handleVerify)
	})

	// authorized page
	s.Group(func(s chi.Router) {
		s.Use(a.authMiddleware)

		s.Get("/", a.renderIndex)

		s.Route("/api", func(s chi.Router) {
			s.Route("/pages", func(s chi.Router) {

				s.Get("/", a.handlePages)
				s.Get("/{id}", a.handleNOP)
				s.Get("/{id}/backlinks", a.handleNOP)
				s.Get("/{id}/blocks", a.handleNOP)

				s.Post("/", a.handleCreatePage)
				s.Post("/{id}/restore", a.handleNOP)

				s.Patch("/{id}", a.handleNOP)

				s.Delete("/{id}", a.handleNOP)

			})

			// block routes
			{
				s.Post("/pages/{id}/blocks", a.handleCreateBlock)
				s.Post("/blocks/{id}/restore", a.handleNOP)

				s.Patch("/blocks/{id}", a.handleUpdateBlock)
				s.Patch("/pages/{id}/blocks/reorder", a.handleNOP)

				s.Delete("/blocks/{id}", a.handleDeleteBlock)
			}

			s.Route("/transactions", func(s chi.Router) {

				s.Get("/", a.handleNOP)
				s.Get("/{id}", a.handleNOP)

				s.Post("/", a.handleNOP)
				s.Post("/{id}/restore", a.handleNOP)

				s.Patch("/{id}", a.handleNOP)
				s.Delete("/{id}", a.handleNOP)

			})

			s.Route("/pockets", func(s chi.Router) {

				s.Get("/", a.handleNOP)

				s.Post("/", a.handleNOP)
				s.Post("/{id}/restore", a.handleNOP)

				s.Patch("/{id}", a.handleNOP)
				s.Delete("/{id}", a.handleNOP)

			})

			s.Route("/tags", func(s chi.Router) {

				s.Get("/", a.handleNOP)

				s.Post("/", a.handleNOP)
				s.Post("/{id}/restore", a.handleNOP)
				s.Post("/{id}/attach", a.handleNOP)

				s.Delete("/{id}", a.handleNOP)
				s.Delete("/{id}/detach", a.handleNOP)

			})

			s.Route("/edges", func(s chi.Router) {
				s.Get("/", a.handleNOP)
				s.Post("/", a.handleNOP)
				s.Delete("/", a.handleNOP)
			})
		})

		// page fragment
		s.Route("/pages", func(s chi.Router) {
			s.Get("/{id}/fragment", a.handlePageFragment)
		})

		s.NotFound(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "method not allowed",
			})
		})
	})

	// static file
	s.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	s.Handle("/templui/js/*",
		http.StripPrefix("/templui/js/", http.FileServer(http.Dir("static/js"))),
	)

	return s
}
