# Search Orchestrator - Project Instructions

## What this is
A Go library (`pkg/`) for multi-stage search orchestration over OpenSearch. Can also run as a standalone HTTP server (`cmd/` + `internal/`).

## Module
`github.com/ahmedalaahagag/search-orchestrator` — Go 1.26.1

## Architecture

### Separation of concerns
- **Orchestrator** = search only. Builds OpenSearch DSL, executes multi-stage fallback, pagination, facets, sorting.
- **Query analysis** = NOT our concern. The orchestrator accepts `*model.QueryAnalysis` as input. Callers map their upstream (QUS, custom NLP, etc.) to `QueryAnalysis` before calling `Search()`. Pass `nil` for whitespace tokenization.

### Package layout
```
pkg/                     # Public — importable by other services
  config/                # SearchConfig from YAML
  model/                 # Domain types (SearchRequest, SearchResponse, SearchPlan, QueryAnalysis)
  opensearch/            # OpenSearch HTTP client + response types
  orchestrator/          # Core orchestration (Orchestrator + Planner)
  query/                 # OpenSearch DSL builder (queries, filters, facets, pagination)
  observability/         # Prometheus metrics
internal/                # Private — standalone server only
  api/                   # HTTP handlers, routes, validation
cmd/                     # CLI entry point (cobra)
configs/                 # search.yaml
scripts/                 # setup-index.sh, seed data
```

### Key types
- `orchestrator.New(logger, metrics, osClient, planner, cfg)` — constructor
- `orchestrator.Search(ctx, req, analysis)` — main entry point, analysis is optional (*nil* = raw query with whitespace tokenization)
- `planner.BuildPlan(req, analysis)` — builds SearchPlan from request + query analysis
- `query.BuildStageQuery(tokens, stage)` — per-token `dis_max` across fields
- `query.BuildFullRequest(stageQuery, plan)` — assembles complete OpenSearch request body

### Query strategy
- Per token: `dis_max` across all configured fields (individual `match` queries with boost)
- Exact mode: `bool.must` — every token must match
- Partial mode: `bool.should` with `minimum_should_match` based on `omit_percentage`
- Filters: `bool.filter` for defaults, `post_filter` for user facet filters
- No fuzzy, no synonyms, no KNN — those belong upstream or to a separate concern

## Config
- `configs/search.yaml` — drives all search behavior (stages, fields, boosts, filters, sorts, facets)
- Index pattern: `hellofresh_{market}_productsonline`
- Analyzer sub-fields: `.concept` (keyword), `.shingle` (phrase proximity), `.text` (stemmed)
- Env vars prefixed with `SEARCH_` (envconfig)

## Local development
- `make docker-up` — starts OpenSearch
- `make seed` — creates indexes and loads seed data (compressed ndjson in scripts/)
- `make run` — starts standalone HTTP server
- `make test` — runs all tests

## Testing
- Golden tests in `pkg/orchestrator/` with fixtures in `testdata/golden/`
- Mock OpenSearch client for unit tests
- `go test ./... -v -race`

## Git
- Remote: `github.com/ahmedalaahagag/search-orchestrator`
- Branch: `main`
- Seed data files (.ndjson.gz) are gitignored
