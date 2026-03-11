# Problems & Solutions

A log of significant problems encountered during development and how they were resolved.

---

## 1. Tight Coupling: Orchestrator Owned the QUS Client

**Problem:** The orchestrator originally created and managed its own QUS (Query Understanding Service) HTTP client internally. This meant:
- The orchestrator had a hard dependency on a specific QUS endpoint and transport
- Callers couldn't use their own QUS client, mock it, or skip it
- Testing required mocking an HTTP client the orchestrator owned
- The orchestrator mixed two concerns: search execution and query understanding

**Solution:** Removed the QUS client entirely from the orchestrator in two steps:
1. Changed `Search()` to accept `*model.QUSAnalyzeResponse` as an input parameter instead of calling QUS internally (`3576c78`)
2. Deleted the `pkg/qus` package entirely (`0c0dc97`)

Now callers bring their own QUS. Pass `nil` to skip QUS and use raw whitespace tokenization. This follows dependency inversion — the orchestrator depends on a data contract (the QUS response struct), not a transport implementation.

---

## 2. Internal Packages Couldn't Be Imported by Other Services

**Problem:** All core packages (`model`, `orchestrator`, `query`, `opensearch`) lived under `internal/`, making them inaccessible to the sibling `search-service` that needed to import the orchestrator as a library.

**Solution:** Moved all reusable packages from `internal/` to `pkg/` (`5e7e4eb`, `23f5d22`):
- `internal/model` → `pkg/model`
- `internal/orchestrator` → `pkg/orchestrator`
- `internal/query` → `pkg/query`
- `internal/infra/opensearch` → `pkg/opensearch`
- `internal/infra/observability` → `pkg/observability`

Kept `internal/api` (HTTP handlers) as internal — only needed by the standalone server. Also renamed the module to `github.com/ahmedalaahagag/search-orchestrator` to make it a proper importable Go module.

---

## 3. Range Filters Broke on Keyword Fields (Lexicographic vs. Numeric Comparison)

**Problem:** Range filters (`gt`, `gte`, `lt`, `lte`) used standard OpenSearch `range` queries:
```json
{ "range": { "price": { "gte": 2000 } } }
```
But some index fields (e.g. price) were stored as `keyword` type. OpenSearch compares keywords lexicographically, so `"249" < "2000"` evaluated to `false` because `"2" < "2"` is false and `"4" > "0"`. This caused filters like "price >= 2000" to silently return wrong results.

**Solution:** Replaced `range` queries with painless script filters that parse keyword values as doubles at query time (`41e1935`):
```json
{
  "script": {
    "script": {
      "source": "doc['price'].size() > 0 && Double.parseDouble(doc['price'].value) >= params.val",
      "params": { "val": 2000 },
      "lang": "painless"
    }
  }
}
```
The `doc['field'].size() > 0` guard prevents errors on documents missing the field. This trades some query performance for correctness — acceptable since these filters are not high-frequency and the alternative (reindexing all keyword fields as numeric) was a much larger change.

---

## 4. Search Config Diverged from Production

**Problem:** The initial `search.yaml` was a simplified placeholder that didn't match the production search-adapter-service configuration. Differences included:
- Missing fields (only searched `title` and `tags`, missed `categories`, `ingredients`, `recipe_cuisine`, `headline`)
- Wrong boost values (didn't reflect tuned production weights)
- Missing analyzer sub-fields (production uses `.concept`, `.shingle`, `.text` per field)
- Wrong default filters (had generic `is_hidden`/`active`, production uses `is_addon`/`is_hidden`/`hide_on_sold_out`)
- Included price-based sorts that don't apply to recipe data
- Static index name instead of dynamic `hellofresh_{market}_productsonline`

**Solution:** Aligned the entire config with the production search-adapter-service (`3a25c5a`):
- Added all searchable fields with three analyzer sub-fields each
- Matched production boost values across 17 field entries per stage
- Updated default filters to match production (`is_addon`, `is_hidden`, `hide_on_sold_out`)
- Removed price sorts, kept `relevance` and `newest`
- Dynamic index pattern with `{market}` placeholder
- Updated response model to include recipe-specific fields (`description`, `headline`, `slug`, `imageUrl`, `categories`, `tags`, `allergens`, `ingredients`)

---

## 5. Insufficient Observability for Debugging Search Behavior

**Problem:** When search results were unexpected, there was no way to understand what the orchestrator was doing internally:
- Which tokens were extracted from the query
- Which stage was being executed and why
- What the search plan looked like (filters, sort, market)
- What OpenSearch query body was actually sent

**Solution:** Added structured logging at key decision points (`5c0b483`, `48a30ca`):
- **QUS analysis result:** Logs token count, concept count, filter count, and whether QUS provided a sort override
- **Search plan:** Logs tokens, filter counts (default vs. user), sort config, stage count, and market
- **Stage execution:** Logs stage name, target index, tokens, and query mode before each OpenSearch call
- **Stage result:** Logs hit count vs. minimum threshold after each stage
- **OpenSearch request body:** Logged at DEBUG level to avoid noise in production

This follows the pattern of logging at decision boundaries — not every line, but every point where the orchestrator makes a choice.

---

## 6. No Local Development Environment

**Problem:** Developers had to point at a shared or production OpenSearch cluster to test anything. No way to run the system locally end-to-end.

**Solution:** Added a complete local dev setup (`3a25c5a`):
- `docker-compose.yaml` with single-node OpenSearch (no security plugin for simplicity)
- `scripts/setup-index.sh` that creates indexes with production-equivalent mappings for US, CA, and GB markets
- Compressed seed data files (`.ndjson.gz`) for each market, gitignored to keep the repo lean
- `Makefile` targets: `make docker-up`, `make seed`, `make run`
