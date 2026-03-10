package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hellofresh/search-orchestrator/internal/infra/observability"
	"github.com/hellofresh/search-orchestrator/internal/model"
	"github.com/hellofresh/search-orchestrator/internal/orchestrator"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRouter(orch *orchestrator.Orchestrator) http.Handler {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	metrics := observability.NewMetrics()
	return NewRouter(logger, orch, metrics)
}

func TestHealthEndpoint(t *testing.T) {
	router := testRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestSearchEndpoint_InvalidJSON(t *testing.T) {
	router := testRouter(&orchestrator.Orchestrator{})

	req := httptest.NewRequest(http.MethodPost, "/v1/search", bytes.NewBufferString(`{invalid`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchEndpoint_MissingFields(t *testing.T) {
	router := testRouter(&orchestrator.Orchestrator{})

	body, _ := json.Marshal(model.SearchRequest{})
	req := httptest.NewRequest(http.MethodPost, "/v1/search", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errors := resp["errors"].([]any)
	assert.Len(t, errors, 3) // query, locale, market
}

func TestSearchEndpoint_InvalidSort(t *testing.T) {
	router := testRouter(&orchestrator.Orchestrator{})

	body, _ := json.Marshal(model.SearchRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
		Sort:   "invalid_sort",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/search", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMetricsEndpoint(t *testing.T) {
	router := testRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "search_qus_failures_total")
}
