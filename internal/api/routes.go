package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/observability"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/orchestrator"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func NewRouter(logger *logrus.Logger, orch *orchestrator.Orchestrator, metrics *observability.Metrics) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(correlationIDLogger(logger))

	r.Get("/healthz", healthHandler())
	r.Get("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}).ServeHTTP)
	r.Post("/v1/search", searchHandler(logger, orch, metrics))

	return r
}

func correlationIDLogger(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := middleware.GetReqID(r.Context())
			if reqID != "" {
				logger.WithField("request_id", reqID)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
