package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (a *App) routes() http.Handler {
	s := chi.NewRouter()

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
				s.Get("/{id}", a.handleGetPage)
				s.Get("/{id}/backlinks", a.handleGetBacklinks)
				s.Get("/{id}/blocks", a.handleGetPageBlocks)

				s.Post("/", a.handleCreatePage)
				s.Post("/{id}/restore", a.handleRestorePage)
				s.Post("/{id}/wiki-links", a.handleSyncWikiLinks)

				s.Patch("/{id}", a.handleUpdatePage)

				s.Delete("/{id}", a.handleDeletePage)

			})

			// block routes
			{
				s.Post("/pages/{id}/blocks", a.handleCreateBlock)
				s.Post("/blocks/{id}/restore", a.handleRestoreBlock)

				s.Patch("/blocks/{id}", a.handleUpdateBlock)
				s.Patch("/pages/{id}/blocks/reorder", a.handleReorderBlocks)

				s.Delete("/blocks/{id}", a.handleDeleteBlock)
			}

			s.Route("/transactions", func(s chi.Router) {

				s.Get("/", a.handleTransactions)
				s.Get("/{id}", a.handleGetTransaction)

				s.Post("/", a.handleCreateTransaction)
				s.Post("/{id}/restore", a.handleRestoreTransaction)

				s.Patch("/{id}", a.handleUpdateTransaction)
				s.Delete("/{id}", a.handleDeleteTransaction)

			})

			s.Route("/pockets", func(s chi.Router) {

				s.Get("/", a.handlePockets)

				s.Post("/", a.handleCreatePocket)
				s.Post("/{id}/restore", a.handleRestorePocket)

				s.Patch("/{id}", a.handleUpdatePocket)
				s.Delete("/{id}", a.handleDeletePocket)

			})

			s.Route("/tags", func(s chi.Router) {

				s.Get("/", a.handleGetTags)

				s.Post("/", a.handleCreateTag)
				s.Post("/{id}/restore", a.handleRestoreTag)
				s.Post("/{id}/attach", a.handleAttachTag)

				s.Patch("/{id}", a.handleUpdateTag)

				s.Delete("/{id}", a.handleDeleteTag)
				s.Delete("/{id}/detach", a.handleDetachTag)

			})

			s.Route("/edges", func(s chi.Router) {
				s.Get("/", a.handleGetEdges)
				s.Post("/", a.handleCreateEdge)
				s.Delete("/", a.handleDeleteEdge)
			})
		})

		// page fragment
		s.Route("/pages", func(s chi.Router) {
			s.Get("/{id}/fragment", a.handlePageFragment)
		})

		// finance fragments
		s.Route("/fin", func(s chi.Router) {
			s.Get("/pockets", a.handlePocketsFragment)
			s.Get("/transactions", a.handleTransactionsFragment)
			s.Get("/trash", a.handleTrashFragment)
		})
	})

	// static file
	s.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	s.Handle("/templui/js/*",
		http.StripPrefix("/templui/js/", http.FileServer(http.Dir("static/js"))),
	)

	s.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "not found",
		})
	})

	return s
}
