package model

// QueryAnalysis holds pre-processed query understanding results.
// Callers produce this from any source (QUS, custom NLP, manual input).
// The orchestrator is agnostic to the upstream system.
type QueryAnalysis struct {
	NormalizedQuery string          `json:"normalizedQuery"`
	Tokens          []string        `json:"tokens"`
	Filters         []AppliedFilter `json:"filters,omitempty"`
	Sort            string          `json:"sort,omitempty"` // sort key name matching search.yaml sorts (e.g. "newest")
	Warnings        []string        `json:"warnings,omitempty"`
}
