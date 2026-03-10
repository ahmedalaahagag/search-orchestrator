package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmedalaahagag/search-orchestrator/internal/infra/observability"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/opensearch"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/query"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type goldenTestCase struct {
	Name     string              `json:"name"`
	Input    goldenInput         `json:"input"`
	QUS      *goldenQUS          `json:"qus"`
	OS       []goldenOSResponse  `json:"os"`
	Expected goldenExpected      `json:"expected"`
}

type goldenInput struct {
	Query   string               `json:"query"`
	Locale  string               `json:"locale"`
	Market  string               `json:"market"`
	Sort    string               `json:"sort"`
	Page    model.PageRequest    `json:"page"`
	Filters []model.RequestFilter `json:"filters,omitempty"`
}

type goldenQUS struct {
	Available bool                     `json:"available"`
	Response  *model.QUSAnalyzeResponse `json:"response,omitempty"`
}

type goldenOSResponse struct {
	TotalHits int              `json:"totalHits"`
	Hits      []goldenHit      `json:"hits"`
}

type goldenHit struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

type goldenExpected struct {
	Stage      string   `json:"stage"`
	TotalHits  int      `json:"totalHits"`
	ItemCount  int      `json:"itemCount"`
	HasNextPage bool    `json:"hasNextPage"`
	HasCursor  bool     `json:"hasCursor"`
	HasWarnings bool    `json:"hasWarnings"`
}

func loadGoldenTests(t *testing.T) []goldenTestCase {
	t.Helper()
	pattern := filepath.Join("..", "..", "testdata", "golden", "search_*.json")
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no golden test files found matching %s", pattern)

	var cases []goldenTestCase
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)

		var tc goldenTestCase
		require.NoError(t, json.Unmarshal(data, &tc), "parsing %s", f)
		cases = append(cases, tc)
	}
	return cases
}

func goldenOSClient(responses []goldenOSResponse) *mockOSClient {
	osResponses := make([]*opensearch.SearchResponse, len(responses))
	for i, r := range responses {
		hits := make([]opensearch.Hit, len(r.Hits))
		for j, h := range r.Hits {
			source, _ := json.Marshal(map[string]any{
				"id":    h.ID,
				"title": h.Title,
			})
			hits[j] = opensearch.Hit{
				ID:     h.ID,
				Score:  h.Score,
				Source: source,
				Sort:   []any{h.Score, h.ID},
			}
		}
		osResponses[i] = &opensearch.SearchResponse{
			Hits: opensearch.Hits{
				Total: opensearch.TotalHits{Value: r.TotalHits},
				Hits:  hits,
			},
		}
	}
	return &mockOSClient{responses: osResponses}
}

func goldenQUSClient(g *goldenQUS) *mockQUSClient {
	if g == nil || !g.Available {
		return &mockQUSClient{err: assert.AnError}
	}
	return &mockQUSClient{resp: g.Response}
}

func TestGoldenBehavior(t *testing.T) {
	cases := loadGoldenTests(t)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			qusClient := goldenQUSClient(tc.QUS)
			osClient := goldenOSClient(tc.OS)

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)
			metrics := observability.NewMetrics()
			cfg := testSearchConfig()
			planner := NewPlanner(cfg)
			orch := New(logger, metrics, qusClient, osClient, planner, cfg)

			req := model.SearchRequest{
				Query:   tc.Input.Query,
				Locale:  tc.Input.Locale,
				Market:  tc.Input.Market,
				Sort:    tc.Input.Sort,
				Page:    tc.Input.Page,
				Filters: tc.Input.Filters,
			}

			if req.Page.Size == 0 {
				req.Page.Size = 24
			}
			if req.Sort == "" {
				req.Sort = "relevance"
			}

			resp, err := orch.Search(context.Background(), req)
			require.NoError(t, err)

			assert.Equal(t, tc.Expected.Stage, resp.Meta.Stage)
			assert.Equal(t, tc.Expected.TotalHits, resp.Meta.TotalHits)
			assert.Equal(t, tc.Expected.ItemCount, len(resp.Items))
			assert.Equal(t, tc.Expected.HasNextPage, resp.Page.HasNextPage)

			if tc.Expected.HasCursor {
				assert.NotEmpty(t, resp.Page.Cursor)
				// Verify cursor is decodable.
				decoded := query.DecodeCursor(resp.Page.Cursor)
				assert.NotNil(t, decoded)
			}

			if tc.Expected.HasWarnings {
				assert.NotEmpty(t, resp.Meta.Warnings)
			}
		})
	}
}
