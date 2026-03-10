package query

import (
	"encoding/json"
	"testing"

	"github.com/hellofresh/search-orchestrator/internal/model"
	"github.com/hellofresh/search-orchestrator/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFacetAggregations_Basic(t *testing.T) {
	facets := []config.FacetConfig{
		{Field: "category", Type: "terms", Size: 20, ExcludeSelf: true},
		{Field: "brand", Type: "terms", Size: 20, ExcludeSelf: true},
	}

	aggs := BuildFacetAggregations(facets, nil)
	assert.Len(t, aggs, 2)
	assert.Contains(t, aggs, "agg_category")
	assert.Contains(t, aggs, "agg_brand")
}

func TestBuildFacetAggregations_SelfExclusion(t *testing.T) {
	facets := []config.FacetConfig{
		{Field: "category", Type: "terms", Size: 20, ExcludeSelf: true},
		{Field: "brand", Type: "terms", Size: 20, ExcludeSelf: true},
	}

	userFilters := []model.AppliedFilter{
		{Field: "category", Operator: "eq", Value: "burgers"},
		{Field: "brand", Operator: "eq", Value: "acme"},
	}

	aggs := BuildFacetAggregations(facets, userFilters)

	raw, err := json.Marshal(aggs)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	// The category agg should exclude the category filter (only have brand filter).
	catAgg := result["agg_category"].(map[string]any)
	catFilter := catAgg["filter"].(map[string]any)
	catBool := catFilter["bool"].(map[string]any)
	catFilterArr := catBool["filter"].([]any)
	assert.Len(t, catFilterArr, 1, "category agg should only have brand filter")

	// The brand agg should exclude the brand filter (only have category filter).
	brandAgg := result["agg_brand"].(map[string]any)
	brandFilter := brandAgg["filter"].(map[string]any)
	brandBool := brandFilter["bool"].(map[string]any)
	brandFilterArr := brandBool["filter"].([]any)
	assert.Len(t, brandFilterArr, 1, "brand agg should only have category filter")
}

func TestBuildFacetAggregations_NoUserFilters(t *testing.T) {
	facets := []config.FacetConfig{
		{Field: "category", Type: "terms", Size: 20, ExcludeSelf: true},
	}

	aggs := BuildFacetAggregations(facets, nil)

	raw, err := json.Marshal(aggs)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	catAgg := result["agg_category"].(map[string]any)
	assert.Contains(t, catAgg, "aggs")
	assert.Contains(t, catAgg, "filter")
}

func TestBuildFacetAggregations_DefaultSize(t *testing.T) {
	facets := []config.FacetConfig{
		{Field: "category", Type: "terms", Size: 0, ExcludeSelf: false},
	}

	aggs := BuildFacetAggregations(facets, nil)
	raw, err := json.Marshal(aggs)
	require.NoError(t, err)

	// Should default to size 20.
	assert.Contains(t, string(raw), `"size":20`)
}
