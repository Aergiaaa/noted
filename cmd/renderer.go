package main

import (
	"github.com/Aergiaaa/noted/ui/pages"
	"net/http"
)

func (a *App) renderError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	w.WriteHeader(code)
	pages.Error(code, msg).Render(r.Context(), w)
}

func (app *App) renderIndex(w http.ResponseWriter, r *http.Request) {
	pages.Landing().Render(r.Context(), w)
}
