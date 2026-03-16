# Problems & Solutions

A log of significant problems encountered during development and how they were resolved.

---

## 1. Tight Coupling: Orchestrator Was Coupled to QUS

**Problem:** The orchestrator originally had a hard dependency on the QUS (Query Understanding Service). This evolved through two phases:

**Phase 1 — Owned the QUS client:** The orchestrator created and managed its own QUS HTTP client internally. Callers couldn't use their own client, mock it, or skip it. Testing required mocking an HTTP client the orchestrator owned.

**Phase 2 — Used QUS-specific types:** After removing the HTTP client, `Search()` still accepted `*model.QUSAnalyzeResponse` with QUS-specific types (`QUSToken`, `QUSConcept`, `QUSFilter`, `QUSSortSpec`). Callers had to construct QUS types even without a QUS service. The `pkg/config` and `pkg/observability` packages also contained QUS-specific config and metrics.

**Solution:** Decoupled in three steps:
1. Changed `Search()` to accept `*model.QUSAnalyzeResponse` instead of calling QUS internally (`3576c78`)
2. Deleted the `pkg/qus` package entirely (`0c0dc97`)
3. Replaced all QUS-specific types with a generic `*model.QueryAnalysis` (`d3b3a0e`, v0.3.0):
   - `QueryAnalysis{NormalizedQuery, Tokens []string, Filters []AppliedFilter, Sort string, Warnings}`
   - Removed `QUSConfig` from `pkg/config/` and QUS metrics from `pkg/observability/`
   - Callers map their upstream (QUS, custom NLP, etc.) → `QueryAnalysis` before calling `Search()`

Pass `nil` to skip analysis and use raw whitespace tokenization. The orchestrator is now fully agnostic to the upstream query understanding system.

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
- **Query analysis result:** Logs token count, filter count, and whether analysis provided a sort override
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

---

## 7. Painless Script Limitations for Range Filters on Keyword Fields with Suffixes

**Problem:** Some keyword fields store values with non-numeric suffixes (e.g. `total_calories: "560 kcal"`). The initial painless script used `Double.parseDouble(doc['field'].value)` which threw a `NumberFormatException` on these values. Two fix attempts failed:
1. `String.replaceAll()` — throws `Cannot cast java.lang.String to java.util.function.Function` due to painless casting issues
2. `String.split()` — throws `dynamic method [java.lang.String, split/1] not found` because painless doesn't whitelist `split`

**Solution:** Used `indexOf` + `substring` to strip suffixes before parsing (`v0.2.3`):
```painless
if (doc['field'].size() == 0) return false;
def v = doc['field'].value.trim();
def i = v.indexOf(' ');
Double.parseDouble(i > 0 ? v.substring(0, i) : v) >= params.val
```
This handles both clean values (`"249"`) and suffixed values (`"560 kcal"`) by splitting at the first space. The `size() == 0` guard prevents errors on documents missing the field.

**Lesson:** Painless is a restricted subset of Java — always check the [painless API whitelist](https://www.elastic.co/guide/en/elasticsearch/painless/current/painless-api-reference.html) before assuming standard `String` methods are available.

---

## 8. Go RE2 Regex: Negative Lookahead Silently Fails

**Problem:** Comprehension rules for price extraction used negative lookahead to avoid matching time/calorie patterns:
```regex
(under|less than)\s+(\d+)(?!\s*(minutes?|mins?|cal))
```
Go's `regexp` package uses RE2, which does NOT support lookahead (`(?!...)` or `(?=...)`). When `regexp.Compile()` fails, the comprehension engine logged a warning and skipped the rule — but since all price rules used lookahead, no price filters were ever extracted.

**Solution:** Removed negative lookahead entirely. Instead, rely on rule ordering — more specific rules (prep_time, calories) are defined before the generic price rule. Since comprehension marks consumed character regions, the specific rules consume "under 30 minutes" first, preventing the price rule from re-matching the same text. This is simpler and works correctly with RE2.

**Lesson:** Never use lookahead/lookbehind in Go regex patterns. If disambiguation is needed, use rule ordering + consumed-region tracking instead.

---

## 9. Comprehension Field Names Didn't Match Index Mapping

**Problem:** Comprehension rules used shorthand field names (`prep_time`, `calories`) that didn't match the actual OpenSearch index field names (`preparation_time`, `total_calories`). This caused `No field found for [prep_time] in mapping` errors at query time, even though the filter was correctly extracted from the query text.

Other mismatches discovered:
- `difficulty_level` stored as `"1"`, `"2"`, `"3"` — not `"easy"`, `"medium"`, `"hard"`
- `preparation_time` stored in seconds (e.g. `"1800"`) — rules said "30" meaning 30 minutes
- Sort fields: `created_at` doesn't exist, correct field is `updated_at`

**Solution:** Aligned all comprehension rules (across 8 languages) with actual staging index mapping:
- `prep_time` → `preparation_time` with multiplier 60 (minutes → seconds)
- `calories` → `total_calories`
- `difficulty_level` values: `"easy"` → `"1"`, `"medium"` → `"2"`, `"hard"` → `"3"`
- Sort field: `created_at` → `updated_at`

**Lesson:** Always verify filter field names against the actual index mapping before deploying comprehension rules. Use `GET /_mapping` to confirm field names and sample documents to confirm stored value formats.

---

## 10. OpenSearch `type` Field Mapping: text vs keyword

**Problem:** QUS stopword and compound loading queries used `term` filters on the `type` field:
```json
{ "term": { "type": "SW" } }
```
This worked locally where `type` was mapped as `keyword`, but failed on staging where `type` was mapped as `text` with a `.keyword` sub-field. Term queries on `text` fields don't work because the indexed tokens are analyzed (lowercased, etc.), so an exact match on `"SW"` finds nothing.

The result was `locales: 0` — no stopwords or compounds loaded, silently degrading search quality.

**Solution:** Changed term queries in QUS to target `type.keyword` instead of `type` (QUS `v0.2.1`):
```json
{ "term": { "type.keyword": "SW" } }
```

**Lesson:** Always use the `.keyword` sub-field for term queries on fields that might be mapped as `text`. This is forward-compatible — if the field is already `keyword`, the query still works (just without the sub-field). Check `GET /_mapping` to confirm field types on each environment.

---

## 11. Product Index Mapping Lost During Cross-Cluster Copy

**Problem:** Product indexes were copied from MXP staging to the test cluster using scroll+bulk API. The copy script created the destination index on-the-fly (auto-created by OpenSearch on first bulk insert), which resulted in **dynamic mapping** — OpenSearch inferred basic types (`text` + `keyword`) for all string fields.

The original MXP staging mapping had rich multi-field definitions per searchable field:

| Sub-field | Analyzer | Purpose |
|---|---|---|
| `title` (base) | — | `keyword` for exact match and aggregations |
| `title.concept` | `concept_analyzer` (keyword tokenizer + lowercase) | Full title as single token for concept matching |
| `title.shingle` | `shingle_analyzer` (standard + porter_stem + shingle 2–4) | Word n-gram phrase matching |
| `title.text` | `text_analyzer` (standard + porter_stem + stopwords) | Standard full-text search |
| `title.filter` | `filter_analyzer` (standard + lowercase) | Filtering |

All 4 custom analyzers (`concept_analyzer`, `shingle_analyzer`, `text_analyzer`, `filter_analyzer`) and their token filters (`shingle_filter`, `EnglishPossessiveFilter`, `custom_sw_filter`) were also lost.

Since the orchestrator queries `title.concept`, `title.shingle`, `title.text` etc., and OpenSearch silently returns 0 hits for non-existent fields, **every search returned 0 results** even for complete words like "chicken".

**Solution:** Re-copied all product indexes by:
1. Fetching the full mapping + settings (including analyzers) from the source index via `GET /{index}`
2. Creating the destination index with the correct mapping before inserting any docs via `PUT /{index}` with the mapping body
3. Then bulk-inserting documents via scroll+bulk as before

**Lesson:** When copying indexes across clusters, always create the destination index with the source mapping and settings first. Auto-created indexes from dynamic mapping lose custom analyzers, multi-field definitions, and field type overrides. The `_reindex` API within the same cluster preserves mappings, but scroll+bulk across clusters does not.

---

## 12. Partial/Prefix Search Returns 0 Hits for Incomplete Words

**Problem:** Typing "chi", "chic", or "chick" (building towards "chicken") returned 0 results across all search stages. This affected search-as-you-type behavior.

Root cause: all search stages used `match` queries, which compare analyzed tokens. The `text_analyzer` uses Porter stemming:
- "chi" → stem "chi" ≠ "chicken" → **no match**
- "chick" → stem "chick" ≠ "chicken" → **no match**
- "chicken" → stem "chicken" = "chicken" → **match**

`match` queries are token-based, not prefix-based. There is no overlap between the indexed term "chicken" and the search term "chi" — they are entirely different tokens.

**Solution:** Added a third search stage `fallback_prefix` with `query_mode: "prefix"`:

```yaml
- name: fallback_prefix
  minimum_hits: 1
  query_mode: prefix
  fields:
    - { name: title.text, boost: 100 }
    - { name: categories.text, boost: 90 }
    - { name: ingredients.text, boost: 80 }
    # ... other .text fields
```

The `prefix` query mode uses OpenSearch `prefix` queries instead of `match`:
```json
{ "prefix": { "title.text": { "value": "chi", "boost": 100 } } }
```

This scans the term dictionary for terms starting with "chi", which matches "chicken", "chipotle", etc.

The stage only activates when both `exact` and `fallback_partial` return 0 hits, so it has no impact on normal searches with complete words.

**Lesson:** `match` queries only find exact analyzed tokens, not prefixes. For search-as-you-type, use `prefix` queries, `match_phrase_prefix`, edge-ngram analyzers, or `search_as_you_type` field type. A prefix fallback stage is the least invasive option since it doesn't require mapping changes or re-indexing.
