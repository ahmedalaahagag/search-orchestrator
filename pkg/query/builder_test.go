package query

import (
	"encoding/json"
	"testing"

	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testFields = []config.FieldConfig{
	{Name: "title", Boost: 5.0},
	{Name: "brand", Boost: 3.0},
}

func TestBuildStageQuery_ExactMode_SingleToken(t *testing.T) {
	stage := model.SearchStage{
		Name:      "exact",
		QueryMode: "exact",
		Fields:    testFields,
	}

	q := BuildStageQuery([]string{"chicken"}, stage)
	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	boolQ := result["bool"].(map[string]any)
	must := boolQ["must"].([]any)
	assert.Len(t, must, 1)

	disMax := must[0].(map[string]any)["dis_max"].(map[string]any)
	queries := disMax["queries"].([]any)
	assert.Len(t, queries, 2)
}

func TestBuildStageQuery_ExactMode_MultipleTokens(t *testing.T) {
	stage := model.SearchStage{
		Name:      "exact",
		QueryMode: "exact",
		Fields:    testFields,
	}

	q := BuildStageQuery([]string{"chicken", "burger"}, stage)
	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	boolQ := result["bool"].(map[string]any)
	must := boolQ["must"].([]any)
	assert.Len(t, must, 2)
}

func TestBuildStageQuery_PartialMode(t *testing.T) {
	stage := model.SearchStage{
		Name:           "partial",
		QueryMode:      "partial",
		OmitPercentage: 34,
		Fields:         testFields,
	}

	// 3 tokens, 34% = floor(3*0.34)=1 can be omitted, so minMatch = 2
	q := BuildStageQuery([]string{"cheap", "chicken", "burger"}, stage)
	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	boolQ := result["bool"].(map[string]any)
	should := boolQ["should"].([]any)
	assert.Len(t, should, 3)
	assert.Equal(t, float64(2), boolQ["minimum_should_match"])
}

func TestBuildStageQuery_PartialMode_SingleToken_FallsBackToMust(t *testing.T) {
	stage := model.SearchStage{
		Name:           "partial",
		QueryMode:      "partial",
		OmitPercentage: 34,
		Fields:         testFields,
	}

	// 1 token, 34% = floor(1*0.34)=0 omitted, minMatch=1 == len(tokens), falls back to must
	q := BuildStageQuery([]string{"chicken"}, stage)
	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	boolQ := result["bool"].(map[string]any)
	_, hasMust := boolQ["must"]
	assert.True(t, hasMust, "single token partial should fall back to must")
}

func TestBuildStageQuery_EmptyTokens(t *testing.T) {
	stage := model.SearchStage{
		Name:      "exact",
		QueryMode: "exact",
		Fields:    testFields,
	}

	q := BuildStageQuery(nil, stage)
	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Contains(t, result, "match_all")
}

func TestBuildStageQuery_MaxTermCount(t *testing.T) {
	stage := model.SearchStage{
		Name:         "exact",
		QueryMode:    "exact",
		MaxTermCount: 2,
		Fields:       testFields,
	}

	q := BuildStageQuery([]string{"cheap", "chicken", "burger"}, stage)
	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	boolQ := result["bool"].(map[string]any)
	must := boolQ["must"].([]any)
	assert.Len(t, must, 2) // truncated to MaxTermCount
}

func TestWrapWithFilters(t *testing.T) {
	inner := Query{"match_all": Query{}}
	filters := []model.AppliedFilter{
		{Field: "hidden", Operator: "eq", Value: false},
		{Field: "availability", Operator: "in", Value: []string{"in_stock"}},
	}

	q := WrapWithFilters(inner, filters)
	raw, err := json.Marshal(q)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	boolQ := result["bool"].(map[string]any)
	assert.Contains(t, boolQ, "must")
	assert.Contains(t, boolQ, "filter")

	filterArr := boolQ["filter"].([]any)
	assert.Len(t, filterArr, 2)
}

func TestWrapWithFilters_Empty(t *testing.T) {
	inner := Query{"match_all": Query{}}
	q := WrapWithFilters(inner, nil)
	assert.Equal(t, inner, q)
}

func TestBuildPostFilter(t *testing.T) {
	filters := []model.AppliedFilter{
		{Field: "category", Operator: "eq", Value: "burgers"},
	}

	pf := BuildPostFilter(filters)
	raw, err := json.Marshal(pf)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	boolQ := result["bool"].(map[string]any)
	filterArr := boolQ["filter"].([]any)
	assert.Len(t, filterArr, 1)
}

func TestBuildPostFilter_Empty(t *testing.T) {
	pf := BuildPostFilter(nil)
	assert.Nil(t, pf)
}

func TestBuildSort(t *testing.T) {
	specs := []model.SortSpec{
		{Field: "_score", Direction: "desc"},
		{Field: "id.keyword", Direction: "asc"},
	}

	sorts := BuildSort(specs)
	assert.Len(t, sorts, 2)

	raw, err := json.Marshal(sorts)
	require.NoError(t, err)

	var result []map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))

	assert.Contains(t, result[0], "_score")
	assert.Contains(t, result[1], "id.keyword")
}

func TestBuildFilterClause_Operators(t *testing.T) {
	tests := []struct {
		name     string
		filter   model.AppliedFilter
		wantKey  string
	}{
		{"eq", model.AppliedFilter{Field: "status", Operator: "eq", Value: "active"}, "term"},
		{"in", model.AppliedFilter{Field: "tags", Operator: "in", Value: []string{"a", "b"}}, "terms"},
		{"gt", model.AppliedFilter{Field: "price", Operator: "gt", Value: 10}, "range"},
		{"gte", model.AppliedFilter{Field: "price", Operator: "gte", Value: 10}, "range"},
		{"lt", model.AppliedFilter{Field: "price", Operator: "lt", Value: 20}, "range"},
		{"lte", model.AppliedFilter{Field: "price", Operator: "lte", Value: 20}, "range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause := buildFilterClause(tt.filter)
			assert.Contains(t, clause, tt.wantKey)
		})
	}
}
