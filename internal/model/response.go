package model

type SearchResponse struct {
	Items    []SearchItem  `json:"items"`
	Facets   []FacetResult `json:"facets,omitempty"`
	Page     PageInfo      `json:"page"`
	Meta     SearchMeta    `json:"meta"`
}

type SearchItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Score       float64  `json:"score"`
	Description string   `json:"description,omitempty"`
	Headline    string   `json:"headline,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	ImageURL    string   `json:"imageUrl,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Allergens   []string `json:"allergens,omitempty"`
	Ingredients []string `json:"ingredients,omitempty"`
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
