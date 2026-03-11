# Search Orchestrator

A Go library for multi-stage search orchestration over OpenSearch with threshold-based fallback, faceted filtering, and cursor-based pagination.

## Install

```bash
go get github.com/ahmedalaahagag/search-orchestrator
```

## Quick Start

### 1. Define your search config

Create a `search.yaml` that drives all search behavior — stages, fields, boosts, filters, sorts, and facets.

```yaml
index: myapp_{market}_products  # {market} is replaced from the request

stages:
  - name: exact
    minimum_hits: 24
    query_mode: exact             # All tokens must match (bool.must)
    max_term_count: 12
    fields:
      - { name: title.concept, boost: 150 }
      - { name: title.shingle, boost: 120 }
      - { name: title.text, boost: 100 }
      - { name: tags.text, boost: 70 }

  - name: fallback_partial
    minimum_hits: 1
    query_mode: partial           # Some tokens can be omitted (bool.should)
    omit_percentage: 34
    max_term_count: 12
    fields:
      - { name: title.concept, boost: 150 }
      - { name: title.text, boost: 100 }
      - { name: description.text, boost: 20 }

default_filters:
  - { field: is_hidden, operator: eq, value: false }
  - { field: active, operator: eq, value: true }

sorts:
  relevance:
    - { field: _score, direction: desc }
    - { field: id, direction: asc }      # Tiebreaker for stable pagination
  newest:
    - { field: updated_at, direction: desc }
    - { field: id, direction: asc }

facets:
  - { field: category, type: terms, size: 20, exclude_self: true }
  - { field: brand, type: terms, size: 20, exclude_self: true }
```

### 2. Wire up in your service

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/ahmedalaahagag/search-orchestrator/pkg/config"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/model"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/observability"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/opensearch"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/orchestrator"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()

    // Load search config.
    searchCfg, err := config.LoadSearchConfig("configs/search.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Create OpenSearch client.
    osClient := opensearch.NewClient(opensearch.ClientConfig{
        URL:      "http://localhost:9200",
        Username: "admin",
        Password: "admin",
        Timeout:  5 * time.Second,
    })

    // Create orchestrator.
    metrics := observability.NewMetrics()
    planner := orchestrator.NewPlanner(searchCfg)
    orch := orchestrator.New(logger, metrics, osClient, planner, searchCfg)

    // Search.
    resp, err := orch.Search(context.Background(),
        model.SearchRequest{
            Query:  "chicken",
            Locale: "en-US",
            Market: "us",
            Page:   model.PageRequest{Size: 24},
            Sort:   "relevance",
        },
        nil, // no QUS — uses whitespace tokenization
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d items (stage: %s)", resp.Meta.TotalHits, resp.Meta.Stage)
}
```

### 3. Pagination

Use the cursor from the response to fetch the next page:

```go
// First page
resp, _ := orch.Search(ctx, model.SearchRequest{
    Query:  "chicken",
    Locale: "en-US",
    Market: "us",
    Page:   model.PageRequest{Size: 24},
    Sort:   "relevance",
}, nil)

// Next page
if resp.Page.HasNextPage {
    resp, _ = orch.Search(ctx, model.SearchRequest{
        Query:  "chicken",
        Locale: "en-US",
        Market: "us",
        Page:   model.PageRequest{Size: 24, Cursor: resp.Page.Cursor},
        Sort:   "relevance",
    }, nil)
}
```

### 4. User filters

Pass runtime filters from the user (e.g. facet selections):

```go
resp, _ := orch.Search(ctx, model.SearchRequest{
    Query:  "chicken",
    Locale: "en-US",
    Market: "us",
    Page:   model.PageRequest{Size: 24},
    Sort:   "relevance",
    Filters: []model.RequestFilter{
        {Field: "category", Operator: "eq", Value: "meals"},
        {Field: "brand", Operator: "in", Value: []string{"classic", "gourmet"}},
    },
}, nil)
```

User filters are applied as `post_filter` so they don't affect facet counts.

### 5. QUS integration (optional)

If you have a Query Understanding Service, pass its output to influence tokenization, filters, and sort:

```go
qusResult := &model.QUSAnalyzeResponse{
    NormalizedQuery: "chicken burger",
    Tokens: []model.QUSToken{
        {Value: "chicken", Normalized: "chicken", Position: 0},
        {Value: "burger", Normalized: "burger", Position: 1},
    },
    Filters: []model.QUSFilter{
        {Field: "tags", Operator: "eq", Value: "budget-friendly"},
    },
}

resp, err := orch.Search(ctx, req, qusResult)
```

When `nil` is passed, the orchestrator tokenizes the raw query by whitespace.

## How It Works

Stages execute sequentially. Each builds an OpenSearch query from the tokenized input and configured fields. The first stage that meets its `minimum_hits` threshold wins. If none do, the last stage's results are returned.

```
Request + QUS → Planner → SearchPlan → Stage 1 (exact) → 3 hits (need 24) → skip
                                      → Stage 2 (partial) → 47 hits (need 1) → return
```

**Query strategy per token:** `dis_max` across all configured fields — each field gets an individual `match` query with its boost weight.

**Filter operators:** `eq` (term), `in` (terms), `gt`/`gte`/`lt`/`lte` (range).

## Packages

| Package | Description |
|---|---|
| `pkg/config` | Load and parse `search.yaml` |
| `pkg/model` | Domain types: request, response, search plan, QUS types |
| `pkg/orchestrator` | Core orchestration with multi-stage fallback |
| `pkg/query` | OpenSearch DSL builder |
| `pkg/opensearch` | OpenSearch HTTP client |
| `pkg/observability` | Prometheus metrics |

## Standalone Server

The repo includes a standalone HTTP server for development and testing.

```bash
make docker-up   # Start OpenSearch
make seed        # Create indexes + seed data
make run         # Start server on :8081
```

```bash
curl -X POST http://localhost:8081/v1/search \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "chicken",
    "locale": "en-US",
    "market": "us",
    "page": { "size": 24 },
    "sort": "relevance"
  }'
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SEARCH_HTTP_PORT` | `8081` | HTTP server port |
| `SEARCH_OPENSEARCH_URL` | `http://localhost:9200` | OpenSearch endpoint |
| `SEARCH_OPENSEARCH_USERNAME` | | OpenSearch username |
| `SEARCH_OPENSEARCH_PASSWORD` | | OpenSearch password |
| `SEARCH_OPENSEARCH_TIMEOUT` | `5s` | Request timeout |
| `SEARCH_CONFIG_DIR` | `configs` | Path to search config directory |

## Testing

```bash
make test
# or
go test ./... -v -race
```
