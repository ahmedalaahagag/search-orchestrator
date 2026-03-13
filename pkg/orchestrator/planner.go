package orchestrator

import (
	"github.com/ahmedalaahagag/search-orchestrator/pkg/config"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/query"
)

type Planner struct {
	cfg config.SearchConfig
}

func NewPlanner(cfg config.SearchConfig) *Planner {
	return &Planner{cfg: cfg}
}

func (p *Planner) BuildPlan(req model.SearchRequest, analysis *model.QueryAnalysis) model.SearchPlan {
	plan := model.SearchPlan{
		PageSize: req.Page.Size,
		Market:   req.Market,
	}

	// Use analysis tokens if available, otherwise whitespace-tokenize the raw query.
	if analysis != nil && analysis.NormalizedQuery != "" {
		plan.NormalizedQuery = analysis.NormalizedQuery
		plan.Tokens = analysis.Tokens
	} else {
		plan.NormalizedQuery = req.Query
		plan.Tokens = tokenize(req.Query)
	}

	// Build stages from config.
	for _, sc := range p.cfg.Stages {
		plan.Stages = append(plan.Stages, model.SearchStage{
			Name:           sc.Name,
			QueryMode:      sc.QueryMode,
			MinimumHits:    sc.MinimumHits,
			OmitPercentage: sc.OmitPercentage,
			MaxTermCount:   sc.MaxTermCount,
			Fields:         sc.Fields,
		})
	}

	// Merge filters: explicit request > analysis-inferred > defaults.
	// RequiredFilters (e.g. week, menu_key) go into DefaultFilters so they
	// restrict hit counts and stage fallback, not just post_filter.
	plan.DefaultFilters = defaultFilters(p.cfg.DefaultFilters)
	for _, f := range req.RequiredFilters {
		plan.DefaultFilters = append(plan.DefaultFilters, model.AppliedFilter{
			Field: f.Field, Operator: f.Operator, Value: f.Value,
		})
	}
	plan.UserFilters = mergeUserFilters(req.Filters, analysis)

	// Resolve sort.
	plan.Sort = p.resolveSort(req.Sort, analysis)

	// Attach facet config.
	plan.Facets = p.cfg.Facets

	// Decode cursor for pagination.
	if req.Page.Cursor != "" {
		plan.SearchAfter = query.DecodeCursor(req.Page.Cursor)
	}

	return plan
}

func tokenize(query string) []string {
	if query == "" {
		return nil
	}
	var tokens []string
	start := 0
	for i := 0; i <= len(query); i++ {
		if i == len(query) || query[i] == ' ' {
			if i > start {
				tokens = append(tokens, query[start:i])
			}
			start = i + 1
		}
	}
	return tokens
}

func defaultFilters(cfgFilters []config.FilterConfig) []model.AppliedFilter {
	filters := make([]model.AppliedFilter, 0, len(cfgFilters))
	for _, f := range cfgFilters {
		filters = append(filters, model.AppliedFilter{
			Field:    f.Field,
			Operator: f.Operator,
			Value:    f.Value,
		})
	}
	return filters
}

func mergeUserFilters(reqFilters []model.RequestFilter, analysis *model.QueryAnalysis) []model.AppliedFilter {
	seen := make(map[string]bool)
	var filters []model.AppliedFilter

	// Explicit request filters take priority.
	for _, f := range reqFilters {
		seen[f.Field] = true
		filters = append(filters, model.AppliedFilter{
			Field:    f.Field,
			Operator: f.Operator,
			Value:    f.Value,
		})
	}

	// Analysis-inferred filters (skip duplicates).
	if analysis != nil {
		for _, f := range analysis.Filters {
			if seen[f.Field] {
				continue
			}
			seen[f.Field] = true
			filters = append(filters, f)
		}
	}

	return filters
}

func (p *Planner) resolveSort(reqSort string, analysis *model.QueryAnalysis) []model.SortSpec {
	// Analysis sort takes priority if present.
	if analysis != nil && analysis.Sort != "" {
		if specs, ok := p.cfg.Sorts[analysis.Sort]; ok {
			return toSortSpecs(specs)
		}
	}

	// Fall back to request sort.
	sortKey := reqSort
	if sortKey == "" {
		sortKey = "relevance"
	}

	if specs, ok := p.cfg.Sorts[sortKey]; ok {
		return toSortSpecs(specs)
	}

	// Ultimate fallback.
	return []model.SortSpec{
		{Field: "_score", Direction: "desc"},
		{Field: "id", Direction: "asc"},
	}
}

func toSortSpecs(sorts []config.Sort) []model.SortSpec {
	specs := make([]model.SortSpec, 0, len(sorts))
	for _, s := range sorts {
		specs = append(specs, model.SortSpec{
			Field:     s.Field,
			Direction: s.Direction,
		})
	}
	return specs
}
