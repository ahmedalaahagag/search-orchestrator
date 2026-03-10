package query

import (
	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
	"github.com/ahmedalaahagag/search-orchestrator/pkg/config"
)

// BuildFacetAggregations creates OpenSearch aggregations with self-filter exclusion.
// Each facet is wrapped in a filter aggregation that applies all user filters
// EXCEPT the filter on its own field, so clicking a facet value doesn't collapse
// the count of other values in the same facet.
func BuildFacetAggregations(facets []config.FacetConfig, userFilters []model.AppliedFilter) Query {
	aggs := make(Query, len(facets))

	for _, facet := range facets {
		innerAgg := buildInnerAgg(facet)

		if facet.ExcludeSelf && len(userFilters) > 0 {
			// Self-filter exclusion: apply all user filters except this facet's field.
			otherFilters := excludeField(userFilters, facet.Field)
			if len(otherFilters) > 0 {
				aggs["agg_"+facet.Field] = Query{
					"filter": Query{
						"bool": Query{
							"filter": buildFilterClauses(otherFilters),
						},
					},
					"aggs": Query{
						facet.Field: innerAgg,
					},
				}
				continue
			}
		}

		// No self-filter exclusion needed (no user filters, or no filter on this field).
		aggs["agg_"+facet.Field] = Query{
			"aggs": Query{
				facet.Field: innerAgg,
			},
			// Use a match_all filter wrapper so the response shape is consistent.
			"filter": Query{
				"bool": Query{
					"filter": buildFilterClauses(userFilters),
				},
			},
		}
	}

	return aggs
}

func buildInnerAgg(facet config.FacetConfig) Query {
	size := facet.Size
	if size == 0 {
		size = 20
	}

	switch facet.Type {
	case "terms":
		return Query{
			"terms": Query{
				"field": facet.Field + ".keyword",
				"size":  size,
			},
		}
	default:
		return Query{
			"terms": Query{
				"field": facet.Field + ".keyword",
				"size":  size,
			},
		}
	}
}

func excludeField(filters []model.AppliedFilter, field string) []model.AppliedFilter {
	var result []model.AppliedFilter
	for _, f := range filters {
		if f.Field != field {
			result = append(result, f)
		}
	}
	return result
}
