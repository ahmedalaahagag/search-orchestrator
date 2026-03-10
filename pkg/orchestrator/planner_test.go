package orchestrator

import (
	"testing"

	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/config"
	"github.com/stretchr/testify/assert"
)

func testSearchConfig() config.SearchConfig {
	return config.SearchConfig{
		Index: "hellofresh_{market}_productsonline",
		Stages: []config.StageConfig{
			{
				Name:        "exact",
				MinimumHits: 24,
				QueryMode:   "exact",
				Fields: []config.FieldConfig{
					{Name: "title.concept", Boost: 150},
					{Name: "title.shingle", Boost: 120},
					{Name: "title.text", Boost: 100},
					{Name: "categories.concept", Boost: 140},
				},
			},
			{
				Name:           "fallback_partial",
				MinimumHits:    1,
				QueryMode:      "partial",
				OmitPercentage: 34,
				Fields: []config.FieldConfig{
					{Name: "title.concept", Boost: 150},
					{Name: "title.text", Boost: 100},
					{Name: "description.text", Boost: 20},
				},
			},
		},
		DefaultFilters: []config.FilterConfig{
			{Field: "is_addon", Operator: "eq", Value: false},
			{Field: "is_hidden", Operator: "eq", Value: false},
			{Field: "hide_on_sold_out", Operator: "eq", Value: false},
		},
		Sorts: map[string][]config.Sort{
			"relevance": {{Field: "_score", Direction: "desc"}, {Field: "id", Direction: "asc"}},
			"newest":    {{Field: "updated_at", Direction: "desc"}, {Field: "id", Direction: "asc"}},
		},
		Facets: []config.FacetConfig{
			{Field: "categories", Type: "terms", Size: 20, ExcludeSelf: true},
		},
	}
}

func TestPlanner_BuildPlan_WithQUS(t *testing.T) {
	planner := NewPlanner(testSearchConfig())

	req := model.SearchRequest{
		Query:  "cheap chicken burger",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
	}

	qus := &model.QUSAnalyzeResponse{
		NormalizedQuery: "cheap chicken burger",
		Tokens: []model.QUSToken{
			{Value: "cheap", Normalized: "cheap", Position: 0},
			{Value: "chicken", Normalized: "chicken", Position: 1},
			{Value: "burger", Normalized: "burger", Position: 2},
		},
		Filters: []model.QUSFilter{
			{Field: "price", Operator: "lt", Value: float64(10)},
		},
	}

	plan := planner.BuildPlan(req, qus)

	assert.Equal(t, "cheap chicken burger", plan.NormalizedQuery)
	assert.Equal(t, []string{"cheap", "chicken", "burger"}, plan.Tokens)
	assert.Len(t, plan.Stages, 2)
	assert.Equal(t, "exact", plan.Stages[0].Name)
	assert.Equal(t, "fallback_partial", plan.Stages[1].Name)
	assert.Len(t, plan.DefaultFilters, 3)
	assert.Len(t, plan.UserFilters, 1)
	assert.Equal(t, "price", plan.UserFilters[0].Field)
	assert.Equal(t, 24, plan.PageSize)
	assert.Len(t, plan.Sort, 2)
	assert.Equal(t, "_score", plan.Sort[0].Field)
}

func TestPlanner_BuildPlan_WithoutQUS(t *testing.T) {
	planner := NewPlanner(testSearchConfig())

	req := model.SearchRequest{
		Query:  "chicken burger",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 10},
		Sort:   "newest",
	}

	plan := planner.BuildPlan(req, nil)

	assert.Equal(t, "chicken burger", plan.NormalizedQuery)
	assert.Equal(t, []string{"chicken", "burger"}, plan.Tokens)
	assert.Len(t, plan.UserFilters, 0)
	assert.Equal(t, "updated_at", plan.Sort[0].Field)
	assert.Equal(t, "desc", plan.Sort[0].Direction)
}

func TestPlanner_FilterMerging_RequestPriority(t *testing.T) {
	planner := NewPlanner(testSearchConfig())

	req := model.SearchRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Filters: []model.RequestFilter{
			{Field: "price", Operator: "lt", Value: float64(5)},
		},
	}

	qus := &model.QUSAnalyzeResponse{
		NormalizedQuery: "chicken",
		Tokens:          []model.QUSToken{{Value: "chicken", Normalized: "chicken", Position: 0}},
		Filters: []model.QUSFilter{
			{Field: "price", Operator: "lt", Value: float64(10)},
		},
	}

	plan := planner.BuildPlan(req, qus)

	// Request filter should win over QUS filter for same field.
	assert.Len(t, plan.UserFilters, 1)
	assert.Equal(t, float64(5), plan.UserFilters[0].Value)
}

func TestPlanner_SortResolution_QUSPriority(t *testing.T) {
	planner := NewPlanner(testSearchConfig())

	req := model.SearchRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
		Sort:   "relevance",
	}

	qus := &model.QUSAnalyzeResponse{
		NormalizedQuery: "chicken",
		Tokens:          []model.QUSToken{{Value: "chicken", Normalized: "chicken", Position: 0}},
		Sort:            &model.QUSSortSpec{Field: "updated_at", Direction: "desc"},
	}

	plan := planner.BuildPlan(req, qus)

	// QUS sort should override request sort.
	assert.Equal(t, "updated_at", plan.Sort[0].Field)
	assert.Equal(t, "desc", plan.Sort[0].Direction)
}

func TestPlanner_DefaultSort(t *testing.T) {
	planner := NewPlanner(testSearchConfig())

	req := model.SearchRequest{
		Query:  "chicken",
		Locale: "en-GB",
		Market: "uk",
		Page:   model.PageRequest{Size: 24},
	}

	plan := planner.BuildPlan(req, nil)

	assert.Equal(t, "_score", plan.Sort[0].Field)
	assert.Equal(t, "desc", plan.Sort[0].Direction)
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"chicken burger", []string{"chicken", "burger"}},
		{"  spaced  out  ", []string{"spaced", "out"}},
		{"single", []string{"single"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, tokenize(tt.input))
		})
	}
}
