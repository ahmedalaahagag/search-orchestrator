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
