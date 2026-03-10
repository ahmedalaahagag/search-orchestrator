# Search Orchestrator

A search orchestration service that sits between clients and OpenSearch, providing query understanding, multi-stage search with threshold-based fallback, faceted filtering, and cursor-based pagination.

## Architecture

```
Client -> [HTTP API] -> [Orchestrator] -> [QUS] (query understanding)
                                       -> [OpenSearch] (search execution)
```

The orchestrator follows a pipeline:

1. **QUS Call** - Sends the query to the Query Understanding Service for normalization, tokenization, and inferred filters. Falls back gracefully if QUS is unavailable.
2. **Plan Building** - Builds a search plan with stages, filters, sort, and pagination from config + QUS output.
3. **Stage Execution** - Executes search stages sequentially (e.g. `exact` then `fallback_partial`), stopping at the first stage that meets the minimum hit threshold.
4. **Response Mapping** - Maps OpenSearch hits to a clean response with items, facets, pagination cursors, and metadata.

## Getting Started

### Prerequisites

- Go 1.24+
- OpenSearch instance
- QUS service (optional, degrades gracefully)

### Configuration

Copy the example env file and fill in your values:

```bash
cp .env.example .env
```

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

Search behavior (stages, facets, sorts, default filters) is configured in `configs/search.yaml`.

### Run

```bash
go run . http
```

### Test

```bash
go test ./...
```

## API

### `POST /v1/search`

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
- `page.cursor` (for pagination)
- `sort` (`relevance`, `price_asc`, `price_desc`, `newest` — default: `relevance`)
- `filters` (array of `{ "field": "...", "operator": "...", "value": ... }`)

### `GET /healthz`

Returns `{"status": "ok"}`.

### `GET /metrics`

Prometheus metrics endpoint.
