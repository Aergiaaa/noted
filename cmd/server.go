package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func (app *app) serve() error {
	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Starting server on %s", s.Addr)

	return s.ListenAndServe()
}
