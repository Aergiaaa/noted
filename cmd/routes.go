package main

import (
	"net/http"

	// "github.com/templui/templui/utils"

	"github.com/Aergiaaa/noted/ui"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *app) routes() http.Handler {
	s := chi.NewMux()

	s.Use(
		middleware.SetHeader("X-Frame-Options", "DENY"),
		middleware.SetHeader("X-Content-Type-Options", "nosniff"),
		middleware.SetHeader("Referrer-Policy", "strict-origin-when-cross-origin"),
	)

	s.Get("/", app.handleIndex)

	// public page
	s.Route("/auth", func(s chi.Router) {

		s.Post("/login", app.handleNOP)
		s.Post("/verify-totp", app.handleNOP)
		s.Post("/logout", app.handleNOP)

	})

	// authorized page
	s.Group(func(s chi.Router) {
		s.Use(app.authMiddleware)

		s.Route("/pages", func(s chi.Router) {

			s.Get("/", app.handleNOP)
			s.Get("/{id}", app.handleNOP)
			s.Get("/{id}/backlinks", app.handleNOP)
			s.Get("/{id}/blocks", app.handleNOP)

			s.Post("/", app.handleNOP)
			s.Post("/{id}/restore", app.handleNOP)

			s.Patch("/{id}", app.handleNOP)

			s.Delete("/{id}", app.handleNOP)

		})

		// block routes
		{
			s.Post("/pages/{id}/blocks", app.handleNOP)
			s.Post("/blocks/{id}/restore", app.handleNOP)

			s.Patch("/blocks/{id}", app.handleNOP)
			s.Patch("/pages/{id}/blocks/reorder", app.handleNOP)

			s.Delete("/blocks/{id}", app.handleNOP)
		}

		s.Route("/transactions", func(s chi.Router) {

			s.Get("/", app.handleNOP)
			s.Get("/{id}", app.handleNOP)

			s.Post("/", app.handleNOP)
			s.Post("/{id}/restore", app.handleNOP)

			s.Patch("/{id}", app.handleNOP)
			s.Delete("/{id}", app.handleNOP)

		})

		s.Route("/pockets", func(s chi.Router) {

			s.Get("/", app.handleNOP)

			s.Post("/", app.handleNOP)
			s.Post("/{id}/restore", app.handleNOP)

			s.Patch("/{id}", app.handleNOP)
			s.Delete("/{id}", app.handleNOP)

		})

		s.Route("/tags", func(s chi.Router) {

			s.Get("/", app.handleNOP)

			s.Post("/", app.handleNOP)
			s.Post("/{id}/restore", app.handleNOP)
			s.Post("/{id}/attach", app.handleNOP)

			s.Delete("/{id}", app.handleNOP)
			s.Delete("/{id}/detach", app.handleNOP)

		})

		s.Route("/edges", func(s chi.Router) {
			s.Get("/", app.handleNOP)
			s.Post("/", app.handleNOP)
			s.Delete("/", app.handleNOP)
		})

	})

	s.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return s
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	ui.Index().Render(r.Context(), w)
}

func (a *app) handleNOP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("NOP"))
}
