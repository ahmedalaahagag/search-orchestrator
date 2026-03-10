package api

import (
	"testing"

	"github.com/hellofresh/search-orchestrator/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      model.SearchRequest
		wantErrs int
		fields   []string
	}{
		{
			name:     "valid request",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk"},
			wantErrs: 0,
		},
		{
			name:     "empty query",
			req:      model.SearchRequest{Locale: "en-GB", Market: "uk"},
			wantErrs: 1,
			fields:   []string{"query"},
		},
		{
			name:     "empty locale",
			req:      model.SearchRequest{Query: "chicken", Market: "uk"},
			wantErrs: 1,
			fields:   []string{"locale"},
		},
		{
			name:     "empty market",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB"},
			wantErrs: 1,
			fields:   []string{"market"},
		},
		{
			name:     "all empty",
			req:      model.SearchRequest{},
			wantErrs: 3,
			fields:   []string{"query", "locale", "market"},
		},
		{
			name:     "page size too large",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk", Page: model.PageRequest{Size: 101}},
			wantErrs: 1,
			fields:   []string{"page.size"},
		},
		{
			name:     "page size negative",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk", Page: model.PageRequest{Size: -1}},
			wantErrs: 1,
			fields:   []string{"page.size"},
		},
		{
			name:     "invalid sort",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk", Sort: "invalid"},
			wantErrs: 1,
			fields:   []string{"sort"},
		},
		{
			name:     "invalid sort price_asc",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk", Sort: "price_asc"},
			wantErrs: 1,
			fields:   []string{"sort"},
		},
		{
			name:     "valid sort newest",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk", Sort: "newest"},
			wantErrs: 0,
		},
		{
			name:     "valid page size 1",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk", Page: model.PageRequest{Size: 1}},
			wantErrs: 0,
		},
		{
			name:     "valid page size 100",
			req:      model.SearchRequest{Query: "chicken", Locale: "en-GB", Market: "uk", Page: model.PageRequest{Size: 100}},
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateRequest(tt.req)
			assert.Len(t, errs, tt.wantErrs)

			if tt.wantErrs > 0 {
				errFields := make([]string, len(errs))
				for i, e := range errs {
					errFields[i] = e.Field
				}
				for _, f := range tt.fields {
					assert.Contains(t, errFields, f)
				}
			}
		})
	}
}
