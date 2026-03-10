package api

import "github.com/hellofresh/search-orchestrator/internal/model"

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

var validSorts = map[string]bool{
	"relevance":  true,
	"price_asc":  true,
	"price_desc": true,
	"newest":     true,
}

func validateRequest(req model.SearchRequest) []ValidationError {
	var errs []ValidationError

	if req.Query == "" {
		errs = append(errs, ValidationError{Field: "query", Message: "query is required"})
	}

	if req.Locale == "" {
		errs = append(errs, ValidationError{Field: "locale", Message: "locale is required"})
	}

	if req.Market == "" {
		errs = append(errs, ValidationError{Field: "market", Message: "market is required"})
	}

	if req.Page.Size < 0 || req.Page.Size > 100 {
		errs = append(errs, ValidationError{Field: "page.size", Message: "page.size must be between 1 and 100"})
	}

	if req.Sort != "" && !validSorts[req.Sort] {
		errs = append(errs, ValidationError{Field: "sort", Message: "sort must be one of: relevance, price_asc, price_desc, newest"})
	}

	return errs
}
