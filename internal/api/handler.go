package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hellofresh/search-orchestrator/internal/infra/observability"
	"github.com/hellofresh/search-orchestrator/internal/model"
	"github.com/hellofresh/search-orchestrator/internal/orchestrator"
	"github.com/sirupsen/logrus"
)

func searchHandler(logger *logrus.Logger, orch *orchestrator.Orchestrator, metrics *observability.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req model.SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			metrics.RequestsTotal.WithLabelValues("bad_request").Inc()
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"errors": []ValidationError{{Field: "body", Message: "invalid request body"}},
			})
			return
		}

		if errs := validateRequest(req); len(errs) > 0 {
			metrics.RequestsTotal.WithLabelValues("bad_request").Inc()
			writeJSON(w, http.StatusBadRequest, map[string]any{"errors": errs})
			return
		}

		applyDefaults(&req)

		resp, err := orch.Search(r.Context(), req)
		if err != nil {
			metrics.RequestsTotal.WithLabelValues("error").Inc()
			logger.WithError(err).Error("search failed")
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		metrics.RequestsTotal.WithLabelValues("ok").Inc()
		metrics.SearchDuration.Observe(time.Since(start).Seconds())
		metrics.ResultCount.Observe(float64(len(resp.Items)))

		writeJSON(w, http.StatusOK, resp)
	}
}

func applyDefaults(req *model.SearchRequest) {
	if req.Page.Size == 0 {
		req.Page.Size = 24
	}
	if req.Sort == "" {
		req.Sort = "relevance"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
