package opensearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Search_Success(t *testing.T) {
	osResp := SearchResponse{
		Hits: Hits{
			Total: TotalHits{Value: 42, Relation: "eq"},
			Hits: []Hit{
				{
					ID:     "1",
					Score:  5.5,
					Source: json.RawMessage(`{"id":"1","title":"Chicken Burger"}`),
					Sort:   []any{5.5, "1"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/products/_search", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(osResp)
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{URL: srv.URL, Timeout: 5 * time.Second})

	body := []byte(`{"query":{"match_all":{}},"size":10}`)
	resp, err := client.Search(context.Background(), "products", body)

	require.NoError(t, err)
	assert.Equal(t, 42, resp.Hits.Total.Value)
	assert.Len(t, resp.Hits.Hits, 1)
	assert.Equal(t, "1", resp.Hits.Hits[0].ID)
	assert.Equal(t, 5.5, resp.Hits.Hits[0].Score)
}

func TestClient_Search_BasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "admin", user)
		assert.Equal(t, "secret", pass)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResponse{Hits: Hits{Total: TotalHits{Value: 0}}})
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{
		URL:      srv.URL,
		Username: "admin",
		Password: "secret",
		Timeout:  5 * time.Second,
	})

	_, err := client.Search(context.Background(), "products", []byte(`{}`))
	require.NoError(t, err)
}

func TestClient_Search_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"cluster unavailable"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientConfig{URL: srv.URL, Timeout: 5 * time.Second})

	_, err := client.Search(context.Background(), "products", []byte(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}
