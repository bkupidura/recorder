package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter creates http router.
func NewRouter(opts *Options) *chi.Mux {
	httpRouter := chi.NewRouter()

	httpRouter.Group(func(r chi.Router) {
		r.Use(middleware.CleanPath)
		r.Use(middleware.Recoverer)
		r.Get("/ready", healthHandler(opts.WorkingPools))
		r.Get("/healthz", healthHandler(opts.WorkingPools))
		r.Method("GET", "/metrics", promhttp.Handler())
	})
	httpRouter.Group(func(r chi.Router) {
		r.Use(middleware.CleanPath)
		r.Use(middleware.Recoverer)
		if len(opts.AuthUsers) > 0 {
			r.Use(middleware.BasicAuth("api", opts.AuthUsers))
		} else {
			log.Printf("auth for HTTP API disabled")
		}

		r.Mount("/debug", middleware.Profiler())

		recordings := http.FileServer(http.Dir(opts.RecordingPath))
		r.Handle("/", http.RedirectHandler("/recordings/", http.StatusMovedPermanently))
		r.Handle("/recordings/*", http.StripPrefix("/recordings/", recordings))

		r.Post("/api/record", recordHandler(opts.WorkingPools["record"]))
	})

	return httpRouter
}
