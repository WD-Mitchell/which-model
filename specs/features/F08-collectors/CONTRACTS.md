---
kind: feature-contracts
feature: F08
version: "1.0"
project: which-model
---

# F08 — Catalog Fetch: Contracts (`internal/catalog/fetch`)

Packages: `internal/catalog/fetch/modelsdev/` and `internal/catalog/fetch/aa/` (no `fetch/transport.go` — F04 `internal/httpkit` is imported directly). Imports allowed per `specs/global/CONTRACTS.md §8`: `internal/config`? NO — F08 imports `internal/httpkit` (F04), `internal/decimal` (F02), `internal/catalog/identity` (F07), stdlib. It MUST NOT import `internal/config`, `internal/usage`, `internal/routing`, `internal/pick`, or `internal/catalog/csvstore`. Error codes follow the global `Failure.Code` vocabulary (`specs/global/CONTRACTS.md §1.6`).

## 1. Types

### 1.1 models.dev — `internal/catalog/fetch/modelsdev/provider.go` and `benchmark.go`

```go
package modelsdev

// ProvidersURL is the single provider-catalogue endpoint (one unauthenticated
// GET, no pagination).
const ProvidersURL = "https://models.dev/api.json"

// BenchmarksURL is the benchmark-catalogue endpoint — DISTINCT from
// ProvidersURL (pinned by a test).
const BenchmarksURL = "https://models.dev/models.json"

// ProviderModel is one non-deprecated models.dev model record.
type ProviderModel struct {
    Provider     string   // record["provider"] (e.g. "openai")
    ModelID      string   // record["id"]
    Name         string   // identity.CleanModelName(record["name"])
    Status       string   // record["status"] — "deprecated" records are dropped before this point
    BaseModel    string   // record["base_model"] ("" when absent)
    Reasoning    bool     // record exposes effort levels (len(EffortLevels) > 0)
    EffortLevels []string // sorted, normalized via identity.ParseEffort ("none"->"default"->"high");
                          // empty when Reasoning == false
}

// BenchmarkEvidence is one extracted (benchmark name, score) pair for a model.
type BenchmarkEvidence struct {
    Name   string // selected benchmark name, as passed in selectedNames
    Score  decimal.Decimal
    Effort string // "" = non-effort-scoped record; else normalized level in identity.ReasoningLevels
}

// BenchmarkRecord is one models.dev model's extracted evidence.
type BenchmarkRecord struct {
    CanonicalID string              // models.dev record id
    Name        string              // models.dev record name (NOT cleaned — F23 keys on CanonicalID)
    Benchmarks  []BenchmarkEvidence // only selectedNames names; max-resolved per (name, effort)
}
```

### 1.2 Artificial Analysis — `internal/catalog/fetch/aa/api.go`, `fields.go`, `page.go`

```go
package aa

// APIKeyEnv is the canonical environment variable for the AA API key.
const APIKeyEnv = "ARTIFICIAL_ANALYSIS_API"

// PrimaryURL / FreeURL: the v2 language-models endpoint and its free tier.
// FreeURL is used ONLY as the single 403 fallback.
const (
    PrimaryURL = "https://artificialanalysis.ai/api/v2/language/models"
    FreeURL    = PrimaryURL + "/free"
)

// MaxPages caps the pagination loop (pagination.page must equal the
// requested page on every response, else hard error).
const MaxPages = 100

// AABenchmarkField names an evaluations.* field mapped to a generated
// benchmark column. Fields and column names are VERBATIM from
// docs/plan/annex-b-catalog-port.md §2.4.
type AABenchmarkField struct {
    Field  string // evaluations.<field> JSON key
    Column string // generated raw-CSV column name (benchmark:<name>)
}

var AABenchmarkFields []AABenchmarkField

// AAModel is one deduplicated AA v2 model record.
type AAModel struct {
    Slug                  string                     // item["slug"]
    IntelligenceIndex     *decimal.Decimal           // fraction x 100, Round(2)
    CodingIndex           *decimal.Decimal           // fraction x 100, Round(2)
    AgenticIndex          *decimal.Decimal           // fraction x 100, Round(2)
    MedianResponseSeconds *decimal.Decimal           // performance.median_end_to_end_response_time_seconds
    CostPerTaskUSD        *decimal.Decimal           // artificial_analysis_intelligence_index_cost.cost_per_task.total_cost
    Benchmarks            map[string]decimal.Decimal // keyed by AABenchmarkField.Column; fraction x 100, Round(2)
}

// PageMetrics is the extracted currentModel data from a model's public page.
type PageMetrics struct {
    Slug                          string
    TimePerIntelligenceTaskSeconds *decimal.Decimal // intelligenceIndexTimePerTask
    FallbackCostUSD                *decimal.Decimal // intelligenceIndexCostPerTask.cost.total (only when requireFallbackCost)
}

// ModelPageURL is the page URL for a slug.
func ModelPageURL(slug string) string // "https://artificialanalysis.ai/models/" + slug
```

### 1.3 Fetch errors — `internal/catalog/fetch/errors.go`

```go
package fetch

// Error wraps a collector failure. Code is a global Failure.Code value
// (specs/global/CONTRACTS.md §1.6), plus the F08-owned "missing_api_key".
type Error struct {
    Code string
    Err  error
}

func (e *Error) Error() string
func (e *Error) Unwrap() error

// MissingAPIKeyError is returned by LoadAAAPIKey; Code "missing_api_key"
// (F23 maps to exit 2). Message: "missing ARTIFICIAL_ANALYSIS_API environment
// variable or .env entry".
func MissingAPIKeyError() *Error
```

## 2. Functions

### 2.1 models.dev — `internal/catalog/fetch/modelsdev/*.go`

```go
// F23-facing wrappers (no client/url params; default httpkit client):
func FetchModelsDevProviders() ([]ProviderModel, error)
func FetchModelsDevBenchmarks(selectedNames []string) ([]BenchmarkRecord, error)

// Testable cores (httptest-injectable):
func FetchModelsDevProvidersFrom(client *httpkit.Client, url string) ([]ProviderModel, error)
func FetchModelsDevBenchmarksFrom(client *httpkit.Client, url string, selectedNames []string) ([]BenchmarkRecord, error)
```

- `FetchModelsDevProvidersFrom`: `client.SetAllowList([]string{url})` then one `client.GetJSON(ctx, req)`-style GET (per F04 contract; `Accept: application/json`); payload unmarshal failure → `response_json`; `status == "deprecated"` records dropped; `EffortLevels` via `identity.ParseEffort` on `reasoning_options[].values`, sorted, deduped.
- `FetchModelsDevBenchmarksFrom`: same transport; extraction pre-seeded `{name: nil for name in selectedNames}` (annex-b §2.3 line 108 — selectedNames are BENCHMARK names from benchmarks.toml, never model ids); variant regex ports; effort `"none"`→`"default"`; max-wins per `(name, effort)` (never the dead Python `BENCHMARK_HARNESS_PRIORITY`); non-numeric score → `unsupported_response`; negative score → `unsupported_response`.
- **File isolation invariant (test-enforced):** `benchmark.go` source contains none of the tokens `FetchModelsDevProviders`, `ProvidersURL`, `ProviderModel`; `provider.go` contains none of `FetchModelsDevBenchmarks`, `BenchmarksURL`, `BenchmarkRecord`.

### 2.2 AA key — `internal/catalog/fetch/aa/key.go`

```go
// LoadAAAPIKey resolves the AA API key: env ARTIFICIAL_ANALYSIS_API first,
// then <repoRoot>/.env. Both absent -> MissingAPIKeyError(). The .env
// scanner: KEY=VALUE lines, blank lines and '#' comments skipped, one layer
// of matching surrounding quotes stripped, duplicate key = hard error
// (code "credential_file"), malformed line = hard error. Errors never
// contain the key value.
func LoadAAAPIKey(repoRoot string) (string, error)

// AAV2Client returns the AA client: httpkit.NewClient(httpkit.WithTimeout(20 * time.Second)).
// models.dev collectors use httpkit.NewClient() defaults (10s) — Decision D1.
func AAV2Client() *httpkit.Client

// AAPageClient returns the page-scraper client:
// httpkit.NewClient(httpkit.WithTimeout(20*time.Second), httpkit.WithMaxBytes(2<<20)).
func AAPageClient() *httpkit.Client
```

### 2.3 AA v2 API — `internal/catalog/fetch/aa/api.go`

```go
// F23-facing wrapper:
func FetchAAv2(client *httpkit.Client, apiKey string) ([]AAModel, error)

// Testable core:
func FetchAAv2From(client *httpkit.Client, apiKey string, primaryURL, freeURL string) ([]AAModel, error)
```

- Every request: header `x-api-key: <apiKey>`; allow-list set to the current page URL before each request; pages `?page=1..N` while `pagination.has_more`, cap `MaxPages` (exceeded → `unsupported_response`).
- Envelope: `{"data": [...], "pagination": {"page": int, "has_more": bool, "total_pages": int}}`; response `pagination.page != requested page` → `unsupported_response` (hard).
- Item mapping per `docs/plan/research/model-data-pipeline-spec.md §1.3`; evaluations values are fractions → `*100` then `decimal.Round(2)`; negative → `unsupported_response`; missing optional fields → nil pointer / absent map key.
- **403 fallback (Main pin):** on page-1 primary failure, `var he *httpkit.Error; if errors.As(err, &he) && he.StatusCode == 403 { retry entire pagination on freeURL }`. `httpkit.Error.StatusCode` is 0 for non-HTTP failures. 401 → propagate as `unauthorized`, never fallback. Message text is never matched.
- Highest-wins dedup across tau-variant records: group by `slug` (variant suffix stripped per annex-b §2.4), keep max per converted value.

### 2.4 AA page scraper — `internal/catalog/fetch/aa/page.go`

```go
// F23-facing wrapper (F23 calls with requireFallbackCost=false — best effort):
func FetchAAPage(client *httpkit.Client, slug string, requireFallbackCost bool) (*PageMetrics, error)

// Testable core:
func FetchAAPageFrom(client *httpkit.Client, slug string, requireFallbackCost bool, url string) (*PageMetrics, error)
```

- No custom headers; allow-list set to `url`; `AAPageClient()` defaults (20s, 2 MiB).
- `currentModel` script markers: objects whose `slug` matches the requested slug case-insensitively; extract `intelligenceIndexTimePerTask` and, when `requireFallbackCost`, `intelligenceIndexCostPerTask.cost.total`.
- Marker invariants: 0 matching markers → `(nil, nil)`; >1 markers with unequal slugs → `unsupported_response`; time-marker count != cost-marker count (when cost required) → `unsupported_response`; non-numeric marker values → `unsupported_response`.
- Marker regex/JSON-extraction approach is implementation-detail (free choice), but MUST be a strict parse: any partial/ambiguous marker set is an error, never a guess.

## 3. Config keys, flags, error codes, JSON shapes

- **Config keys owned:** none (F08 reads env `ARTIFICIAL_ANALYSIS_API` and `<repo root>/.env` only; historical config name `AA_API_KEY` is an alias of the env var, documented here for compat — F08 does NOT read config files; graph F08 → F02, F04, F07 excludes F01).
- **Flags owned:** none (F23 owns `--providers`, `--add aa_page`, timeouts).
- **Error codes added:** `missing_api_key` (F08-owned; F23 → exit 2). All other codes are global `Failure.Code` values.
- **Failure.Code mapping for F23** (global §1.6 + status mapping; exit codes per `specs/global/SPEC.md §5`):

| Collector condition | Failure.Code | Exit |
|---|---|---|
| API key absent (env + .env) | `missing_api_key` | 2 |
| .env malformed / duplicate key | `credential_file` | 5 |
| HTTP 401 (AA, after fallback logic) | `unauthorized` | 5 |
| HTTP 403 on both primary and free | `access_denied` | 5 |
| HTTP 429 | `rate_limited` | 1 |
| HTTP 5xx after retries | `provider_status` | 1 |
| 3xx (redirect refused) | `redirect_refused` | 1 |
| response over byte cap | `response_too_large` | 1 |
| deadline exceeded | `timeout` | 1 |
| transport/network failure | `network` | 1 |
| JSON/marker parse failure, pagination invariant, negative values | `unsupported_response` (or `response_json` for unmarshal) | 1 |
| allow-list violation (should not occur; F04 internal) | `untrusted_origin` | 1 |

- **JSON shapes emitted:** none (collectors return Go structs; F23 renders CSV).
- **httpkit dependency note:** F08 relies on F04's `httpkit.Error{Code string, StatusCode int, Err error}` with `StatusCode == 0` for non-HTTP failures and `errors.As` support — pinned by Main; message text is sanitized and not a contract.
