package model

type SearchRequest struct {
	Query   string          `json:"query"`
	Locale  string          `json:"locale"`
	Market  string          `json:"market"`
	Page    PageRequest     `json:"page"`
	Sort    string          `json:"sort"`
	Filters []RequestFilter `json:"filters,omitempty"`
}

type PageRequest struct {
	Size   int    `json:"size"`
	Cursor string `json:"cursor,omitempty"`
}

type RequestFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}
