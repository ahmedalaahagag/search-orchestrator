package model

import "github.com/ahmedalaahagag/search-orchestrator/pkg/config"

type SearchPlan struct {
	NormalizedQuery string
	Tokens          []string
	Market          string
	Stages          []SearchStage
	DefaultFilters  []AppliedFilter
	UserFilters     []AppliedFilter
	Sort            []SortSpec
	Facets          []config.FacetConfig
	PageSize        int
	SearchAfter     []any
}

type SearchStage struct {
	Name           string
	QueryMode      string // "exact" or "partial"
	MinimumHits    int
	OmitPercentage int
	MaxTermCount   int
	Fields         []config.FieldConfig
}

type AppliedFilter struct {
	Field    string
	Operator string
	Value    any
}

type SortSpec struct {
	Field     string
	Direction string
}
