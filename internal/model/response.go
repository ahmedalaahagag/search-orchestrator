package model

type SearchResponse struct {
	Items    []SearchItem  `json:"items"`
	Facets   []FacetResult `json:"facets,omitempty"`
	Page     PageInfo      `json:"page"`
	Meta     SearchMeta    `json:"meta"`
}

type SearchItem struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Score        float64 `json:"score"`
	Price        float64 `json:"price,omitempty"`
	Availability string  `json:"availability,omitempty"`
	Brand        string  `json:"brand,omitempty"`
	Category     string  `json:"category,omitempty"`
}

type FacetResult struct {
	Field   string       `json:"field"`
	Buckets []FacetBucket `json:"buckets"`
}

type FacetBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type PageInfo struct {
	Size        int    `json:"size"`
	HasNextPage bool   `json:"hasNextPage"`
	Cursor      string `json:"cursor,omitempty"`
}

type SearchMeta struct {
	TotalHits int      `json:"totalHits"`
	Stage     string   `json:"stage"`
	Warnings  []string `json:"warnings,omitempty"`
}
