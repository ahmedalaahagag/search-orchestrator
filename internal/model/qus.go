package model

type QUSAnalyzeRequest struct {
	Query  string `json:"query"`
	Locale string `json:"locale"`
	Market string `json:"market"`
}

type QUSAnalyzeResponse struct {
	OriginalQuery   string         `json:"originalQuery"`
	NormalizedQuery string         `json:"normalizedQuery"`
	Tokens          []QUSToken     `json:"tokens"`
	Concepts        []QUSConcept   `json:"concepts"`
	Filters         []QUSFilter    `json:"filters"`
	Sort            *QUSSortSpec   `json:"sort,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
}

type QUSToken struct {
	Value      string `json:"value"`
	Normalized string `json:"normalized"`
	Position   int    `json:"position"`
}

type QUSConcept struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	MatchedText string  `json:"matchedText"`
	Field       string  `json:"field,omitempty"`
	Score       float64 `json:"score"`
	Source      string  `json:"source"`
	Start       int     `json:"start"`
	End         int     `json:"end"`
}

type QUSFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type QUSSortSpec struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}
