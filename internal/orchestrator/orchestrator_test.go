package orchestrator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hellofresh/search-orchestrator/internal/infra/observability"
	"github.com/hellofresh/search-orchestrator/internal/infra/opensearch"
	"github.com/hellofresh/search-orchestrator/internal/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQUSClient is a test double for QUSClient.
type mockQUSClient struct {
	resp *model.QUSAnalyzeResponse
	err  error
}

func (m *mockQUSClient) Analyze(_ context.Context, _ model.QUSAnalyzeRequest) (*model.QUSAnalyzeResponse, error) {
	return m.resp, m.err
}

// mockOSClient is a test double for OpenSearchClient.
type mockOSClient struct {
	responses []*opensearch.SearchResponse
	callCount int
}

func (m *mockOSClient) Search(_ context.Context, _ string, _ []byte) (*opensearch.SearchResponse, error) {
	if m.callCount >= len(m.responses) {
		return &opensearch.SearchResponse{}, nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func newTestOrchestrator(qusClient *mockQUSClient, osClient *mockOSClient) *Orchestrator {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	metrics := observability.NewMetrics()
	cfg := testSearchConfig()
	planner := NewPlanner(cfg)
	return New(logger, metrics, qusClient, osClient, planner, cfg)
}

func makeHits(count int) []opensearch.Hit {
	hits := make([]opensearch.Hit, count)
	for i := 0; i < count; i++ {
		id := string(rune('A' + i))
		hits[i] = opensearch.Hit{
			ID:     id,
			Score:  float64(count - i),
			Source: json.RawMessage(`{"id":"` + id + `","title":"Item ` + id + `"}`),
			Sort:   []any{float64(count - i), id},
		}
	}
	return hits
}

func TestOrchestrator_ExactStageSufficient(t *testing.T) {
	qus := &mockQUSClient{
		resp: &model.QUSAnalyzeResponse{
			NormalizedQuery: "chicken burger",
			Tokens: []model.QUSToken{
				{Value: "chicken", Normalized: "chicken", Position: 0},
				{Value: "burger", Normalized: "burger", Position: 1},
			},
		},
	}

	os := &mockOSClient{
		responses: []*opensearch.SearchResponse{
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 30}, Hits: makeHits(24)}},
		},
	}

	orch := newTestOrchestrator(qus, os)
	resp, err := orch.Search(context.Background(), model.SearchRequest{
		Query:  "chicken burger",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
	})

	require.NoError(t, err)
	assert.Equal(t, "exact", resp.Meta.Stage)
	assert.Equal(t, 30, resp.Meta.TotalHits)
	assert.Len(t, resp.Items, 24)
	assert.Equal(t, 1, os.callCount, "should only call OS once when exact stage is sufficient")
}

func TestOrchestrator_FallbackTriggered(t *testing.T) {
	qus := &mockQUSClient{
		resp: &model.QUSAnalyzeResponse{
			NormalizedQuery: "chicken burger",
			Tokens: []model.QUSToken{
				{Value: "chicken", Normalized: "chicken", Position: 0},
				{Value: "burger", Normalized: "burger", Position: 1},
			},
		},
	}

	os := &mockOSClient{
		responses: []*opensearch.SearchResponse{
			// Stage 1: below threshold (12)
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 5}, Hits: makeHits(5)}},
			// Stage 2: meets threshold (1)
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 8}, Hits: makeHits(8)}},
		},
	}

	orch := newTestOrchestrator(qus, os)
	resp, err := orch.Search(context.Background(), model.SearchRequest{
		Query:  "chicken burger",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
	})

	require.NoError(t, err)
	assert.Equal(t, "fallback_partial", resp.Meta.Stage)
	assert.Equal(t, 8, resp.Meta.TotalHits)
	assert.Equal(t, 2, os.callCount, "should call OS twice for fallback")
}

func TestOrchestrator_BothStagesEmpty(t *testing.T) {
	qus := &mockQUSClient{
		resp: &model.QUSAnalyzeResponse{
			NormalizedQuery: "nonexistent",
			Tokens:          []model.QUSToken{{Value: "nonexistent", Normalized: "nonexistent", Position: 0}},
		},
	}

	os := &mockOSClient{
		responses: []*opensearch.SearchResponse{
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 0}, Hits: nil}},
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 0}, Hits: nil}},
		},
	}

	orch := newTestOrchestrator(qus, os)
	resp, err := orch.Search(context.Background(), model.SearchRequest{
		Query:  "nonexistent",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
	})

	require.NoError(t, err)
	assert.Equal(t, "fallback_partial", resp.Meta.Stage)
	assert.Equal(t, 0, resp.Meta.TotalHits)
	assert.Empty(t, resp.Items)
}

func TestOrchestrator_QUSFailure_DegradedMode(t *testing.T) {
	qus := &mockQUSClient{
		err: assert.AnError,
	}

	os := &mockOSClient{
		responses: []*opensearch.SearchResponse{
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 30}, Hits: makeHits(24)}},
		},
	}

	orch := newTestOrchestrator(qus, os)
	resp, err := orch.Search(context.Background(), model.SearchRequest{
		Query:  "chicken burger",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
	})

	require.NoError(t, err)
	assert.Equal(t, "exact", resp.Meta.Stage)
	assert.Contains(t, resp.Meta.Warnings, "QUS unavailable, using raw query")
	assert.Equal(t, 30, resp.Meta.TotalHits)
}

func TestOrchestrator_Pagination_HasNextPage(t *testing.T) {
	qus := &mockQUSClient{
		resp: &model.QUSAnalyzeResponse{
			NormalizedQuery: "chicken",
			Tokens:          []model.QUSToken{{Value: "chicken", Normalized: "chicken", Position: 0}},
		},
	}

	os := &mockOSClient{
		responses: []*opensearch.SearchResponse{
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 50}, Hits: makeHits(10)}},
		},
	}

	orch := newTestOrchestrator(qus, os)
	resp, err := orch.Search(context.Background(), model.SearchRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 10},
		Sort:   "relevance",
	})

	require.NoError(t, err)
	assert.True(t, resp.Page.HasNextPage)
	assert.NotEmpty(t, resp.Page.Cursor)
}

func TestOrchestrator_Pagination_NoNextPage(t *testing.T) {
	qus := &mockQUSClient{
		resp: &model.QUSAnalyzeResponse{
			NormalizedQuery: "chicken",
			Tokens:          []model.QUSToken{{Value: "chicken", Normalized: "chicken", Position: 0}},
		},
	}

	os := &mockOSClient{
		responses: []*opensearch.SearchResponse{
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 5}, Hits: makeHits(5)}},
		},
	}

	orch := newTestOrchestrator(qus, os)
	resp, err := orch.Search(context.Background(), model.SearchRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
	})

	require.NoError(t, err)
	assert.False(t, resp.Page.HasNextPage)
}

func TestOrchestrator_UserFiltersApplied(t *testing.T) {
	qus := &mockQUSClient{
		resp: &model.QUSAnalyzeResponse{
			NormalizedQuery: "chicken",
			Tokens:          []model.QUSToken{{Value: "chicken", Normalized: "chicken", Position: 0}},
		},
	}

	os := &mockOSClient{
		responses: []*opensearch.SearchResponse{
			{Hits: opensearch.Hits{Total: opensearch.TotalHits{Value: 30}, Hits: makeHits(24)}},
		},
	}

	orch := newTestOrchestrator(qus, os)
	resp, err := orch.Search(context.Background(), model.SearchRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
		Filters: []model.RequestFilter{
			{Field: "categories", Operator: "eq", Value: "burgers"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, 30, resp.Meta.TotalHits)
}
