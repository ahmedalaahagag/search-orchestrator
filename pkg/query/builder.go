package query

import (
	"math"

	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/config"
)

// Query is the OpenSearch DSL represented as nested maps.
type Query = map[string]any

// BuildStageQuery constructs the text-match portion of the OpenSearch query for a stage.
func BuildStageQuery(tokens []string, stage model.SearchStage) Query {
	if stage.MaxTermCount > 0 && len(tokens) > stage.MaxTermCount {
		tokens = tokens[:stage.MaxTermCount]
	}

	if len(tokens) == 0 {
		return Query{"match_all": Query{}}
	}

	if stage.QueryMode == "partial" && stage.OmitPercentage > 0 {
		return partialMatchQuery(tokens, stage.Fields, stage.OmitPercentage)
	}

	return mustAllQuery(tokens, stage.Fields)
}

// mustAllQuery requires every token to match at least one field via dis_max.
func mustAllQuery(tokens []string, fields []config.FieldConfig) Query {
	must := make([]any, 0, len(tokens))
	for _, token := range tokens {
		must = append(must, tokenDisMax(token, fields))
	}
	return Query{
		"bool": Query{
			"must": must,
		},
	}
}

// partialMatchQuery allows some tokens to not match based on omit percentage.
func partialMatchQuery(tokens []string, fields []config.FieldConfig, omitPct int) Query {
	maxOmit := int(math.Floor(float64(len(tokens)) * float64(omitPct) / 100.0))
	minMatch := len(tokens) - maxOmit

	if minMatch >= len(tokens) {
		return mustAllQuery(tokens, fields)
	}

	should := make([]any, 0, len(tokens))
	for _, token := range tokens {
		should = append(should, tokenDisMax(token, fields))
	}

	return Query{
		"bool": Query{
			"should":               should,
			"minimum_should_match": minMatch,
		},
	}
}

// tokenDisMax creates a dis_max query across all configured fields for a single token.
func tokenDisMax(token string, fields []config.FieldConfig) Query {
	queries := make([]any, 0, len(fields))
	for _, f := range fields {
		queries = append(queries, Query{
			"match": Query{
				f.Name: Query{
					"query": token,
					"boost": f.Boost,
				},
			},
		})
	}
	return Query{
		"dis_max": Query{
			"queries": queries,
		},
	}
}

// WrapWithFilters adds default filter clauses to a query in bool.filter.
func WrapWithFilters(q Query, filters []model.AppliedFilter) Query {
	if len(filters) == 0 {
		return q
	}

	filterClauses := buildFilterClauses(filters)

	return Query{
		"bool": Query{
			"must":   q,
			"filter": filterClauses,
		},
	}
}

// BuildPostFilter creates a post_filter from user filters (for facet correctness).
func BuildPostFilter(filters []model.AppliedFilter) Query {
	if len(filters) == 0 {
		return nil
	}

	clauses := buildFilterClauses(filters)
	return Query{
		"bool": Query{
			"filter": clauses,
		},
	}
}

// BuildSort creates the sort array for OpenSearch.
func BuildSort(specs []model.SortSpec) []any {
	sorts := make([]any, 0, len(specs))
	for _, s := range specs {
		sorts = append(sorts, Query{
			s.Field: Query{
				"order": s.Direction,
			},
		})
	}
	return sorts
}

// BuildFullRequest assembles the complete OpenSearch request body.
func BuildFullRequest(q Query, plan model.SearchPlan) (Query, error) {
	body := Query{
		"query": WrapWithFilters(q, plan.DefaultFilters),
		"size":  plan.PageSize,
		"sort":  BuildSort(plan.Sort),
	}

	if postFilter := BuildPostFilter(plan.UserFilters); postFilter != nil {
		body["post_filter"] = postFilter
	}

	if len(plan.Facets) > 0 {
		body["aggs"] = BuildFacetAggregations(plan.Facets, plan.UserFilters)
	}

	if len(plan.SearchAfter) > 0 {
		body["search_after"] = plan.SearchAfter
	}

	return body, nil
}

func buildFilterClauses(filters []model.AppliedFilter) []any {
	clauses := make([]any, 0, len(filters))
	for _, f := range filters {
		clauses = append(clauses, buildFilterClause(f))
	}
	return clauses
}

func buildFilterClause(f model.AppliedFilter) Query {
	switch f.Operator {
	case "eq":
		return Query{"term": Query{f.Field: f.Value}}
	case "in":
		return Query{"terms": Query{f.Field: f.Value}}
	case "gt", "gte", "lt", "lte":
		return buildRangeFilter(f)
	default:
		return Query{"term": Query{f.Field: f.Value}}
	}
}

// buildRangeFilter builds a script-based numeric range filter.
// Index fields may be stored as keyword strings (e.g. price "249"),
// so we parse them to double in a painless script for correct comparison.
func buildRangeFilter(f model.AppliedFilter) Query {
	var op string
	switch f.Operator {
	case "gt":
		op = ">"
	case "gte":
		op = ">="
	case "lt":
		op = "<"
	case "lte":
		op = "<="
	}

	// Strip non-numeric suffixes (e.g. "560 kcal" → "560", "1800" → "1800").
	// Use indexOf/substring since painless doesn't whitelist String.split().
	field := f.Field
	source := "if (doc['" + field + "'].size() == 0) return false; def v = doc['" + field + "'].value.trim(); def i = v.indexOf(' '); Double.parseDouble(i > 0 ? v.substring(0, i) : v) " + op + " params.val"

	return Query{
		"script": Query{
			"script": Query{
				"source": source,
				"params": Query{"val": f.Value},
				"lang":   "painless",
			},
		},
	}
}
