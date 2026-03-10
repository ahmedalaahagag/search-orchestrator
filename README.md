# Search Orchestrator

A Go library for multi-stage search orchestration over OpenSearch with threshold-based fallback, faceted filtering, and cursor-based pagination.

Can be used as an importable library in your own service or run as a standalone HTTP server.

## Installation

```bash
go get github.com/ahmedalaahagag/search-orchestrator
```

## Using as a Library

### 1. Create a search config file

The library is driven by a `search.yaml` config file that defines how search queries are built and executed.

```yaml
# search.yaml

# Index name pattern. {market} is replaced at runtime from the request.
index: myapp_{market}_products

# Search stages execute sequentially. The first stage that meets
# minimum_hits stops execution. If none meet the threshold, the
# last stage's results are returned.
stages:
  - name: exact
    minimum_hits: 24          # Minimum results to accept this stage
    query_mode: exact         # All tokens must match
    max_term_count: 12        # Truncate query beyond this many tokens
    fields:
      # Each field uses OpenSearch sub-field analyzers.
      # .concept = keyword analyzer (exact phrase match)
      # .shingle = shingle analyzer (phrase proximity)
      # .text    = stemmed text analyzer (broad match)
      - { name: title.concept, boost: 150 }
      - { name: title.shingle, boost: 120 }
      - { name: title.text, boost: 100 }
      - { name: tags.text, boost: 70 }

  - name: fallback_partial
    minimum_hits: 1
    query_mode: partial       # Allows some tokens to not match
    omit_percentage: 34       # Up to 34% of tokens can be omitted
    max_term_count: 12
    fields:
      - { name: title.concept, boost: 150 }
      - { name: title.text, boost: 100 }
      - { name: description.text, boost: 20 }

# Filters applied to every query (bool.filter clause).
# Operators: eq, in, gt, gte, lt, lte
default_filters:
  - { field: is_hidden, operator: eq, value: false }
  - { field: active, operator: eq, value: true }

# Named sort configurations. Each sort is a list of fields.
# The tiebreaker (e.g. id) ensures stable pagination.
sorts:
  relevance:
    - { field: _score, direction: desc }
    - { field: id, direction: asc }
  newest:
    - { field: updated_at, direction: desc }
    - { field: id, direction: asc }

# Facet aggregations returned with search results.
# exclude_self: true means clicking a facet value won't collapse
# other values in the same facet.
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

    "github.com/ahmedalaahagag/search-orchestrator/pkg/config"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/model"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/observability"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/opensearch"
    "github.com/ahmedalaahagag/search-orchestrator/pkg/orchestrator"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()

    // 1. Load search config from YAML.
    searchCfg, err := config.LoadSearchConfig("configs/search.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // 2. Create OpenSearch client.
    osClient := opensearch.NewClient(opensearch.ClientConfig{
        URL:      "http://localhost:9200",
        Username: "admin",
        Password: "admin",
        Timeout:  5 * time.Second,
    })

    // 3. Create orchestrator.
    metrics := observability.NewMetrics()
    planner := orchestrator.NewPlanner(searchCfg)
    orch := orchestrator.New(logger, metrics, osClient, planner, searchCfg)

    // 4. Execute a search.
    resp, err := orch.Search(context.Background(),
        model.SearchRequest{
            Query:  "chicken",
            Locale: "en-US",
            Market: "us",
            Page:   model.PageRequest{Size: 24},
            Sort:   "relevance",
        },
        nil, // optional *model.QUSAnalyzeResponse from your own QUS
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d items (stage: %s)", resp.Meta.TotalHits, resp.Meta.Stage)
}
```

### 3. Bring your own QUS (optional)

The orchestrator accepts an optional `*model.QUSAnalyzeResponse` as the second argument to `Search()`. If you have your own Query Understanding Service, pass its output to influence tokenization, filters, and sort:

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

If `nil` is passed, the orchestrator uses the raw query string with simple whitespace tokenization.

## Packages

| Package | Import Path | Description |
|---|---|---|
| **config** | `pkg/config` | Load and parse `search.yaml` configuration |
| **model** | `pkg/model` | Domain types: `SearchRequest`, `SearchResponse`, `SearchPlan`, QUS types |
| **orchestrator** | `pkg/orchestrator` | Core search orchestration with multi-stage fallback |
| **query** | `pkg/query` | OpenSearch DSL builder (queries, filters, facets, pagination) |
| **opensearch** | `pkg/opensearch` | OpenSearch HTTP client and response types |
| **observability** | `pkg/observability` | Prometheus metrics and structured logging |
| **qus** | `pkg/qus` | HTTP client for external QUS services |

## Search Config Reference

### `index`

Index name pattern. Use `{market}` as a placeholder — it gets replaced with the lowercase `market` value from the search request.

```yaml
index: myapp_{market}_products
# Request with market: "us" → searches index "myapp_us_products"
```

### `stages`

Stages execute sequentially. Each stage builds an OpenSearch query from the tokenized input and configured fields. If a stage returns >= `minimum_hits`, it's accepted and execution stops.

| Field | Type | Description |
|---|---|---|
| `name` | string | Stage identifier (returned in response metadata) |
| `minimum_hits` | int | Minimum results to accept this stage |
| `query_mode` | string | `exact` (all tokens must match) or `partial` (allows omission) |
| `omit_percentage` | int | Max % of tokens that can be omitted (partial mode only) |
| `max_term_count` | int | Truncate query tokens beyond this count |
| `fields` | list | Fields to search with boost values |

**Query modes:**
- `exact` — Every token must match at least one field. Built as `bool.must` with `dis_max` per token across all fields.
- `partial` — Allows up to `omit_percentage`% of tokens to not match. Built as `bool.should` with `minimum_should_match`.

### `fields`

Each field entry is a `name` + `boost` pair. The `name` should reference an OpenSearch field path including the sub-field analyzer:

- `.concept` — keyword/concept analyzer (highest precision)
- `.shingle` — shingle analyzer (phrase proximity matching)
- `.text` — stemmed text analyzer (broadest recall)

Higher boost = more weight when that field matches.

### `default_filters`

Filters applied to every query as `bool.filter` clauses. These are hard requirements (not scoring).

| Operator | OpenSearch Query |
|---|---|
| `eq` | `term` |
| `in` | `terms` |
| `gt`, `gte`, `lt`, `lte` | `range` |

### `sorts`

Named sort configurations referenced by the `sort` field in search requests. Each sort is a list of `{field, direction}` pairs. Always include a tiebreaker field (e.g. `id`) for stable cursor pagination.

### `facets`

Facet aggregations returned alongside search results.

| Field | Type | Description |
|---|---|---|
| `field` | string | Document field to aggregate on |
| `type` | string | Aggregation type (`terms`) |
| `size` | int | Max buckets to return (default: 20) |
| `exclude_self` | bool | Exclude this facet's own filter from its aggregation |

When `exclude_self: true`, clicking a facet value doesn't collapse other values in the same facet — each facet sees all results except its own filter.

## Running as a Standalone Server

The repo also includes a standalone HTTP server for development and testing.

### Prerequisites

- Go 1.26+
- Docker (for local OpenSearch)

### Quick Start

```bash
# Start OpenSearch
make docker-up

# Create indexes and seed sample data
make seed

# Start the server
make run
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SEARCH_HTTP_PORT` | `8081` | HTTP server port |
| `SEARCH_OPENSEARCH_URL` | `http://localhost:9200` | OpenSearch endpoint |
| `SEARCH_OPENSEARCH_USERNAME` | | OpenSearch username |
| `SEARCH_OPENSEARCH_PASSWORD` | | OpenSearch password |
| `SEARCH_OPENSEARCH_TIMEOUT` | `5s` | OpenSearch request timeout |
| `SEARCH_QUS_URL` | `http://localhost:8080` | QUS endpoint |
| `SEARCH_QUS_TIMEOUT` | `3s` | QUS request timeout |
| `SEARCH_CONFIG_DIR` | `configs` | Path to search config directory |

### API

#### `POST /v1/search`

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

**Required fields:** `query`, `locale`, `market`

**Optional fields:**
- `page.size` (1-100, default: 24)
- `page.cursor` (opaque cursor from previous response for pagination)
- `sort` (must match a key in `sorts` config — default: `relevance`)
- `filters` (array of `{ "field": "...", "operator": "...", "value": ... }`)

#### `GET /healthz`

Returns `{"status": "ok"}`.

#### `GET /metrics`

Prometheus metrics endpoint.

## Testing

```bash
go test ./... -v -race
```
