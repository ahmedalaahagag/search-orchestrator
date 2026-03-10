package orchestrator

import (
	"github.com/hellofresh/search-orchestrator/internal/model"
	"github.com/hellofresh/search-orchestrator/internal/query"
	"github.com/hellofresh/search-orchestrator/pkg/config"
)

type Planner struct {
	cfg config.SearchConfig
}

func NewPlanner(cfg config.SearchConfig) *Planner {
	return &Planner{cfg: cfg}
}

func (p *Planner) BuildPlan(req model.SearchRequest, qus *model.QUSAnalyzeResponse) model.SearchPlan {
	plan := model.SearchPlan{
		PageSize: req.Page.Size,
		Market:   req.Market,
	}

	// Use QUS normalized query if available, otherwise raw query.
	if qus != nil && qus.NormalizedQuery != "" {
		plan.NormalizedQuery = qus.NormalizedQuery
		plan.Tokens = extractTokens(qus)
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

	// Merge filters: explicit request > QUS-inferred > defaults.
	plan.DefaultFilters = defaultFilters(p.cfg.DefaultFilters)
	plan.UserFilters = mergeUserFilters(req.Filters, qus)

	// Resolve sort.
	plan.Sort = p.resolveSort(req.Sort, qus)

	// Attach facet config.
	plan.Facets = p.cfg.Facets

	// Decode cursor for pagination.
	if req.Page.Cursor != "" {
		plan.SearchAfter = query.DecodeCursor(req.Page.Cursor)
	}

	return plan
}

func extractTokens(qus *model.QUSAnalyzeResponse) []string {
	tokens := make([]string, 0, len(qus.Tokens))
	for _, t := range qus.Tokens {
		tokens = append(tokens, t.Normalized)
	}
	return tokens
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

func mergeUserFilters(reqFilters []model.RequestFilter, qus *model.QUSAnalyzeResponse) []model.AppliedFilter {
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

	// QUS-inferred filters (skip duplicates).
	if qus != nil {
		for _, f := range qus.Filters {
			if seen[f.Field] {
				continue
			}
			seen[f.Field] = true
			filters = append(filters, model.AppliedFilter{
				Field:    f.Field,
				Operator: f.Operator,
				Value:    f.Value,
			})
		}
	}

	return filters
}

func (p *Planner) resolveSort(reqSort string, qus *model.QUSAnalyzeResponse) []model.SortSpec {
	// QUS sort takes priority if present.
	if qus != nil && qus.Sort != nil {
		sortKey := qusSortToKey(qus.Sort)
		if specs, ok := p.cfg.Sorts[sortKey]; ok {
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

func qusSortToKey(s *model.QUSSortSpec) string {
	switch {
	case s.Field == "updated_at" && s.Direction == "desc":
		return "newest"
	default:
		return "relevance"
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
