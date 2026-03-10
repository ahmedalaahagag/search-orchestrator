package opensearch

import "encoding/json"

type SearchResponse struct {
	Hits         Hits                       `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations,omitempty"`
}

type Hits struct {
	Total TotalHits `json:"total"`
	Hits  []Hit     `json:"hits"`
}

type TotalHits struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"`
}

type Hit struct {
	Index  string          `json:"_index"`
	ID     string          `json:"_id"`
	Score  float64         `json:"_score"`
	Source json.RawMessage `json:"_source"`
	Sort   []any           `json:"sort,omitempty"`
}

type AggResult struct {
	Buckets []AggBucket `json:"buckets"`
}

type AggBucket struct {
	Key      string `json:"key"`
	DocCount int    `json:"doc_count"`
}

type FilterAggResult struct {
	DocCount int                        `json:"doc_count"`
	Inner    map[string]json.RawMessage `json:"-"`
}
