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
		s.Get("/login", a.handle.RenderLogin)
		s.Get("/logout", a.handleLogout)
		s.Post("/verify", a.handleVerify)
	})

	// authorized page
	s.Group(func(s chi.Router) {
		s.Use(a.authMiddleware)

		s.Get("/", a.handle.RenderIndex)

		s.Route("/api", func(s chi.Router) {
			s.Route("/pages", func(s chi.Router) {

				s.Get("/", a.handle.Pages)
				s.Get("/{id}", a.handle.GetPage)
				s.Get("/{id}/backlinks", a.handle.GetBacklinks)
				s.Get("/{id}/blocks", a.handle.GetPageBlocks)

				s.Post("/", a.handle.CreatePage)
				s.Post("/{id}/restore", a.handle.RestorePage)
				s.Post("/{id}/wiki-links", a.handle.SyncWikiLinks)

				s.Patch("/{id}", a.handle.UpdatePage)

				s.Delete("/{id}", a.handle.DeletePage)

			})

			// block routes
			{
				s.Post("/pages/{id}/blocks", a.handle.CreateBlock)
				s.Post("/blocks/{id}/restore", a.handle.RestoreBlock)

				s.Patch("/blocks/{id}", a.handle.UpdateBlock)
				s.Patch("/pages/{id}/blocks/reorder", a.handle.ReorderBlocks)

				s.Delete("/blocks/{id}", a.handle.DeleteBlock)
			}

			s.Route("/transactions", func(s chi.Router) {

				s.Get("/", a.handle.Transactions)
				s.Get("/{id}", a.handle.GetTransaction)

				s.Post("/", a.handle.CreateTransaction)
				s.Post("/{id}/restore", a.handle.RestoreTransaction)

				s.Patch("/{id}", a.handle.UpdateTransaction)
				s.Delete("/{id}", a.handle.DeleteTransaction)

			})

			s.Route("/pockets", func(s chi.Router) {

				s.Get("/", a.handle.Pockets)

				s.Post("/", a.handle.CreatePocket)
				s.Post("/{id}/restore", a.handle.RestorePocket)

				s.Patch("/{id}", a.handle.UpdatePocket)
				s.Delete("/{id}", a.handle.DeletePocket)

			})

			s.Route("/tags", func(s chi.Router) {

				s.Get("/", a.handle.GetTags)
				s.Get("/{id}/pages", a.handle.GetPagesByTag)

				s.Post("/", a.handle.CreateTag)
				s.Post("/{id}/restore", a.handle.RestoreTag)
				s.Post("/{id}/attach", a.handle.AttachTag)

				s.Patch("/{id}", a.handle.UpdateTag)

				s.Delete("/{id}", a.handle.DeleteTag)
				s.Delete("/{id}/detach", a.handle.DetachTag)

			})

			s.Route("/edges", func(s chi.Router) {
				s.Get("/", a.handle.GetEdges)
				s.Post("/", a.handle.CreateEdge)
				s.Delete("/", a.handle.DeleteEdge)
			})
		})

		// page fragment
		s.Route("/pages", func(s chi.Router) {
			s.Get("/{id}/fragment", a.handle.PageFragment)
		})

		// finance fragments
		s.Route("/fin", func(s chi.Router) {
			s.Get("/pockets", a.handle.PocketsFragment)
			s.Get("/transactions", a.handle.TransactionsFragment)
			s.Get("/trash", a.handle.TrashFragment)
			s.Get("/tags/{id}/pages", a.handle.TagPagesFragment)
		})
	})

	// static file
	s.Handle("/static/*", staticNoCache(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))))
	s.Handle("/templui/js/*",
		staticNoCache(http.StripPrefix("/templui/js/", http.FileServer(http.Dir("static/js")))),
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

func staticNoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
