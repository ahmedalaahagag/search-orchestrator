package qus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hellofresh/search-orchestrator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Analyze_Success(t *testing.T) {
	expected := &model.QUSAnalyzeResponse{
		OriginalQuery:   "cheap chicken burger",
		NormalizedQuery: "cheap chicken burger",
		Tokens: []model.QUSToken{
			{Value: "cheap", Normalized: "cheap", Position: 0},
			{Value: "chicken", Normalized: "chicken", Position: 1},
			{Value: "burger", Normalized: "burger", Position: 2},
		},
		Filters: []model.QUSFilter{
			{Field: "price", Operator: "lt", Value: float64(10)},
		},
		Sort: &model.QUSSortSpec{Field: "price", Direction: "asc"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/analyze", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req model.QUSAnalyzeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "cheap chicken burger", req.Query)
		assert.Equal(t, "en-GB", req.Locale)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})

	resp, err := client.Analyze(context.Background(), model.QUSAnalyzeRequest{
		Query:  "cheap chicken burger",
		Locale: "en-GB",
		Market: "uk",
	})

	require.NoError(t, err)
	assert.Equal(t, expected.NormalizedQuery, resp.NormalizedQuery)
	assert.Len(t, resp.Tokens, 3)
	assert.Len(t, resp.Filters, 1)
	assert.NotNil(t, resp.Sort)
}

func TestClient_Analyze_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{BaseURL: srv.URL, Timeout: 50 * time.Millisecond})

	_, err := client.Analyze(context.Background(), model.QUSAnalyzeRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
	})

	assert.Error(t, err)
}

func TestClient_Analyze_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})

	_, err := client.Analyze(context.Background(), model.QUSAnalyzeRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_Analyze_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid`))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})

	_, err := client.Analyze(context.Background(), model.QUSAnalyzeRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
	})

	assert.Error(t, err)
}
