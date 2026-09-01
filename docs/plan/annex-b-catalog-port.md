# Annex B: Catalog & Ranking Port — `internal/catalog/` and `internal/pick/`

This annex is the authoritative Go port specification for `available-model-data-export/` (the Python collector/scoring pipeline in `.github/workflows/update_available_model_data/`) and its ranker (`.agents/skills/meta-orchestration-model-selection/scripts/rank_models.py`), mapped onto `internal/catalog/` (collectors, scoring, CSV storage) and `internal/pick/` (ranking profiles and tier1/tier2 combination arithmetic). It also specifies `internal/routing/`, the provider↔model join that is genuinely new in `which-model` — neither prototype has it. Source of truth for every fact below is `docs/plan/research/model-data-pipeline-spec.md` (cited as `§N.M`); Python line numbers cited there are reproduced here only where the annex reproduces code verbatim. Selection strategies, usage weight bands, and `Candidate.FinalScore` composition are [master plan](./README.md) territory and are referenced, not redefined, here. Usage-provider internals are Annex A; the CLI flag surface is Annex D; agent skills/hooks are Annex C.

---

## 0. Usage independence

`internal/catalog/**` has **no dependency** on `internal/usage/**`, in either direction of import. This is what the master plan's dependency rule (`pick -> routing -> {usage, catalog}`, never upward) buys, and it is worth stating as a testable property rather than an assumption:

- The catalog packages MUST compile and pass their full test suite under `go build -tags nousage`.
- Every `which-model catalog *` command is unaffected by all three levels of the usage toggle ([master plan §6](./README.md)). Refreshing, scoring, and listing the model catalogue never read a credential.
- The only authenticated call anywhere in this annex is the Artificial Analysis v2 API key (§2.3), which is a catalog data source, not a *provider usage* credential. It is unrelated to the usage toggle and is not gated by it.
- **CI MUST build and test with `-tags nousage`** on every change. Without that job the independence claim silently rots the first time someone adds a convenience import.

The one place a real dependency exists is route derivation, which can optionally consult a provider's live model list. §7.1 specifies how that degrades.

---

## 1. Decimal semantics

The Python pipeline uses `decimal.Decimal` with `ROUND_HALF_UP` exclusively — never `float` — for every metric, score, and category composite (`docs/plan/research/model-data-pipeline-spec.md:612`, restating `generate_scores.py:289-294,306-312,347-388`). The Go port MUST use `github.com/shopspring/decimal` end to end in `internal/catalog/score/` and `internal/pick/`; a `float64` MUST NOT appear anywhere on the numeric path from raw JSON ingestion to rendered CSV cell or ranked score.

### 1.1 Rounding equivalence — conclusion

Python's `decimal.ROUND_HALF_UP` is defined as "round to nearest, with ties going away from zero" (not "round toward +∞", despite the name). `shopspring/decimal`'s `Decimal.Round(places int32) Decimal` rounds half away from zero (confirmed against the package's documented rounding mode; `RoundBank` is the distinct banker's-rounding variant that must NOT be used here). **These two rounding modes are identical for every value, not merely for the non-negative score domain** — ties round away from zero regardless of sign in both implementations. Since every quantity quantized in this pipeline (`SCORE_QUANTUM = Decimal("1")`, `§5.2`) is additionally non-negative (scores are min-max normalized into `[0,100]`), the sign question is moot in practice, but the equivalence holds unconditionally.

Exact Go call reproducing `value.quantize(Decimal("1"), rounding=ROUND_HALF_UP)`:

```go
rounded := value.Round(0) // shopspring/decimal: half away from zero, matches ROUND_HALF_UP exactly
intString := rounded.StringFixed(0) // renders "0".."100" with no trailing decimal point, matching Python's str(Decimal) output for an integer-valued Decimal
```

For the fixed-decimal renders used in the raw CSV (`§3.1`: 1dp for `intelligence_index`, 0dp for time/response-time, 2dp for cost), use `value.Round(n).StringFixed(n)` with `n` = 1, 0, 2 respectively — `StringFixed` (not `String()`) is required to guarantee trailing-zero padding identical to Python's `Decimal.quantize(...).__str__()` (e.g. `"63.1"` not `"63.10000"`, and `"2.30"` not `"2.3"` when `n=2`).

### 1.2 Decimal-preserving JSON parsing

The AA v2 API is parsed in Python with `json.loads(text, parse_float=Decimal)` (`§1.3`, `get_aa_api_values.py`), so every JSON number — including ones inside nested nulls-may-appear envelopes like `artificial_analysis_intelligence_index_cost.cost_per_task.total_cost` — is a `Decimal`, never a float, from the moment it leaves the wire.

Go has no `parse_float` hook for `encoding/json`; the port MUST reproduce the guarantee at two levels:

1. **Typed structs** (the common case — AA response envelope, models.dev provider/benchmark records): declare numeric fields as `decimal.Decimal` (or `*decimal.Decimal` where the JSON key may be `null`). `shopspring/decimal.Decimal` implements `UnmarshalJSON` by reading the raw JSON number token as text and calling `decimal.NewFromString` internally — it never round-trips through `float64`. This is the default and preferred path for `internal/catalog/fetch/aa` and `internal/catalog/fetch/modelsdev`.
2. **Dynamic/validation-first decoding** (mirrors Python's blanket `parse_float=Decimal` when a payload is inspected as a generic tree before typed binding, e.g. envelope-shape validation prior to struct decode): use `dec := json.NewDecoder(r); dec.UseNumber()` before calling `dec.Decode(&v)` where `v` is `interface{}`/`map[string]interface{}`. Every JSON number then decodes as `json.Number` (a string alias) instead of `float64`. Convert at the point of consumption with `decimal.NewFromString(n.(json.Number).String())`, never `n.(json.Number).Float64()`.

Both paths MUST reject a JSON number that fails `decimal.NewFromString` (malformed numeric literal) as a hard collector error, matching the Python behavior of `InvalidOperation` propagating as `UpdateError` on malformed `evaluations`/`performance` payloads (`§7`, `UpdateError` row).

---

## 2. Collector specs (`internal/catalog/fetch/`)

Package layout: `internal/catalog/fetch/modelsdev/` (provider catalogue + benchmark catalogue, two independent files per §2.2/§2.3 below), `internal/catalog/fetch/aa/` (v2 API + opt-in public page scraper), `internal/catalog/fetch/transport.go` (shared HTTP client, §2.5).

### 2.0 Stage boundary: Collect vs Derive

The Python pipeline, and therefore the Go port, is two independently invokable stages, not one monolithic "refresh". This boundary is what makes `--refresh-scores` (Annex D) safe to run offline and cheap to run immediately after a `benchmarks.toml` edit — it never touches the network and never needs the AA key.

| Stage | Legacy script | Input | Output | Needs `ARTIFICIAL_ANALYSIS_API`? | Network? |
|---|---|---|---|---|---|
| **Collect** | `update_raw_values.py` | AA v2 API + models.dev | `available_model_raw_values.csv` | **YES** | yes |
| **Derive** | `generate_scores.py` | raw CSV + `benchmarks.toml` | `available_model_scores.csv` | no | **no** |

Evidence: `available-model-data-export/.github/workflows/update_available_model_data/update_raw_values.py` and `.../generate_scores.py` are today invoked as two separate `run:` steps in the workflow (§8), never merged into one script — the port's Go collector/scorer boundary follows the same split. §2.2–§2.5 below specify Collect; §4 (Scoring) specifies Derive.

Derive is a **pure function** of `(raw CSV, benchmarks.toml, active Normalizer, active Aggregator)` (§4.0) — no AA-keyed request, no models.dev request, no HTTP client of any kind is reachable from `internal/catalog/score`. This is a structural property, not a behavioural convention: `internal/catalog/score` MUST NOT import `internal/catalog/fetch/**`, so a Derive-only invocation cannot make a network call even by accident. It MUST be runnable fully offline and MUST NOT be blocked by a global `--offline` flag (Annex D), since it never attempted a network call to begin with.

**Consequence for a consumer without an AA key**: Collect is the only stage requiring credentials, so an operator who has never set `ARTIFICIAL_ANALYSIS_API` cannot run Collect themselves. They instead rely on the raw and scores artifacts the scheduled GitHub Action (§8) commits to the repository — Derive against those committed artifacts (`--refresh-scores`) still works fully offline, but a fresh Collect does not. This is a documented consequence of the two-stage design, not a gap the port needs to paper over with a download/CDN/release-asset mechanism.

### 2.1 Shared HTTP transport (`internal/catalog/fetch/transport.go`)

Applies to every collector in this section. Source: `§1.5` (`http_client.py`).

| property | value | citation |
|---|---|---|
| Per-request timeout | 20s | `§1.5`, `http_client.py:14` |
| Retry budget | 2 retries = 3 attempts total | `§1.5`, `http_client.py:15` |
| Retryable conditions | HTTP `429`, HTTP `500`–`599`, transport-level timeout/connection error | `§1.5` |
| Retry delay | `Retry-After` header if it parses as a number in `[0,10]` inclusive, verbatim; else `2^attempt` seconds (1, 2, 4, …); transport-level errors always use `2^attempt` (no `Retry-After` to consult) | `§1.5` |
| Redirects | **Hard-rejected on every 3xx** (301/302/303/307/308) — the client MUST NOT follow a redirect under any circumstance, specifically so an authenticated `x-api-key` header cannot leak to a redirected origin | `§1.5`, `http_client.py:18-30` |
| TLS | Standard library default certificate + hostname verification (`crypto/tls` default `Config{}` is already `InsecureSkipVerify: false`); the Go port needs no equivalent to Python's `ssl.VERIFY_X509_STRICT` workaround — Go's TLS stack has no strict-X.509-profile toggle to disable | `§1.5` |
| Non-retryable error mapping | `401` → `"API key was rejected (HTTP 401)"`; `403` → `"API access was forbidden for this key (HTTP 403)"`; `429` after retries exhausted → `"rate limit exceeded (HTTP 429)"`; any 3xx → `"redirects are not allowed (HTTP {status})"`; `5xx` after retries exhausted → `"server error persisted (HTTP {status})"`; else → generic `"HTTP {status}"` | `§1.5`, `http_client.py:33-41` |
| Headers sent | `Accept: application/json, text/html;q=0.9`; `User-Agent: centree-model-metrics-updater/1.0` — the port MUST rename the `User-Agent` to a `which-model`-specific token but MUST keep an explicit, static UA rather than the Go default; plus caller-supplied headers (e.g. `x-api-key`) merged in | `§1.5`, `http_client.py:104` |

Go implementation: build on `net/http.Client` with a custom `http.RoundTripper` whose `Transport.CheckRedirect` (actually `http.Client.CheckRedirect`, since `net/http` intercepts redirects at the client, not the transport, before a `RoundTripper` ever sees them) returns `http.ErrUseLastResponse` is insufficient — that still returns a `3xx` response to the caller without an error; the port instead sets `CheckRedirect: func(*http.Request, []*http.Request) error { return errRedirectRejected }`, which makes `Client.Do` return that error directly on any redirect attempt, matching "hard-fail on 3xx" without the caller ever seeing a followed response. Wrap `Client.Do` in a helper (e.g. `fetch.Do(ctx, req) ([]byte, error)`) that owns the timeout (`context.WithTimeout(ctx, 20*time.Second)` per attempt), retry loop, and status-code error mapping table above.

### 2.2 models.dev provider catalogue (`internal/catalog/fetch/modelsdev/provider.go`)

- URL: `https://models.dev/api.json` — `§1.1`.
- **Unauthenticated.** Exactly one GET, no pagination. This is a load-bearing invariant (test `test_provider_collection_is_one_unauthenticated_request`, `§1.1`) — the Go collector MUST issue exactly one request and MUST NOT attach any credential header.
- Response: JSON object keyed by provider id (`anthropic`, `openai`, `github-copilot`, …). Per model: `{"id","name","status","base_model","reasoning" (bool),"reasoning_options":[{"type":"effort","values":[...]}]}`.
- Consumed fields: `id`, `name` (through `clean_model_name`, §3.1), `status`, `base_model`, `reasoning`, and — when `reasoning_options[].type == "effort"` — its `.values`, with `"none"`/`"default"` normalized to `"high"`; values must be a subset of the effort enum (`minimal,low,medium,high,xhigh,max`) — `§1.1`.
- Filtering: a model is dropped when `status == "deprecated"` OR its models.dev key is in that provider's `providers.toml` `excluded_models` — `§1.1`, `§2`.
- Each `providers.toml` key MUST resolve to a top-level models.dev key with matching `record["id"]`, else a hard collector error ("models.dev has no valid provider …") — `§2`.
- Stale exclusion ids (configured but absent from the current models.dev payload) are a non-fatal warning collected onto the run report, not a fatal error — `§2`.

### 2.3 models.dev benchmark catalogue (`internal/catalog/fetch/modelsdev/benchmark.go`)

- URL: `https://models.dev/models.json` — `§1.2`. **Deliberately a different endpoint from §2.2**, and this file MUST have zero import/reference to the provider-catalogue package or its URL constant — enforced today by `test_benchmark_module_has_no_provider_collector_dependency` via source-text scanning; the Go equivalent is an architectural boundary the port MUST preserve (e.g. `modelsdev/benchmark.go` imports neither `modelsdev/provider.go`'s exported URL constant nor any provider-catalogue type) so the same intent (`test_source_urls_are_distinct_and_current`) is verifiable.
- **Unauthenticated.** Exactly one GET, no pagination.
- Response: JSON object keyed by canonical id `"<provider>/<slug>"`; each record `{"id","name","benchmarks":[{"name","score","variant","version"}]}`.
- Matching a provider model (from §2.2) to a canonical record, in priority order (`§1.2`):
  1. `entry.canonical_id` (models.dev `base_model`) if set — must resolve or hard error.
  2. `"{provider}/{model_id}"` direct lookup.
  3. Fallback lookup by **id suffix** (`canonical_id` after the last `/`) or by **cleaned display name**; unlike the provider-family matcher in §2.4, multiple hits here are NOT ambiguous — evidence from every matching record is merged, keeping the highest value per benchmark name.
- Effort scoping (`_effort`, `§1.2`): parse `record["variant"]` against `(?P<effort>minimal|low|medium|high|xhigh|max)(?: effort| reasoning)?(?:, (?:context compaction|with tools))?` or `reasoning effort (?P<effort>none|minimal|low|medium|high|xhigh|max)(?:, ...)?`; `"none"` → `"default"`. No parseable variant → default (non-effort-scoped) value; parseable effort → override scoped to that effort.
- Duplicate resolution: **always the maximum numeric score** across records sharing `(benchmark name, effort)`, source-agnostic — `BENCHMARK_HARNESS_PRIORITY` in the Python source is dead code and MUST NOT be ported.
- Benchmark `version` metadata is ignored entirely — never validated for shape and never used to break ties.  models.dev emits free-form versions (`"1.1 Main"`); shape validation broke the collector in CI (2026-09-01) and the max-score policy never consumed the value.
- Only benchmark names present in the caller-supplied `selected_names` (from `benchmarks.toml`, §6.3) are extracted — the extraction map is pre-seeded `{name: nil for name in selected_names}`.

### 2.4 Artificial Analysis v2 API (`internal/catalog/fetch/aa/api.go`) — the only authenticated source

- Primary URL: `https://artificialanalysis.ai/api/v2/language/models` — `§1.3`.
- Fallback URL (403 ONLY): `https://artificialanalysis.ai/api/v2/language/models/free` — `§1.3`. Fallback trigger is **exactly** "the primary request's terminal error is HTTP 403"; any other terminal error (429 after retries, 5xx after retries, malformed body) MUST propagate without attempting the free endpoint. This is load-bearing (`test_artificial_analysis_falls_back_to_free_endpoint_only_when_forbidden`, `§8.3`).
- Auth: header `x-api-key: <key>`. Env var name `ARTIFICIAL_ANALYSIS_API`.
  - Key resolution order: (1) `os.Getenv("ARTIFICIAL_ANALYSIS_API")` if non-blank after trim; (2) else parse `<repo root>/.env` — simple `KEY=VALUE` line scanner skipping blank/`#` lines, stripping one layer of matching surrounding quotes, hard error if the key is defined more than once in `.env`. Neither present → hard error naming both the env var and the `.env` path (`§1.3`).
  - CI supplies it via `env: ARTIFICIAL_ANALYSIS_API: ${{ secrets.ARTIFICIAL_ANALYSIS_API }}` — unchanged in the Go port (§8).
- Pagination invariants (`§1.3`) — the collector loops `GET {api_url}?page={n}` starting at `n=1`, decoding `{"data":[...],"pagination":{"page":int,"has_more":bool,"total_pages":int}}`, and MUST hard-fail the whole collection if any of:
  - `pagination.page != n` (server returned a different page than requested),
  - `total_pages` outside `[1,100]` (`MAX_PAGES = 100`),
  - `has_more == false` and `page != total_pages`,
  - `has_more == true` and `page >= total_pages`.
  Loop terminates when `has_more == false`.
- Consumed fields per item (`§1.3`): `slug`; `evaluations.artificial_analysis_intelligence_index` / `..._coding_index` / `..._agentic_index` (required keys, nullable values — a Go struct MUST distinguish "key absent" from "key present with `null`" using `*decimal.Decimal` plus explicit `json.RawMessage`/two-pass decode if the key-presence distinction itself is asserted, matching Python's KeyError-vs-None distinction tested by `§8.4` "required-metric-KEY-presence" cases); `performance.median_end_to_end_response_time_seconds` (required key, nullable, if present must be `>= 0`); `artificial_analysis_intelligence_index_cost` (required top-level key, nullable; if present, must contain `cost_per_task` which if present must contain `total_cost` which must be `>= 0`).
- `AA_BENCHMARK_FIELDS` — verbatim, reproduced as a Go declaration in `internal/catalog/fetch/aa/fields.go`:

```go
// AABenchmarkField is (API evaluation field, generated benchmark column, value is a 0..1 fraction).
// The two AA index fields are already on a 0..100 scale; the per-evaluation
// fields are proportions in the V2 response and must be converted to percentages.
type AABenchmarkField struct {
    APIField   string
    Column     string
    IsFraction bool
}

var AABenchmarkFields = []AABenchmarkField{
    {"artificial_analysis_coding_index", "Artificial Analysis Coding Index", false},
    {"artificial_analysis_agentic_index", "Artificial Analysis Coding Agent Index", false},
    {"tau_banking", "τ3 Banking", true},
    {"tau3_banking", "τ3 Banking", true},
    {"tau2_banking", "τ3 Banking", true},
    {"terminalbench_v2_1", "Terminal-Bench", true},
    {"terminalbench_hard", "Terminal-Bench Hard", true},
    {"scicode", "SciCode", true},
    {"ifbench", "IFBench", true},
    {"ifeval", "IFEval", true},
    {"hle", "Humanity's Last Exam", true},
    {"gpqa_diamond", "GPQA Diamond", true},
    {"mmmu_pro", "MMMU Pro", true},
    {"gdpval_aa_normalized", "GDPval-AA", true},
}
```

  Rules (`§1.3`): when `IsFraction` and the raw value is in `[0,1]`, multiply by 100 before storing as the benchmark cell. When multiple entries map to the same `Column` (the three `tau*_banking` variants → `τ3 Banking`, `terminalbench_v2_1`/`terminalbench_hard` are distinct columns already), **the highest converted value wins** per `(model, reasoning, Column)`.
- Name/effort parsing (`§1.3`): split `"Model Name (config)"` via `(.+)\s+\(([^()]*)\)`; treat the parenthetical as a reasoning-effort annotation only if it matches `\b(?:minimal|low|medium|high|xhigh|extra[- ]high|max(?:imum)?|thinking|adaptive|non[- ]reasoning|reasoning|effort)\b` (case-insensitive); otherwise the full string (parens included) goes through `clean_model_name` untouched. `_reasoning(configuration, slug)` maps to one of `minimal|low|medium|high|xhigh|max` via regex alternation, supports a `"<Model> <N> fallback"` parenthetical suffix, and is a hard error on an ambiguous (>1 distinct alias matched) or unrecognized configuration string; `"non-reasoning"` → `nil` (routed to the `"default"` row); `"thinking"`/bare `"reasoning"` → `"thinking"`; `"adaptive"`/`"adaptive reasoning"` → `"adaptive"`.
- `_root_slug`: strip a trailing `-YYYYMMDD` or `-YYYY-MM-DD` date suffix, then a trailing `-latest` (case-insensitive). `match_provider_models` groups AA entries by name-token key and requires every slug in a group to resolve to exactly one `_root_slug` value, else the whole group is silently skipped.
- Provider-model→AA-family matching (`_provider_keys`): token sets built from the cleaned display name and from `model_id` with any `-picker` suffix stripped. A provider model whose key set matches **more than one** AA family is a hard collector error ("ambiguously matches multiple … families") — this ambiguity MUST stay fatal in the port, matching §3.3's fail-loud posture. Cross-provider aggregate matching also checks `canonical_id` equality; a conflict there is also a hard error.
- Output: one row per `(aggregate, reasoning level)` observed across providers/AA. If an AA model exists at that exact effort level, it is the base row (benchmark cells merged with any provider-supplied cells, max-per-name); else the closest AA model whose base effort (pre-fallback-suffix) matches is used if unambiguous; else no AA financials attach and only benchmark evidence populates the row.

### 2.5 Artificial Analysis public model page (`internal/catalog/fetch/aa/page.go`) — opt-in only

- URL template `https://artificialanalysis.ai/models/{slug}` (URL-escaped slug) — `§1.4`.
- **No authentication, no custom headers.** This is asserted by `test_page_cli_requests_only_public_slug_pages_without_headers` and MUST hold in the Go collector too.
- Parsing: scan raw HTML/JS for `"currentModel":\s*\{` markers; for each, extract the balanced `{...}` object via brace-matching that understands quoted strings/escapes; reject an object containing a duplicate key. Use only the object whose `.slug` equals the requested slug. Zero markers → both output fields nil (no error); markers present but none match the slug → hard error (slug mismatch); more than one match for the slug → hard error (ambiguous).
- Consumed fields: `intelligenceIndexTimePerTask` → `time_per_intelligence_index_task_seconds` (must be `>=0`); `intelligenceIndexCostPerTask.cost.total` → fallback cost (must be `>=0`), consulted only when the caller explicitly requests `require_fallback_cost`.
- Wiring: this collector is reachable ONLY via an explicit opt-in flag on the refresh command (Annex D scope for the exact flag name; mirrors Python's `--add aa_page`). The scheduled nightly run (§8) MUST NEVER invoke it — enforced today by `test_default_row_collection_never_calls_public_page_collector`; the Go port MUST keep an equivalent test asserting the default collection path never references this package's exported entry point.

---

## 3. Identity, matching, merging (`internal/catalog/identity.go`, `internal/catalog/merge.go`)

### 3.1 `CleanModelName` (`§4.1`, `model_types.py:27-59`)

Strips balanced `()`, `[]`, `{}` groups, including nested groups, and even malformed/mismatched ones (an unmatched opener suppresses everything after it to end of string), then collapses/trims whitespace to single spaces. Go: a single-pass rune scanner tracking a bracket-depth counter per opener type (or a unified depth stack, since Python's implementation treats any of the three opener runes as depth-incrementing and any matching-or-not closer as depth-decrementing — reproduce the exact "malformed opener suppresses remainder" behavior, do not use a regex that assumes balanced brackets). Examples: `"Claude Opus 4.5 [claude-opus-4-5-20251101]"` → `"Claude Opus 4.5"`; `"Claude Haiku 4.5 (latest)"` → `"Claude Haiku 4.5"`.

### 3.2 Identity key

Model identity is the tuple `(CleanModelName(model), reasoning)` **after** context-dependent `default`→`high` collapsing. This collapse is applied in exactly two places and nowhere else:

| location | collapses `default`→`high`? |
|---|---|
| `internal/catalog/csvstore` raw-CSV merge (`csv_store._collapse_default_reasoning`) | **yes** |
| `internal/catalog/score` generation, reading the raw CSV (`generate_scores._merge_input_rows`) | **yes** |
| `internal/catalog/csvstore` raw-CSV writer (fresh rows straight from a collector) | **no** — a `"default"` row is written as `"default"` |
| `internal/pick` ranking, reading the scores CSV (`rank_models.py`) | **no** — `LoadScoreRows` MUST reject duplicate `(model, reasoning)` identities as a hard error rather than merging them |

### 3.3 `_merge_input_rows` (`§4.3`) — scores-CSV generation input merge

Group rows by collapsed identity in first-seen order. For each later row sharing an identity, per non-identity column: if the stored value is currently unset, adopt the new value; else, if BOTH values are set and the column is a `benchmark:` column, keep `max(current, new)`; else the first non-null value wins and the duplicate's value is discarded silently (safe because the upstream raw-CSV merge in §3.4 already performed most of this collapsing).

### 3.4 `MergeRows` / `MergePartialRefresh` (`§4.4`) — raw-CSV incremental merge

`MergeRows(fresh, current)`: both inputs first pass through the `default`→`high` collapse. For a fresh row matching a current row's identity, every core metric column takes **fresh if non-null, else current** — fresh always wins when non-null (a "refresh" merge, not a max-merge; zero is a valid win, never falsy-skipped). Benchmark cells: a fresh cell that is null AND whose benchmark name is NOT in that fresh row's `AuthoritativeBenchmarks` set falls back to the current CSV's value for that cell (preserves stale-but-still-true evidence across partial refreshes while allowing an explicit "clear" when an override table exists for the model but omits that name).

`MergePartialRefresh(fresh, current, refreshedFamilies, preserveUnselected)`: after `MergeRows`, if `preserveUnselected` is true (only when `--provider` explicitly narrows below the full configured provider set), rows for models not in `refreshedFamilies` are appended unchanged from `current` so an unrefreshed provider's data survives a partial run.

### 3.5 Effort parsing regexes

Reproduce verbatim from §2.3/§2.4: the models.dev benchmark `_effort` pattern and the AA `_reasoning`/parenthetical-annotation patterns. Do not simplify or "improve" these regexes — they are tuned against the exact vocabulary both upstream sources use, and the 113-test suite (§9) pins their exact matching/rejection behavior.

### 3.6 `_root_slug`

Strip trailing `-YYYYMMDD` or `-YYYY-MM-DD`, then trailing `-latest` (case-insensitive), in that order, at most once each.

### 3.7 Ambiguity errors that MUST stay fatal

- A provider model whose key set matches more than one AA family (§2.4).
- A provider model matched to a previously-seen aggregate whose `canonical_id` conflicts (§2.4).
- A models.dev benchmark canonical-id lookup with a declared `base_model`/`canonical_id` that does not exist in the benchmark payload (§2.3, priority 1).
- Duplicate `(model, reasoning)` identity in the scores CSV as read by `internal/pick` (§3.2 table, row 4).

None of these may be downgraded to a warning or resolved by "first match wins" — the Python source treats every one as a correctness bug in the upstream data or configuration, not a normal runtime condition.

### 3.8 `_benchmark_key` alias map — verbatim (`§4.5`, `generate_scores.py:118-133`)

```go
// BenchmarkKey returns a stable key used to deduplicate benchmark aliases/variants
// before they are averaged into a category composite (§4). Two benchmark names
// that collapse to the same key are ONE evidence source, never two.
func BenchmarkKey(value string) string {
    normalized := strings.ToLower(norm.NFKC.String(value)) // Unicode NFKC + casefold
    normalized = strings.NewReplacer("\u2019", "'", "`", "'").Replace(normalized)
    var b strings.Builder
    for _, r := range normalized {
        if unicode.IsLetter(r) || unicode.IsDigit(r) {
            b.WriteRune(r)
        }
    }
    compact := b.String()
    if alias, ok := benchmarkKeyAliases[compact]; ok {
        return alias
    }
    return compact
}

var benchmarkKeyAliases = map[string]string{
    "financeagent":                       "financeagent",
    "gdpval":                             "gdpval",
    "gdpvalaa":                           "gdpval",
    "humanityslastexam":                  "humanityslastexam",
    "artificialanalysiscodingindex":      "artificialanalysiscodingindex",
    "artificialanalysiscodingagentindex": "artificialanalysiscodingagentindex",
}
```

Note: Python's `casefold()` is a stronger normalization than Go's `strings.ToLower` for a handful of non-ASCII code points (e.g. German ß). None of the 51 current benchmark names (§6.2) contain such code points, so `strings.ToLower` on NFKC-normalized text is behavior-equivalent for the actual input domain; if a future benchmark name introduces a casefold-sensitive character, `golang.org/x/text/cases.Fold()` MUST replace the plain `ToLower` call. The literal `’` (U+2019 right single quotation mark) is normalized to ASCII `'` before the alnum filter, exactly as in Python, so `"Humanity's Last Exam"` (straight quote) and `"Humanity’s Last Exam"` (curly quote) key identically.

---

## 4. Scoring (`internal/catalog/score/`)

### 4.0 Pluggable normalization and aggregation

The formulae in §4.1–§4.8 reproduce `generate_scores.py` exactly, because M1's acceptance criterion is byte-for-byte equivalence. But the method is known-suspect ([master plan §7](./README.md)), so it MUST go in behind two interfaces from the start — cheap now, expensive to retrofit once callers assume a concrete implementation.

```go
// Normalizer maps a raw metric value to a comparable score. Implementations
// receive the full eligible column so distribution-aware methods (quantile,
// winsorized, robust) can compute the statistics they need.
type Normalizer interface {
    Name() string // stable id recorded in evidence, e.g. "minmax-linear"
    Normalize(values []decimal.Decimal, higherIsBetter bool) []decimal.Decimal
}

// Aggregator combines weighted component scores into a composite.
type Aggregator interface {
    Name() string // e.g. "weighted-arithmetic-mean"
    Aggregate(components []decimal.Decimal, weights []decimal.Decimal) decimal.Decimal
}
```

Shipped defaults, which MUST reproduce today's output exactly:

| Interface | Default implementation | Behaviour |
| --- | --- | --- |
| `Normalizer` | `minmax-linear` | §4.1 verbatim |
| `Aggregator` | `weighted-arithmetic-mean` | §4.5–§4.6 and §5.3 verbatim |

Normative requirements:

- The chosen normalizer and aggregator names MUST be recorded in the scores artifact metadata and in `which-model explain` evidence. A score that cannot be traced to the method that produced it is not evidence.
- A degenerate-range column (`min == max`) keeps the §4.1 mandatory/optional split regardless of normalizer — that is a data-completeness rule, not a normalization rule.
- Changing a default is a deliberate, documented migration accompanied by a differential report of which rankings move. **Never a silent swap** — the whole point of recording the method is that a consumer can tell when it changed.
- `Normalize` takes the whole column rather than one value precisely so distribution-aware methods are expressible. A per-value signature would have locked out quantile, robust, and winsorized normalizers permanently.

Candidate implementations and the weight-scale question are the R1 research track ([master plan §7.4](./README.md)); this annex fixes only the interface and the defaults.

### 4.0a Dual absolute and relative columns

Min-max destroys absolute meaning: `benchmark:SWE-Bench Verified = 91` in the scores CSV means "91% of the way between the worst and best model in this dataset", not a 91% pass rate. The raw CSV held the real 96.0% and normalization discarded it.

**Decision: the scores artifact carries both**, in distinctly named columns.

| Column form | Content | Stable across refreshes |
| --- | --- | --- |
| `<metric>` / `benchmark:<name>` | Absolute native value, carried through from the raw CSV unchanged | yes |
| `<metric>_score` / `benchmark:<name>_score` | Relative normalized 0–100 under the active `Normalizer` | no — min/max are dataset properties |

This is a **schema change** from the current scores CSV, which reuses the bare `benchmark:<name>` header to hold the *normalized* value (§6.2) — a genuine trap, since the header is identical to the raw CSV's but the number means something different. The migration:

1. Absolute columns keep the raw CSV's exact header spelling and units, so a reader can diff raw against scores and see matching values.
2. Relative columns gain the explicit `_score` suffix, which the non-benchmark metric columns already use (`intelligence_index_score`); benchmark columns are brought into line with that existing convention rather than inventing a new one.
3. `internal/pick` reads only `_score` columns, so ranking behaviour is unchanged and M1's equivalence proof is unaffected.
4. `which-model catalog scores --schema-version` reports which layout a file uses, so a stale artifact is detected rather than silently misread.

Interpretability is the point: an operator asking "is this model actually good at SWE-Bench" gets 96.0, while the ranker gets the commensurable score it needs. Neither answer is sacrificed to the other.

### 4.1 Normalization formula (`§5.1`)

```go
func normalizedScore(value, min, max decimal.Decimal, higherIsBetter bool) decimal.Decimal {
    numerator := value.Sub(min)
    if !higherIsBetter {
        numerator = max.Sub(value)
    }
    return oneHundred.Mul(numerator).Div(max.Sub(min))
}
```

Min/max are computed **per column, over eligible rows only** (rows with all three Tier-1 metrics present — §4.3). A degenerate range (`min == max`) is a hard error for a mandatory column; for an optional column (any `OptionalMetrics` member or any `benchmark:` column) it instead leaves that column blank for every row. Zero populated values in a column follows the identical mandatory/optional split.

### 4.2 Rounding/quantum

`SCORE_QUANTUM = decimal.NewFromInt(1)`. Every score — per-metric, per-benchmark, category composite, `planning_capability_score` — is `.Round(0)` (§1.1) rendered as an integer string `"0"`.."100"`.

### 4.3 Tier-1 mandatory metrics (`§5.3`)

```go
var RequiredTier1Metrics = []string{
    "intelligence_index",
    "median_end_to_end_response_time_seconds",
    "cost_per_intelligence_index_task_usd",
}

var CoreMetrics = map[string]bool{ // value = higher-is-better
    "intelligence_index":                             true,
    "time_per_intelligence_index_task_seconds":        false,
    "cost_per_intelligence_index_task_usd":             false,
    "median_end_to_end_response_time_seconds":          false,
    "artificial_analysis_coding_index":                 true,
    "artificial_analysis_agentic_index":                true,
}

var OptionalMetrics = map[string]bool{
    "time_per_intelligence_index_task_seconds": true,
    "artificial_analysis_coding_index":          true,
    "artificial_analysis_agentic_index":          true,
}
```

A merged raw-CSV row lacking ANY of the 3 `RequiredTier1Metrics` is dropped entirely from the scores CSV (not zero-filled). Zero eligible rows after this filter is a hard collector error.

### 4.4 `CategoryMinimumEvidence` — verbatim (`§5.4`, `generate_scores.py:99-111`)

```go
var CategoryMinimumEvidence = map[string]int{
    "reasoning_score":             2,
    "knowledge_score":             2,
    "research_score":              2,
    "instruction_following_score": 2,
    "software_engineering_score":  2,
    "ui_visual_score":             2,
    "agentic_tools_score":         2,
    "finance_score":               2,
    "evidence_capture_score":      2,
    "security_score":              1, // sole exception: only 2 candidate benchmarks exist today (CyberGym, CTI-REALM); requiring 2 would make the category almost never computable
    "data_ml_score":               2,
}
```

`planning_capability_score` is intentionally absent — it has its own fixed-composition rule (§4.6), never a benchmark-group average.

`CategoryGroups` maps each `*_score` column to its `benchmarks.toml` group id — verbatim (`§5.4`, `generate_scores.py:81-93`):

```go
var CategoryGroups = map[string]string{
    "reasoning_score":             "reasoning",
    "knowledge_score":             "knowledge",
    "research_score":              "research",
    "instruction_following_score": "instruction_following",
    "software_engineering_score":  "software_engineering",
    "ui_visual_score":             "ui_visual",
    "agentic_tools_score":         "agentic_tools",
    "finance_score":               "finance",
    "evidence_capture_score":      "evidence_capture",
    "security_score":              "security",
    "data_ml_score":               "data_ml",
}
```

Computation for a non-planning category column (`§5.4`): resolve the group's benchmark name list from `benchmarks.toml`; call `SourceScores` (§4.5) for the row to get an `alias key → normalized score` map; for each name in the group (in `benchmarks.toml` order), dedup via `BenchmarkKey` (§3.8), collect the score if present; if `len(collected) < CategoryMinimumEvidence[column]`, the category score is blank; else it is the **unweighted arithmetic mean** of the collected per-benchmark normalized scores, rounded per §4.2.

### 4.5 `SourceScores` — AA-index-preferred-over-models.dev rule (`§5.5`, verbatim structure)

```go
func SourceScores(outputRow map[string]string) map[string]decimal.Decimal {
    result := map[string]decimal.Decimal{}
    for _, pair := range []struct{ column, sourceName string }{
        {"artificial_analysis_coding_index_score", "Artificial Analysis Coding Index"},
        {"artificial_analysis_agentic_index_score", "Artificial Analysis Coding Agent Index"},
    } {
        if v, ok := decimalScore(outputRow[pair.column]); ok {
            result[BenchmarkKey(pair.sourceName)] = v
        }
    }
    for column, value := range outputRow {
        if !strings.HasPrefix(column, BenchmarkColumnPrefix) {
            continue
        }
        v, ok := decimalScore(value)
        if !ok {
            continue
        }
        name := strings.TrimPrefix(column, BenchmarkColumnPrefix)
        key := BenchmarkKey(name)
        if _, exists := result[key]; !exists { // setdefault semantics — AA-index entries inserted above win ties
            result[key] = v
        }
    }
    return result
}
```

The two AA-index columns are inserted first under alias keys `artificialanalysiscodingindex` / `artificialanalysiscodingagentindex`. The subsequent loop over dynamic `benchmark:` columns MUST use insert-if-absent (`setdefault`) semantics, not overwrite: if models.dev also publishes a benchmark literally named `"Artificial Analysis Coding Index"` (it is one of the 51 configured names, §6.2), its value is silently ignored because the AA-index-derived score already claims that alias key. This prevents one underlying signal from being counted twice in a category mean — iteration order over a Go map is non-deterministic, so the implementation MUST insert the two AA-index entries into the result map before ranging over the dynamic columns (as shown), not rely on map iteration order to enforce precedence.

### 4.6 `planning_capability_score` — fixed weighted formula, verbatim (`§5.6`, `generate_scores.py:352-368`)

```go
var planningComponents = []struct {
    column string
    weight decimal.Decimal
}{
    {"reasoning_score", decimal.RequireFromString("0.40")},
    {"knowledge_score", decimal.RequireFromString("0.30")},
    {"agentic_tools_score", decimal.RequireFromString("0.20")},
    {"research_score", decimal.RequireFromString("0.10")},
}

func planningCapabilityScore(outputRow map[string]string) string {
    values := make([]decimal.Decimal, len(planningComponents))
    for i, c := range planningComponents {
        v, ok := decimalScore(outputRow[c.column])
        if !ok {
            return "" // any missing component -> blank, no partial credit
        }
        values[i] = v
    }
    weighted := decimal.Zero
    for i, c := range planningComponents {
        weighted = weighted.Add(values[i].Mul(c.weight))
    }
    return weighted.Round(0).String()
}
```

All four component category scores must themselves already be non-blank (i.e. they individually satisfied `CategoryMinimumEvidence`, §4.4) — there is no partial credit and no imputation for a missing component.

**This weighting is fixed and MUST NOT be re-weighted by a caller/profile.** `planning_capability_score` is a Tier-2 category like any other from `internal/pick`'s point of view (a profile may assign it a tier-2 weight, e.g. the `"planning"` profile at weight 5, §5.1), but its own *internal* composition (0.40/0.30/0.20/0.10 across reasoning/knowledge/agentic_tools/research) is computed once, upstream, in `internal/catalog/score`, before any profile is applied. Allowing a profile to override these four internal weights would let two different callers derive two different `planning_capability_score` values for the identical model+reasoning row, which breaks the CSV's contract as a profile-independent artifact (`§9`: "the scores CSV is entirely a pure function of the raw CSV + benchmarks.toml").

### 4.7 Blank-means-not-computable

Every `_score` cell (per-metric, per-benchmark, category, planning) that lacks sufficient evidence is rendered as an **empty string**, never a zero. `internal/pick` MUST treat an empty/missing score cell as "not available for this row" (§5.3 step 4/5), never as a literal 0 — conflating the two would silently bias every ranking's tier-2 average downward for rows with sparse benchmark coverage.

### 4.8 Output row assembly order (`§5.7`)

Per eligible row: `(model, reasoning)` identity columns, then one `{metric}_score` column per entry of `CoreMetrics` in fixed declaration order (Tier-1 metrics first), then the 12 `CategoryScoreColumns` in the fixed order `reasoning_score, knowledge_score, research_score, planning_capability_score, instruction_following_score, software_engineering_score, ui_visual_score, agentic_tools_score, finance_score, evidence_capture_score, security_score, data_ml_score` (computed from the metric-score columns just written, since categories read from the output row, not raw values), then each dynamic `benchmark:<name>` column re-scored via the §4.1 formula with `higherIsBetter = true` always.

---

## 5. Ranking (`internal/pick/`)

### 5.1 `PROFILES` — verbatim (`§6.1`, `rank_models.py:124-171`)

```go
type Tier1Axis string

const (
    AxisIntelligence Tier1Axis = "intelligence"
    AxisCost         Tier1Axis = "cost"
    AxisSpeed        Tier1Axis = "speed"
)

// Tier1ScoreColumn maps the 3 fixed ranking axes to their scores-CSV columns.
var Tier1ScoreColumn = map[Tier1Axis]string{
    AxisIntelligence: "intelligence_index_score",
    AxisCost:         "cost_per_intelligence_index_task_usd_score",
    AxisSpeed:        "median_end_to_end_response_time_seconds_score",
}

var CategoryNames = []string{
    "reasoning", "knowledge", "research", "planning_capability",
    "instruction_following", "software_engineering", "ui_visual",
    "agentic_tools", "finance", "evidence_capture", "security", "data_ml",
}

type Profile struct {
    Name         string
    Tier1Share   decimal.Decimal
    Tier2Share   decimal.Decimal
    Tier1Weights map[Tier1Axis]decimal.Decimal
    Tier2Weights map[string]decimal.Decimal
}

func d(n int64) decimal.Decimal { return decimal.NewFromInt(n) }

func mustProfile(name string, tier1Share, tier2Share int64, tier1 map[Tier1Axis]int64, tier2 map[string]int64) Profile {
    p := Profile{Name: name, Tier1Share: d(tier1Share), Tier2Share: d(tier2Share),
        Tier1Weights: map[Tier1Axis]decimal.Decimal{}, Tier2Weights: map[string]decimal.Decimal{}}
    for k, v := range tier1 {
        p.Tier1Weights[k] = d(v)
    }
    for k, v := range tier2 {
        p.Tier2Weights[k] = d(v)
    }
    if err := ValidateProfile(p); err != nil {
        panic(fmt.Sprintf("built-in profile %q is invalid: %v", name, err)) // mirrors Python: an invalid built-in profile crashes at import time
    }
    return p
}

var Profiles = map[string]Profile{
    "simple_implementation": mustProfile("simple_implementation", 80, 20,
        map[Tier1Axis]int64{AxisIntelligence: 1, AxisCost: 5, AxisSpeed: 5},
        map[string]int64{"instruction_following": 5}),

    "simple_action_execution": mustProfile("simple_action_execution", 65, 35,
        map[Tier1Axis]int64{AxisIntelligence: 1, AxisCost: 5, AxisSpeed: 5},
        map[string]int64{"instruction_following": 5, "evidence_capture": 5, "agentic_tools": 3, "software_engineering": 2}),

    "balanced_implementation": mustProfile("balanced_implementation", 70, 30,
        map[Tier1Axis]int64{AxisIntelligence: 3, AxisCost: 3, AxisSpeed: 3},
        map[string]int64{"software_engineering": 5, "instruction_following": 3, "agentic_tools": 2}),

    "complex_implementation": mustProfile("complex_implementation", 60, 40,
        map[Tier1Axis]int64{AxisIntelligence: 5, AxisCost: 1, AxisSpeed: 1},
        map[string]int64{"software_engineering": 5, "planning_capability": 4, "instruction_following": 2}),

    "ui_ux": mustProfile("ui_ux", 60, 40,
        map[Tier1Axis]int64{AxisIntelligence: 3, AxisCost: 2, AxisSpeed: 3},
        map[string]int64{"ui_visual": 5, "software_engineering": 4, "instruction_following": 3, "evidence_capture": 2}),

    "complex_action_execution": mustProfile("complex_action_execution", 60, 40,
        map[Tier1Axis]int64{AxisIntelligence: 4, AxisCost: 2, AxisSpeed: 2},
        map[string]int64{"agentic_tools": 5, "instruction_following": 4, "evidence_capture": 2}),

    "financial_work": mustProfile("financial_work", 60, 40,
        map[Tier1Axis]int64{AxisIntelligence: 5, AxisCost: 1, AxisSpeed: 2},
        map[string]int64{"finance": 5, "knowledge": 4, "reasoning": 4, "research": 3, "instruction_following": 3}),

    "research": mustProfile("research", 60, 40,
        map[Tier1Axis]int64{AxisIntelligence: 4, AxisCost: 2, AxisSpeed: 2},
        map[string]int64{"research": 5, "knowledge": 4, "reasoning": 3, "instruction_following": 2, "agentic_tools": 2}),

    "planning": mustProfile("planning", 60, 40,
        map[Tier1Axis]int64{AxisIntelligence: 5, AxisCost: 1, AxisSpeed: 1},
        map[string]int64{"planning_capability": 5}),

    "orchestration": mustProfile("orchestration", 60, 40,
        map[Tier1Axis]int64{AxisIntelligence: 5, AxisCost: 5, AxisSpeed: 4},
        map[string]int64{"planning_capability": 5, "instruction_following": 5}),

    "review": mustProfile("review", 65, 35,
        map[Tier1Axis]int64{AxisIntelligence: 4, AxisCost: 3, AxisSpeed: 2},
        map[string]int64{"instruction_following": 5, "software_engineering": 4, "reasoning": 4, "security": 3, "evidence_capture": 2}),
}
```

`mustProfile` runs `ValidateProfile` and panics on failure, exactly mirroring `_profile`'s import-time crash — all 11 built-in profiles are guaranteed valid by construction (a Go `init()`/package-level `var` initializer panicking on a bad built-in is the correct equivalent of Python failing at import time; a `which-model` `TestBuiltinProfilesAreValid` test MUST also assert `Profiles` is non-empty and every entry passes `ValidateProfile` explicitly, since a panicking package var makes the failure mode a build/startup crash rather than a clean test failure otherwise).

### 5.2 `ValidateProfile` rules — verbatim (`§6.2`, `rank_models.py:80-103`)

1. `Tier1Share > 0` and `Tier2Share >= 0`, else error "tier 1 share must be positive and tier 2 share cannot be negative".
2. `Tier1Share + Tier2Share == 100` exactly, else error "tier 1 and tier 2 shares must sum to 100".
3. `Tier1Weights` keys must be EXACTLY `{intelligence, cost, speed}` — missing or unknown keys both error, naming which.
4. Every Tier-1 weight must satisfy `0 < w <= 5`.
5. Every `Tier2Weights` key must be a member of `CategoryNames` (12 names) — unknown category errors.
6. Every Tier-2 weight must satisfy `0 < w <= 5` (a category is omitted from the map entirely to skip it — it may never be present with weight 0).

### 5.3 Tier1/Tier2 combination arithmetic — including the no-tier-2 asymmetry (`§6.3`, `rank_models.py:378-435`)

For each candidate row:

1. `tier1Values[axis] = ParseScore(row, Tier1ScoreColumn[axis])` for each of the 3 fixed axes.
2. **If any of the 3 Tier-1 scores is missing, the row is EXCLUDED** with reason `"missing_tier1:<comma-joined-missing-axis-names>"` — no imputation, hard cut. This exclusion happens BEFORE any availability filtering (§5.5).
3. `tier1Score = Σ(tier1Values[axis] * profile.Tier1Weights[axis]) / Σ(profile.Tier1Weights[axis])` — a weighted average over exactly the 3 fixed axes, independent of `Tier1Share`.
4. For each category in `profile.Tier2Weights`: fetch `{category}_score` from the row; if blank, append the category to `missingOptional` and (later) a warning, but do NOT exclude the row.
5. If at least one category value was found: `tier2Score = Σ(categoryValue[c] * weight[c] for c in found) / Σ(weight[c] for c in found)` — the weighted average is **renormalized over only the categories that had data**; a missing category is excluded from both numerator and denominator, never treated as zero.
6. **If NO category value was found at all** (every requested tier-2 category was blank for this row) AND `profile.Tier2Weights` is non-empty: `tier2Score = nil`, with warning `"no optional task-category scores available; Tier 1 score used"`.
7. Final combination — **the asymmetry**:
   - If `tier2Score == nil`: `totalScore = tier1Score`; `tier1Contribution = tier1Score`; `tier2Contribution = 0`. **The row's total is the RAW, un-shared Tier-1 score — it is NOT scaled by `Tier1Share`.**
   - Else: `tier1Contribution = tier1Score * Tier1Share / 100`; `tier2Contribution = tier2Score * Tier2Share / 100`; `totalScore = tier1Contribution + tier2Contribution`.

   This means a row with zero tier-2 evidence can score numerically **higher** than an otherwise-identical row that does have tier-2 data, because the no-tier-2 row skips the `* Tier1Share / 100` scale-down entirely while the tier-2-having row's total is capped at `Tier1Share`% of its raw Tier-1 score plus at most `Tier2Share`% of 100. This is Python's documented intentional "don't punish missing data" design (`§6.3`, closing note), not a bug — **the Go port MUST preserve it exactly**, including in test fixtures that assert relative ranking order.

### 5.4 Tie-breaking — verbatim 7-key tuple (`§6.4`, `rank_models.py:439-449`)

Sort candidates by, in order (all descending except the two casefold name keys, which are ascending):

1. `totalScore` DESC
2. raw Tier-1 intelligence axis score DESC
3. `tier2Contribution` DESC
4. raw Tier-1 speed axis score DESC
5. raw Tier-1 cost axis score DESC
6. `model` name, case-folded, ASC
7. `reasoning`, case-folded, ASC

Go: implement as a `sort.Slice` comparator or a `less` function checking each key in this exact order with early return on the first non-equal key — do not substitute a different composite key or a stable-sort-and-hope-for-determinism approach; the tuple's exact order is behaviorally load-bearing (verified today by `test_optional_values_are_weighted_and_top_n_is_deterministic`, §9.2).

### 5.5 Exclusion rules, in application order (`§6.5`)

1. **Missing any Tier-1 score** → excluded, `reasons: ["missing_tier1:…"]`. Applied first, to every row, independent of any availability filter.
2. **Availability filter, applied LAST** — after every complete row has already been scored. A row whose `(model, reasoning)` identity is not in the caller-supplied available set is excluded with `reasons: ["not_live_available"]`. This ordering is deliberate: it lets the ranker report `missing_tier1` (a data-completeness problem) as a structurally distinct exclusion from `not_live_available` (a runtime-availability problem) even for a row that would fail both, and it means availability filtering never masks a Tier-1 completeness defect in the underlying catalog.
3. If zero candidates remain after both filters, one of two **distinct terminal errors**:
   - An availability filter was supplied and zero candidates survive it → `"no candidates remain after live model-and-effort availability and Tier 1 filtering"`.
   - No availability filter was supplied (every row failed Tier-1 completeness alone) → `"no candidates contain all mandatory Tier 1 scores"`.

   These two error strings MUST stay distinct in the Go port — a caller (or `which-model explain`) uses the string to distinguish "your catalog has no usable data" from "your provider/model availability is too narrow", and the exit-code contract (`exit 3`, no viable candidate) is shared by both but the message is not.

### 5.6 Warning strings — verbatim (`§6.6`)

- `"missing optional category scores: <names>"` — one or more (but not all) requested tier-2 categories were blank for this row.
- `"no optional task-category scores available; Tier 1 score used"` — ALL requested tier-2 categories were blank for this row (only emitted when `profile.Tier2Weights` is non-empty).

### 5.7 `--available` mechanics → first-class route store

Reproduce `rank_models.py`'s identity/availability parsing (`§6.7`) as the *fallback* input format for `internal/routing` (§7): separators tried in priority order `"|"`, `"::"`, `","`, `"/"` via a last-occurrence split; both halves non-blank after trimming or error. JSON array elements may be a plain string (parsed via the separator rule), an object `{"model","reasoning"}`, or a 2-element `[model, reasoning]` array. Plain-text fallback: one identity per non-blank, non-`#`-comment line, with an optional `model,reasoning`/`model|reasoning` header line (case/space-insensitive) skipped. Matching against the scores CSV is **exact tuple membership**, no fuzzy/substring/case-insensitive matching, no second cleaning pass on the model name (the scores CSV's `model` value is assumed already clean per §3.1). §7.4 specifies how this file format becomes a persisted `Route` store rather than a per-invocation flag.

### 5.8 Output JSON schema (`§6.9`)

Preserve the top-level shape (`profile`, `recommendation`, `alternatives`, `excluded`, `candidate_count`, `availability_filter_applied`) and the per-candidate shape (`model`, `reasoning`, `total_score`, `tier1_score`, `tier2_score` (nullable), `tier1_contribution`, `tier2_contribution`, `category_scores` (only populated categories), `warnings`) as the payload of `which-model pick --json` and `which-model explain --json` (exact command/flag mapping is Annex D scope). `_tie_*` internal sort keys are never serialized. Decimal values serialize as JSON numbers via `decimal.Decimal.MarshalJSON` (which, like unmarshal, is text-based and precision-preserving) — this differs from Python's `_json_safe`, which explicitly converts `Decimal` to `float` before `json.dumps` (Python's `json` module cannot natively serialize `Decimal`); the Go port's JSON output is therefore **more precise** than the reference implementation's, which is an acceptable and intentional divergence, not a regression, since Go's `encoding/json` has no float-only numeric-serialization constraint to work around.

---

## 6. Storage (`internal/catalog/csvstore/`)

### 6.1 Raw CSV schema (`available_model_raw_values.csv` equivalent — `§3.1`)

Core columns, fixed:

| column | type/unit | higher-is-better | Tier-1 mandatory | render precision |
|---|---|---|---|---|
| `model` | string, `CleanModelName` output | n/a | n/a (identity) | verbatim |
| `reasoning` | enum `minimal,low,medium,high,xhigh,max,default` | n/a | n/a (identity) | verbatim |
| `intelligence_index` | decimal | higher | yes | 1dp |
| `time_per_intelligence_index_task_seconds` | seconds | **lower** | no (optional) | 0dp |
| `cost_per_intelligence_index_task_usd` | USD | **lower** | yes | 2dp |
| `median_end_to_end_response_time_seconds` | seconds | **lower** | yes | 0dp |
| `artificial_analysis_coding_index` | decimal | higher | no (optional) | 1dp |
| `artificial_analysis_agentic_index` | decimal | higher | no (optional) | 1dp |
| `benchmark:<name>` (dynamic) | decimal | higher (always) | no | 1dp |

All numeric cells are `decimal.Decimal`, rendered per §1.1/§4.2 rounding, blank string for unset. Row identity `(model, reasoning)` is unique — a duplicate is a hard validation error.

### 6.2 Scores CSV schema (`available_model_scores.csv` equivalent — `§3.2`)

Column order, fixed: `(model, reasoning)` + one `{metric}_score` per `CoreMetrics` entry (Tier-1 first) + the 12 `CategoryScoreColumns` (§4.8 order) + one `benchmark:<name>` column per dynamic name, in the same order as the raw CSV's dynamic columns (holding the per-benchmark *normalized score*, not the raw value). Every score cell is an integer `"0"`.."100"` string or blank ("not computable", never zero-imputed — §4.7). Only rows with all 3 `RequiredTier1Metrics` populated survive from the raw CSV.

### 6.2a Raw-CSV provenance hash and staleness

The scores CSV is derived from a specific raw CSV (§6.1) plus a specific `benchmarks.toml`, under a specific `Normalizer`/`Aggregator` pair (§4.0 already requires the latter two names be recorded in artifact metadata). To detect when the scores artifact has fallen behind a raw CSV that was refreshed independently (`--refresh-benchmarks` without a following `--refresh-scores`, [master plan §7.5](./README.md) staleness rule), the scores artifact additionally records:

- `raw_sha256` — sha256 over the raw CSV's exact on-disk bytes as written by `ReplaceWithBackup` (§6.4), computed once at the end of Derive and never recomputed from a re-serialization (recomputing from parsed rows risks a byte-format drift the hash is supposed to catch).
- `normalizer` / `aggregator` — the `Name()` strings already required by §4.0.

**Mechanism: a header comment line, not a sidecar file or a metadata column.** The scores CSV is a strict, closed-schema artifact (§6.2: fixed column order, every score cell a `"0"`.."100"` integer string or blank) — adding a `raw_sha256` *column* would corrupt that schema (every row would carry a constant, non-score value, and `internal/pick`'s `LoadScoreRows` would need a special-cased column to ignore). A sidecar file (`available_model_scores.csv.meta.json`) is rejected because it can be copied, committed, or deleted independently of the CSV it describes, reintroducing exactly the two-artifacts-can-drift problem the hash exists to prevent. Instead, Go's `encoding/csv` writer is preceded by a single `#`-prefixed comment line as the literal first line of the file, before the header row:

```
# which-model-scores-provenance raw_sha256=<64-hex> normalizer=minmax-linear aggregator=weighted-arithmetic-mean
model,reasoning,intelligence_index_score,...
```

`encoding/csv.Reader` does not natively skip `#` comments (unlike Python's `csv` module usage here, which the port does not inherit — this is new to the Go port, not a reproduction of existing Python behavior); `internal/catalog/csvstore`'s reader MUST strip and parse exactly one leading `#`-prefixed line before handing the remainder to `csv.NewReader`, and MUST hard-error if that line is present but does not parse as the expected `key=value` shape (malformed provenance is not silently ignored). A scores CSV with no leading `#` line at all (e.g. hand-edited, or written by a version predating this rule) is treated as provenance-unknown, not stale — no warning, since there is nothing to compare against.

**Read-time comparison**: on every scores-CSV read, `internal/catalog/csvstore` also reads the raw CSV's current on-disk bytes, computes its sha256, and compares to `raw_sha256` from the scores header. A mismatch emits **exactly one** staleness warning naming both artifacts's paths and instructing the operator to run `--refresh-scores`; it is never a hard error, since a stale scores CSV is still usable — `which-model pick` degrades exactly as it does for a stale route table (`§7.2`).

**This is deliberately the same mechanism as §7.2's route-table staleness**, not a second one: both record a content hash of the upstream artifact they were derived from, both compare that hash at read time, and both surface a mismatch as a warning rather than a hard error. §7.2's route table hashes the scores CSV it was built against; this section's scores CSV hashes the raw CSV it was built against — the same pattern applied one layer further upstream, so an operator learns the same staleness idiom once and it holds at every stage boundary in the pipeline.

### 6.3 The benchmark column set is dynamic — MUST NOT be hardcoded

`benchmarks.toml` currently declares 11 groups (`software_engineering, reasoning, knowledge, research, instruction_following, agentic_tools, evidence_capture, ui_visual, security, data_ml, finance`) under `[benchmark_selection].groups`, each backed by a `[benchmark_groups.<name>]` table with a `benchmarks` string-array, plus a top-level `[benchmark_selection].benchmarks` array of directly-named benchmarks not tied to any group. Expansion algorithm (`§3.3`, `model_config.py:70-78`): concatenate `[configured[group].benchmarks for group in selected groups, in declared order]` then the direct `benchmarks` array, then de-duplicate keeping first occurrence. **This currently expands to 51 unique benchmark names (§3.3's numbered table), not the 24 that happen to be persisted in the checked-in raw/scores CSVs today** — those CSVs are stale relative to the current `benchmarks.toml` (confirmed by mtime comparison in the source investigation, `§3.3` closing note). The Go port's CSV reader/writer, header validator, and any fixture/golden test **MUST treat the `benchmark:` column set as fully dynamic, resolved from `benchmarks.toml` at run time** — hardcoding either the 24-column or the 51-column snapshot as a fixed struct/schema is a reimplementation defect, not a simplification. `internal/catalog/csvstore` MUST expose a `ResolveBenchmarkColumns(cfg BenchmarkConfig) []string` used identically by the raw-CSV writer, the scores-CSV generator, and any header-diff/migration tooling — there must be exactly one place in the Go codebase that computes this column list.

### 6.4 Atomic write path (`§9` cross-cutting note; verbatim algorithm from `csv_store.py:240-273`)

`ReplaceWithBackup(outputPath string, content []byte, expectedOriginal []byte) (backupPath string, err error)`:

1. Read the current file at `outputPath`; if `expectedOriginal` is supplied and differs from what was just read, fail with a "changed while data was being collected" error (compare-and-swap on the pre-collection read).
2. Compute a candidate backup path `outputPath + "." + UTCTimestamp + ".bak"`; if it exists, append `.1`, `.2`, … until a free name is found (never overwrite an existing backup).
3. Create a temp file in the **same directory** as `outputPath` (required for the final rename to be atomic on the same filesystem/volume), write `content`, flush, `fsync` the file descriptor.
4. Re-read `outputPath` and compare to the original bytes from step 1 — if it changed underneath us during step 3, abort (no backup written, temp file removed).
5. Write the backup file using **exclusive create** (`os.OpenFile(backupPath, O_WRONLY|O_CREATE|O_EXCL, 0o644)` — fails if the path already exists, closing the TOCTOU gap left by step 2's existence check), write the original bytes, flush, `fsync`.
6. Re-read `outputPath` again and compare to the original — if changed, delete the just-written backup and abort.
7. `os.Rename(tempPath, outputPath)` — atomic replace on POSIX filesystems (the target platforms for `which-model`'s catalog refresh: local dev macOS/Linux and the GitHub Actions `ubuntu-latest` runner, §8; the port does not need Windows-safe replace semantics for this path).
8. On any I/O error at any step, clean up the temp file (if not yet consumed by the rename) and return a wrapped error; the original file is guaranteed untouched by any failure before step 7.

Any interruption before step 7 leaves the original file byte-identical to what it was before the call started — this is the guarantee `internal/catalog/csvstore` tests must assert directly (§9.4, "atomic backup/replace" group).

### 6.5 Strict/closed TOML schema validation

`providers.toml` and `benchmarks.toml` are closed schemas: an unknown key at any level, a blank list entry, or a duplicate list entry is a **hard error**, not a warning — this is load-bearing for the Python test suite (`test_provider_config_rejects_invalid_shape_keys_and_exclusions`, `test_benchmark_config_rejects_unknown_groups_and_invalid_entries`, §9.4) and MUST be preserved with equal strictness in Go.

Go TOML library: **`github.com/BurntSushi/toml`**. Decode via `toml.Decode(data, &target)` to get a `toml.MetaData` alongside the typed struct, then call `meta.Undecoded()` — any non-empty result is an unknown-key hard error naming the offending key path (mirrors Python `tomllib`'s document being checked key-by-key against the exact expected shape). Structural rules to enforce explicitly after decode (BurntSushi/toml does not do these itself):

- `providers.toml`: top-level document must decode to exactly `{providers: {<id>: {excluded_models: []string}}}` (`§2`); each `excluded_models` entry must be non-blank after trim, and the list must be de-duplicated (a duplicate entry is a hard error, not silently collapsed).
- `benchmarks.toml`: `[benchmark_selection].groups` entries must each name a `[benchmark_groups.<name>]` table that exists; each group's `benchmarks` list and the top-level direct `benchmarks` list must contain non-blank, trimmed strings (duplicates across the *same* list are a hard error; duplicates *across different* groups are expected and handled by the dedup-on-expansion rule in §6.3, which is a different, later step from this validation).

---

## 7. Routing join (`internal/routing/`) — new in `which-model`

Neither prototype has this layer: `available-model-data-export` only produces `(model, reasoning) → score` rows with no concept of a live usage account, and `usage-allowance-checks` only produces `(provider account) → usage windows` with no concept of a catalog score. `internal/routing` is the join that lets `internal/pick` filter/weight catalog candidates by which provider account can actually serve them right now.

### 7.1 `Route` production

A `Route` (contract type, reproduced from the master plan for reference — not redefined here) binds a usage provider + provider-native model id to a catalog `(Model, Reasoning)` identity:

```go
type Route struct {
    Provider   string     // usage provider id, e.g. "claude", "codex", "copilot" — Annex A namespace
    ModelID    string     // provider-native model id, e.g. "claude-opus-4-5-20251101"
    Model      string     // catalog display name; joins ScoreRow.Model
    Reasoning  string     // joins ScoreRow.Reasoning
    WindowIDs  []string   // which usage Window IDs (Annex A) gate this route
    Provenance Provenance // how this route was established
}

// Provenance records the source that established a Route, so a consumer can
// judge its confidence. Ordered most- to least-authoritative.
type Provenance string

const (
    ProvenanceProviderLive Provenance = "provider_live" // the provider's own live model list; requires credentials
    ProvenanceModelsDev    Provenance = "models_dev"    // models.dev catalogue; unauthenticated
    ProvenanceUserDeclared Provenance = "user_declared" // hand-authored in routes.toml
)
```

Production algorithm (`internal/routing/build.go`), run as part of the same refresh that updates the catalog (§8):

1. For each configured usage provider (Annex A `Descriptor.ID`) with `Kind` in `{Subscription, APIKeyBilling}` (gateway/local-tool kinds generally have no models.dev-listed catalog counterpart and are excluded from automatic route building — they MAY be added via a manual override, step 4), fetch that provider's model list. For providers whose usage account maps directly onto a models.dev provider id (e.g. `anthropic`, `openai`, `github-copilot` — the exact three configured in `providers.toml` today, §2), the provider-native model id list and each model's `base_model`/name annotations come straight from the models.dev provider catalogue (§2.2) — `internal/routing` does not re-fetch anything; it consumes the already-collected `internal/catalog/fetch/modelsdev` output for the current refresh cycle.
2. Apply `providers.toml` `excluded_models` (§2.2) — an excluded provider-native model id is never turned into a `Route`.
3. For each remaining provider-native model, compute `Model = CleanModelName(record.name)` (§3.1) and enumerate `Reasoning` from the model's declared effort levels (`reasoning_options[].values`, §2.2; a non-reasoning model gets a single `Reasoning = "default"` route). Look up `(Model, Reasoning)` in the current scores CSV (§6.2, collapsing `default`→`high` per the §3.2 rule since the scores CSV was generated with that collapse applied).
4. **Matching a catalog display name to a provider-native model id MUST fail loud on ambiguity or absence — it MUST NOT guess.** Two distinct failure modes, both hard errors that abort route-table generation for that provider (not silently-skip-the-model):
   - **Absent**: no `(Model, Reasoning)` row exists in the scores CSV for a provider-native model that route-building attempted to match. This is expected and non-fatal at the *model* level (a model can be provider-listed but not yet catalog-scored, e.g. it lacks Tier-1 data, §4.3) — such a model is simply skipped from route production with a warning, NOT a hard error; "fail loud" here means the skip is logged/surfaced, not silently absorbed. This is distinct from case (5) below.
   - **Ambiguous**: a single provider-native model id's cleaned name matches more than one catalog `(Model, Reasoning)` identity that cannot be disambiguated by the model's own declared effort level (e.g. two catalog rows differ only by a reasoning label the provider doesn't expose) — this IS a hard error that aborts route building for that provider entirely, surfaced with both candidate identities named. `internal/routing` MUST NOT pick the "first" or "best-scoring" candidate; an operator must resolve the ambiguity via a manual override (step 5).
5. **Manual override / first-class route store**: a `routes.toml` (or equivalent, Annex D names the exact file) accepts hand-authored `Route` entries in the same `model|reasoning` identity syntax as Python's `--available` file format (§5.7) reused as the *route* format: `provider.model_id = "model|reasoning"` pairs, or the JSON array-of-objects form. This is how Python's ad hoc `--available` filtering mechanism is promoted to a first-class, persisted store rather than a per-invocation CLI flag: `internal/pick`'s availability filter (§5.5, exclusion rule 2) now reads from the persisted `Route` table's identity set instead of a transient file, and a manual override entry always takes precedence over an auto-derived `Route` for the same `(Provider, ModelID)` pair (an override replaces, it does not merge).
6. Persist the resulting route table (auto-derived + manual overrides, overrides winning conflicts) to the local state store (`internal/config`, e.g. a `routes.json`/embedded-DB table alongside the usage cache) with a `RefreshedAt` timestamp per provider. `which-model routes` (Annex D) reads and edits this store; `which-model catalog refresh` and `which-model auth login <provider>` (Annex A/D) both trigger a re-derivation for the affected provider(s).

### 7.1a Source availability under a disabled usage subsystem

`ProvenanceProviderLive` requires provider credentials, so it is available only when the binary was built without `nousage`, usage is enabled, **and** that specific provider is explicitly enabled (Annex A §1a).

With usage disabled at any level, route derivation:

- MUST use `ProvenanceModelsDev` and `ProvenanceUserDeclared` only;
- MUST **skip** the live-provider source without attempting it — no credential read, no network call, so the disabled path cannot raise a macOS Keychain prompt;
- MUST emit **exactly one** warning naming the reduced source set, not one per provider and not one per route. Per-route warnings across a 39-row catalogue would bury the signal and make the default install unusable.

**Fewer routes is not an ambiguity error.** A reduced source set legitimately yields a smaller route table, and that MUST NOT be reported as the hard `Ambiguous` failure from step 4. The distinction: *absence* of a source is expected and warned once; *ambiguity within the sources that are available* still fails loud exactly as specified above. Conversely, a route table built from reduced sources MUST NOT be silently promoted to look authoritative — that is precisely what `Provenance` exists to record.

`which-model routes verify` reports provenance counts (e.g. `provider_live: 0, models_dev: 34, user_declared: 2`) so an operator can see at a glance whether a table was built with or without credentialed confirmation.

### 7.2 Refresh triggers and staleness

A `Route` table is refreshed automatically whenever ANY of its three inputs change: (a) the catalog scores CSV is regenerated (§6.2, since a model may cross the Tier-1-completeness threshold and become newly routable), (b) a usage provider's model list changes (models.dev provider catalogue, polled at the same cadence as the nightly catalog refresh, §8), or (c) `providers.toml`/`routes.toml` is edited. `internal/routing` MUST NOT silently serve a `Route` table computed against a scores CSV that has since been regenerated — the persisted store records the scores-CSV content hash (or generation timestamp) it was built against, and a mismatch at read time is surfaced as a staleness warning (not a hard error — `which-model pick` still functions against a stale route table, degraded, matching the existing `Snapshot.Stale`/`Confidence` pattern used for usage data in Annex A).

### 7.3 Per-model sub-limit window binding

Several usage providers gate specific models under a narrower quota lane than the account's primary window (Annex A owns the *usage-side* representation of these; this section only specifies how a `Route` binds to them):

| provider | sub-limit signal | binds to |
|---|---|---|
| Codex | `additionalRateLimits` (per-model entries, e.g. a GPT-5.x-Codex variant) | `Route.WindowIDs` includes a synthetic window id derived from the sub-limit's own label (Annex A assigns the stable slug; `internal/routing` only references it) — `docs/plan/research/codexbar-provider-survey.md:119,341` |
| Claude | `sevenDayOpus` / `sevenDaySonnet` (model-family-scoped 7-day windows) plus `limits[].scope.model` | a `Route` whose `ModelID` names an Opus-family model gets `WindowIDs = ["5h", "sevenDayOpus"]` (or the Sonnet equivalent); a non-Opus/Sonnet model gets only the account-level windows — `docs/plan/research/codexbar-provider-survey.md:137-139,341` |
| Gemini / Antigravity | Pro/Flash/Flash-Lite per-model-tier 24h windows | `Route.WindowIDs` selects the tier window matching the route's `ModelID` prefix/family (`gemini-*-pro` → Pro window id, etc.) — `docs/plan/research/codexbar-provider-survey.md:341,373` |

The general rule: `Route.WindowIDs` is populated from the `WindowSpec.ID` set a provider's `Descriptor` (Annex A) declares as "model-scoped" (i.e. `Window.ModelScope` is non-empty for that window, per the canonical `Window` type's `ModelScope []string` field) — `internal/routing` matches a route's `ModelID`/`Model` against each candidate window's `ModelScope` entries and adds every match, plus every account-level (non-model-scoped) window unconditionally. A route matching zero model-scoped windows still gets the account-level windows; a route matching more than one model-scoped window (e.g. a model that has both a per-model sub-limit AND falls under a broader tier) gets all of them — `internal/pick`/master-plan band logic, not `internal/routing`, decides how multiple simultaneous windows combine into a single band weight.

---

## 8. GitHub Action (`.github/workflows/update-available-model-data.yml` → `which-model` equivalent)

Current workflow (verified against `available-model-data-export/.github/workflows/update-available-model-data.yml:1-70`):

| aspect | Python prototype | Go port |
|---|---|---|
| Trigger | `schedule: cron "0 6 * * *", timezone Europe/London` + `workflow_dispatch` | driven by `[catalog.publish]` (§8.1) — cron/timezone come from config, `workflow_dispatch` kept unconditionally |
| Concurrency | `group: update-available-model-data-main, cancel-in-progress: false` | **unchanged** — same group name (or the `which-model`-repo-scoped equivalent if the workflow moves repos), same `cancel-in-progress: false` |
| Guard | `if: github.ref == 'refs/heads/main'` | replaced by the per-branch loop over `[catalog.publish].branches` (§8.3) — a single-branch config with `branches = ["main"]` reproduces today's guard exactly |
| Runner/timeout | `ubuntu-latest`, `timeout-minutes: 15` | unchanged |
| Toolchain setup | `actions/checkout@de0fac2…` (pinned SHA, `# v6.0.2` comment) then `actions/setup-python@ece7cb0…` (pinned SHA, `# v6.3.0` comment), Python 3.13 | `actions/checkout@<pinned-sha>` (same pinning style/comment convention) then **`actions/setup-go@<pinned-sha> # vX.Y.Z`** with `go-version` pinned to the repo's `go.mod` directive, PLUS Go build/module caching (`actions/setup-go`'s built-in `cache: true` keyed on `go.sum`, replacing the implicit pip-cache-free Python setup — the Python job had no dependency cache because the collector scripts use only the standard library plus `tomllib`) |
| Refresh steps | `python3 update_raw_values.py` then `python3 generate_scores.py`, two separate `run:` steps | `./which-model catalog refresh` — a single pre-built binary invocation (`go build -o which-model ./cmd/which-model` as its own step) running Collect-then-Derive, never `go run` inside the 15-minute budget (§8.2). Note: this is `catalog refresh` (both stages), NOT the global `--refresh` which would also trigger a usage-cache bypass that is meaningless in CI where no provider credentials exist (§8.5) |
| Secret | `ARTIFICIAL_ANALYSIS_API: ${{ secrets.ARTIFICIAL_ANALYSIS_API }}` env on the refresh step | **unchanged** — same secret name, same env-var name inside the process (§2.4) |
| Test gate | `python3 -m unittest discover -s tests -v` as a required step BEFORE any `git add`/commit | `go test ./internal/catalog/... ./internal/pick/... ./internal/routing/...` as a required step before any `git add`/commit, gated on `run_tests` (default `true`, §8.1) — same fail-closed placement: a test failure blocks the data commit exactly as it does today |
| Staged-commit-only-if-changed | `git add -- <raw csv> <scores csv>`; `git diff --cached --quiet` sets `changed` output; commit/push steps gated on `steps.changes.outputs.changed == 'true'` | **unchanged**, verbatim shell logic — `git add -- <same two output paths, which-model's on-disk equivalents>` then the identical `git diff --cached --quiet` / `$GITHUB_OUTPUT` pattern; no Go-specific replacement needed since this is pure `git` plumbing |
| Commit identity | `github-actions[bot]` name/email, message `"chore(data): refresh available model scores"` | commit message and PR title come from `commit_message`/`pr_title` (default identical to today's string, §8.1); commit identity unchanged |
| Publish target | single branch, direct push | configurable per `[catalog.publish]`: `pull-request` (opens a PR per branch, auto-merge) or `direct-push` (pushes straight to each branch) — §8.4 |

The pinned-SHA-with-version-comment style (`uses: owner/action@<40-char sha> # vX.Y.Z`) already used for `actions/checkout` and `actions/setup-python` MUST be continued for `actions/setup-go`, `gh pr merge`, and any other action the generated Go workflow introduces — this is an existing repo convention, not a new requirement invented for the port, and it MUST be preserved verbatim.

### 8.1 `[catalog.publish]` configuration

Verbatim from the master plan, reproduced here as the authoritative schema for this annex's workflow generator (§8.2):

```toml
[catalog]
raw_csv_path = ""              # empty = resolved cache/repo default
scores_csv_path = ""
provider_config_path = ""      # providers.toml-equivalent
benchmark_config_path = ""     # benchmarks.toml-equivalent
cache_ttl = "24h"
warn_on_stale_scores = true    # raw-CSV hash mismatch warning (§6.2a)

[catalog.publish]
enabled = true
schedule = "0 6 * * *"         # cron; MUST be literal in generated workflow YAML
timezone = "Europe/London"
environment = ""               # optional GitHub Actions environment for scoped secrets
branches = ["main"]            # PLURAL - one PR or push per branch
mode = "pull-request"          # "pull-request" | "direct-push"
auto_merge = true              # pull-request mode only
merge_method = "squash"        # "squash" | "merge" | "rebase"
commit_message = "chore(data): refresh available model scores"
pr_title = "chore(data): refresh available model scores"
pr_labels = ["data", "automated"]
run_tests = true               # fail-closed gate before any commit
```

| key | meaning |
|---|---|
| `raw_csv_path` / `scores_csv_path` | override the resolved default artifact paths (§6.1/§6.2); blank resolves to the repo-relative default |
| `provider_config_path` / `benchmark_config_path` | override `providers.toml`/`benchmarks.toml` locations; blank resolves to the repo-relative default |
| `cache_ttl` | how long a locally cached collector response is trusted before Collect re-fetches it (Annex D CLI scope; unrelated to the raw-CSV staleness hash of §6.2a) |
| `warn_on_stale_scores` | enables/disables the §6.2a read-time staleness warning; does not affect whether the hash is written, only whether a mismatch is reported |
| `enabled` | master switch for the generated Action (§8.5) |
| `schedule` / `timezone` | literal cron and IANA timezone baked into the generated workflow's `on.schedule` (§8.2) |
| `environment` | optional GitHub Actions environment attached to the refresh job; blank emits no `environment:` key |
| `branches` | ordered list of target branches; one publish attempt per branch (§8.3) |
| `mode` | `pull-request` or `direct-push` (§8.4) |
| `auto_merge` | pull-request mode only — enables `gh pr merge --auto` after opening the PR |
| `merge_method` | squash/merge/rebase passed to `gh pr merge` |
| `commit_message` / `pr_title` / `pr_labels` | literal strings/labels used when a change is staged |
| `run_tests` | fail-closed test gate before any commit (§8, "Test gate" row) — MUST default `true`; a repo that disables it accepts the risk explicitly |

### 8.2 Workflow generation: `which-model catalog workflow --write` / `--check`

GitHub Actions cannot read `on.schedule` from a config file at trigger time — the cron expression MUST be literal YAML evaluated by GitHub's scheduler, which never invokes `which-model` to ask what the schedule is. `[catalog.publish]` is therefore not consumed directly by the Action; it is the single source of truth that a **generator** renders into committed YAML:

- `which-model catalog workflow --write` reads `[catalog.publish]` and (over)writes `.github/workflows/refresh-model-data.yml` deterministically from it — same config in, byte-identical YAML out, so regenerating without a config change is a no-op diff.
- `which-model catalog workflow --check` renders the same YAML in memory and diffs it against the committed file, exiting non-zero on any drift. This is designed to run as a CI lint job (on every PR touching `[catalog.publish]` or the workflow file) so a hand-edited workflow or a config change that nobody regenerated for is caught before merge, not discovered at 6am when the schedule silently doesn't match what the config claims.
- Both subcommands operate on the same rendering function; `--check` is not a separate, divergent code path that could drift from `--write`.

### 8.3 Multi-branch publishing

`branches` is processed in listed order, one publish attempt per branch:

- A failure publishing to one branch MUST NOT abort the remaining branches. Reports say `published` only after direct push, `auto-merge-enabled` after GitHub accepts a deferred PR merge, `skipped-no-changes`, or `failed`.
- Exactly one commit is produced from the refresh run and applied to each branch independently.
- The commit-only-if-changed check runs per branch, since a branch that already has current data is skipped, not failed.

### 8.4 `pull-request` vs `direct-push` modes

Both modes MUST be implemented; `mode` selects between them per invocation, not per branch.

- **`pull-request`** (default): opens a PR against the target branch carrying the commit. An optional environment-scoped `CSV_UPDATE_TOKEN` authenticates checkout, push, and PR creation so the PR event triggers required checks; `github.token` is a distinct identity that can approve that PAT-authored PR when repository Actions settings allow approvals. `gh pr merge --auto` then defers the merge until every protection rule passes. Without the optional token, authentication falls back to `github.token` for repositories that do not require this identity split.
- **`direct-push`**: pushes the commit straight to the target branch with no PR. This is the escape hatch for a repo (or a branch within a multi-branch config) that has no branch protection configured and where the overhead of a PR-per-refresh is unwanted. It is deliberately unsafe on a protected branch — pushing directly to a branch requiring PRs will simply be rejected by GitHub, and that rejection is a normal per-branch failure under §8.3's isolation rule, not a special case.

### 8.5 The Action runs the equivalent of `--refresh`; usage is out of scope

The scheduled/dispatched Action step invokes the full Collect-then-Derive process — the same work `--refresh` (without `--refresh-usage`) performs interactively, since the Action holds the AA API key as a repo secret (§2.4) and can therefore run Collect. **The Action never refreshes usage** (`--refresh-usage`'s scope): there are no provider usage credentials in CI, only the AA catalog key, so a usage refresh step would have nothing to authenticate with and is simply absent from the generated workflow — not attempted-and-skipped, not stubbed, absent.

Because Collect requires the AA key, a consumer of `which-model` who has never set `ARTIFICIAL_ANALYSIS_API` cannot run Collect themselves; they rely entirely on the raw and scores artifacts this Action commits to the repository (§2.0). This is the intended division of labour, not a limitation to work around.

### 8.6 `enabled = false`

When `[catalog.publish].enabled` is `false`, `catalog workflow --write` emits no workflow file and, if `.github/workflows/refresh-model-data.yml` already exists from a prior `--write`, removes it. A user who wants only on-demand refresh (`--refresh-benchmarks`/`--refresh-scores`/`--refresh` run by hand or via their own external scheduler) gets no scheduled Action at all — the two modes (scheduled Action, on-demand CLI flags) are non-exclusive, and `enabled = false` is how an operator opts fully out of the first while keeping the second.

### 8.7 Illustrative generated workflow excerpt

The following is representative **generated output** from `catalog workflow --write` against the example config in §8.1 — it is emitted by the renderer, never hand-maintained, and any manual edit to it is exactly the drift `--check` (§8.2) is built to catch:

> **Correction (2026-08-11):** The historical excerpt below is superseded by
> `specs/features/F30-publishing/TASKS.md` F30-T4 and its byte-checked golden
> file. The current renderer uses the standalone Python refresh, optional
> environment-scoped `CSV_UPDATE_TOKEN`, distinct approval identity, and
> outcome-aware reporting.

```yaml
# GENERATED by `which-model catalog workflow --write` from [catalog.publish] — do not hand-edit.
name: refresh-model-data
on:
  schedule:
    - cron: "0 6 * * *" # Europe/London, per [catalog.publish].schedule
  workflow_dispatch: {}
concurrency:
  group: refresh-model-data-main
  cancel-in-progress: false
jobs:
  refresh:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    strategy:
      fail-fast: false
      matrix:
        branch: ["main"] # from [catalog.publish].branches, listed order
    steps:
      - uses: actions/checkout@<40-char-sha> # v6.0.2
        with:
          ref: ${{ matrix.branch }}
      - uses: actions/setup-go@<40-char-sha> # vX.Y.Z
        with:
          go-version-file: go.mod
          cache: true
      - run: go build -o which-model ./cmd/which-model
      - run: go test ./internal/catalog/... ./internal/pick/... ./internal/routing/...
      - run: ./which-model catalog refresh
        env:
          ARTIFICIAL_ANALYSIS_API: ${{ secrets.ARTIFICIAL_ANALYSIS_API }}
      - id: changes
        run: |
          git add -- available_model_raw_values.csv available_model_scores.csv
          git diff --cached --quiet || echo "changed=true" >> "$GITHUB_OUTPUT"
      - if: steps.changes.outputs.changed == 'true'
        run: |
          git -c user.name="github-actions[bot]" -c user.email="github-actions[bot]@users.noreply.github.com" \
            commit -m "chore(data): refresh available model scores"
      - if: steps.changes.outputs.changed == 'true'
        run: gh pr create --base "${{ matrix.branch }}" --title "chore(data): refresh available model scores" --label data --label automated
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      - if: steps.changes.outputs.changed == 'true'
        run: gh pr merge --auto --squash
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## 9. Test port (113 Python tests → Go table-driven packages)

Source: `docs/plan/research/model-data-pipeline-spec.md §8` (full per-test invariant tables). Mapping below groups by target Go package; each row names the invariant class and whether it needs a golden/fixture file or is a pure in-memory unit test.

### 9.1 `internal/catalog/score` ← `tests/test_generate_scores.py` (16 tests, §8.1)

| Go test | kind | invariant |
|---|---|---|
| `TestDynamicBenchmarksNormalizeIndependently` | unit | dynamic `benchmark:` columns get their own min-max-normalized score column with exact expected header set/order |
| `TestGeneratorAgainstCommittedRawFixture` | **golden/fixture** | running the scorer against a checked-in raw-CSV fixture succeeds with expected optional-metric coverage — guards silent schema drift |
| `TestEveryPopulatedScoreIsWholeInteger` | unit | no stray decimals in any non-blank score cell |
| `TestRowsMissingTier1AreOmitted` | unit | a row missing any `RequiredTier1Metrics` entry is excluded, not zero-filled |
| `TestScoreGenerationIsDeterministic` | **golden** | running the generator twice on identical input yields byte-identical output |
| `TestRejectsNonNumericPopulatedValue` | unit | non-numeric string in a populated numeric cell → hard error |
| `TestRejectsNegativeTime` / `TestRejectsNegativeCost` / `TestRejectsNegativeMedianResponseTime` | unit | negative time/cost/response-time values → hard error (3 distinct columns, keep separate test cases) |
| `TestRejectsMalformedRowCellCount` | unit | mismatched CSV cell count → hard error |
| `TestRejectsDegenerateMandatoryRange` (table over each mandatory column incl. response-time) | unit | `min == max` on a mandatory column → hard error, not divide-by-zero |
| `TestOptionalDegenerateRangeLeavesColumnBlank` | unit | same condition on an optional/benchmark column → blank column, not an error |
| `TestBenchmarkAllowsNonPercentageScale` | unit | values outside 0–100 (e.g. 100.1) normalize fine, no implicit range validation |
| `TestAAOptionalMetricsDirectionAndSingletonBlank` | unit | AA coding/agentic index scored higher-is-better, blank when under-populated |
| `TestMissingEitherMandatoryMetricOmitsRowIndependently` | unit (table) | a row missing EITHER median-response-time OR cost is dropped, tested per-metric independently |

### 9.2 `internal/pick` ← `tests/test_model_ranking.py` (10 tests, §8.2)

| Go test | kind | invariant |
|---|---|---|
| `TestAllProfilesHavePositiveMandatoryTier1Weights` | unit | every `Profiles` entry has exactly the 3 `Tier1Axis` keys, all weight `>0` |
| `TestPlanningProfileExactWeights` | unit | `"planning"` profile's `Tier2Weights == {"planning_capability": 5}`, cross-checked against §4.6's planning composite math |
| `TestOrchestrationProfileNoDoubleCounting` | unit | `"orchestration"`: `Tier1Share=60`, `Tier2Share=40`, and its weights don't double-count `planning_capability`'s own already-blended inputs |
| `TestAliasVariantsCountOnceInCategory` | unit | two benchmark columns aliasing to the same `BenchmarkKey` contribute one value to a category mean, not two |
| `TestMissingTier1ExcludesWithoutImputation` | unit | a row missing a Tier-1 score is excluded, no substitute value used |
| `TestMissingTier2WarnsAndUsesTier1` | unit | zero tier-2 data + weighted profile → raw Tier-1 fallback (§5.3 step 6/7) with the exact warning string (§5.6) |
| `TestTier2WeightedAverageAndTopNDeterministic` | unit | weighted tier-2 averaging over only-present categories is correct; `--top`/ranked-slice truncation is stable across repeated runs |
| `TestAvailabilityFilterExactIdentityNoFuzzing` | unit | availability filtering removes exactly the non-matching identities, no fuzzy/substring matching |
| `TestCustomWeightsStillEnforceTier1Completeness` | unit | both inline-JSON and repeated-flag custom-profile paths still enforce tier-1 key-completeness validation (§5.2 rule 3) |
| `TestCLIReturnsSchemaValidRecommendation` | **golden** (schema fixture) | end-to-end `which-model pick --json` output matches the documented JSON schema (§5.8) |

### 9.3 `internal/catalog/fetch` ← `tests/test_model_source_boundaries.py` (18 tests, §8.3)

| Go test | kind | invariant | load-bearing? |
|---|---|---|---|
| `TestModelsDevURLsAreDistinct` | unit | provider-catalogue and benchmark-catalogue URLs are exact-match constants and distinct | — |
| `TestProviderCollectionIsOneUnauthenticatedRequest` | unit (mock transport, request-count assertion) | exactly one HTTP request, zero auth headers | **yes** |
| `TestBenchmarkCollectionIsOneUnauthenticatedRequest` | unit (mock transport) | exactly one HTTP request, zero auth headers | **yes** |
| `TestBenchmarkNameMatchingIgnoresProviderAnnotations` | unit | canonical-id matching by cleaned display name ignores dated-id/`(latest)` annotations | — |
| `TestBenchmarkCLIRequestsOnlyModelsJSON` | unit | standalone collector entry point hits only the benchmark URL, never the provider URL, never sends credentials | — |
| `TestAAIsOnlyAuthenticatedSource` | unit (mock transport, header assertion across all collectors) | across the whole pipeline, only the AA request carries `x-api-key` | **yes** |
| `TestAAFallsBackToFreeOnlyOn403` | unit (mock transport, status-code table) | `/free` fallback triggers ONLY on HTTP 403, never 429/5xx/malformed body | **yes** |
| `TestAAAPICLINeverCallsFreeOrPageEndpoint` | unit | primary-API entry point never calls the free or page endpoint | — |
| `TestPageCLISendsNoAuthHeaders` | unit | public-page collector sends no auth headers, hits only the slug template | **yes** |
| `TestDefaultCollectionNeverCallsPageCollector` | unit | default (non-opt-in) collection path never invokes the page scraper | **yes** |
| `TestMalformedPayloadsFailAtOwnBoundary` | unit (table over both models.dev sources) | malformed JSON names the specific catalogue in the error, not a generic message | — |
| `TestEachCollectorHasDirectCLI` | unit (build/reflection check) | every collector package exposes an independently runnable entry point producing JSON | — |
| `TestWorkflowReferencesExistingEntryPoints` | **golden** (parses the checked-in workflow YAML) | the workflow's `run:`/`uses:` paths reference files that actually exist — catches stale renames | — |
| `TestScoresCSVIsTrackedWhileUserMemoryIgnored` | unit (gitignore-rule assertion) | the scores CSV path is git-tracked while sibling state-dir paths remain ignored | — |
| `TestLegacyScriptsTreeAbsent` | unit | migration-completeness guard — no leftover Python entry points after the Go cutover | — |
| `TestDefaultConfigPathsResolveToRepoRoot` | unit | default config/`.env` paths resolve relative to true repo root regardless of CWD | — |
| `TestAACollectorsSourcePureAndIndependent` | unit (source/import-graph check) | AA collector packages import neither the models.dev provider nor benchmark packages | — |
| `TestBenchmarkCollectorHasNoProviderDependency` | unit (import-graph check) | benchmark-catalogue package imports neither the provider-catalogue package nor its URL constant | **yes** (architectural boundary, §2.3) |

Go import-graph checks (last three rows) are implemented as a `go/packages`-based static check (or `go vet`-style analyzer) run in `go test`, not a string-grep over source text as Python does — same intent, more robust mechanism.

### 9.4 `internal/catalog/{fetch,config,csvstore}` ← `tests/test_update_raw_values.py` (69 tests, §8.4)

Grouped exactly as the source spec groups them; each Go sub-group is a table-driven test function or a small file of related cases in the package it exercises:

| group | target package | kind | load-bearing invariants |
|---|---|---|---|
| Config loading/validation (9 tests) | `internal/catalog/config` | unit | **strict TOML rejection** (unknown keys/blank/duplicate entries, §6.5) — MUST preserve; provider-union dedup/determinism; distinct error message per missing config file |
| models.dev parsing/matching (21 tests) | `internal/catalog/fetch/modelsdev` | unit + 2 golden fixtures (annotated-name normalization, malformed-record rejection cases) | effort/reasoning-option parsing incl. exclusions and deprecated filtering; benchmark extraction scoped to exactly the target names; "with tools" variant resolved by highest value (not harness priority, §2.3) |
| Fail-fast ordering / safety (4 tests) | `internal/catalog` orchestration layer | unit (mock network + mock filesystem, call-order assertions) | validation ordering guarantees zero network calls and zero file mutation before all config/local validation passes; a CSV that changed underneath the process aborts without touching the backup (§6.4 step 4/6) |
| HTTP/TLS/security (3 tests) | `internal/catalog/fetch` transport | unit | default TLS cert/hostname verification preserved (Go default, §2.1); **cross-origin redirect after auth is rejected without forwarding the key** — MUST preserve |
| Secret loading (3 tests) | `internal/catalog/fetch/aa` | unit | env takes precedence over `.env`; `.env` fallback works; missing-key error names both sources |
| AA API mechanics (8 tests) | `internal/catalog/fetch/aa` | unit + 1 golden (pagination-follows-header sequence) | reasoning-variant discovery ordering + explicit-null preservation; required-metric-key-presence (absent key vs. explicit `null`) enforced for both `evaluations` and the cost envelope; malformed envelopes rejected |
| CSV rendering/merge (5 tests) | `internal/catalog/csvstore` | unit | **zero is a valid "win", never falsy-skipped** in merge; merge is per-cell independent within a mixed row; merge is exact-identity-only, no fuzzy matching (§3.4) |
| Public page fallback (6 tests) | `internal/catalog/fetch/aa` (page collector) | unit | exact-slug time binding; API cost takes precedence over page fallback; malformed/negative/ambiguous page data rejected; opt-in-only invocation (§2.5) |
| End-to-end `update()` orchestration (5 tests) | `internal/catalog` orchestration layer | **golden/fixture** (full pipeline run against recorded fixture responses) | default run calls only the v2 API and preserves omitted optional metrics from the existing CSV; partial `--provider` refresh updates only selected providers and preserves the rest (§3.4 `MergePartialRefresh`); missing models.dev benchmarks for a model preserve its exact current CSV values (no accidental blanking) |
| Atomic backup/replace (3 tests) | `internal/catalog/csvstore` | unit (temp-dir filesystem) | backup is byte-identical to pre-update content before the atomic replace; colliding backup filenames get `.1.bak`-style suffixing, never overwritten; a fetch failure leaves the original file untouched and creates no backup (§6.4) |
| HTTP client retry/error semantics (2 tests) | `internal/catalog/fetch` transport | unit | 429 retried per the `Retry-After`/exponential policy (§2.1) without ever logging/exposing the key in the retry path; 401 → key-rejected message, persistent 500 → server-error-persisted message (parameterized over both statuses) |

Total: 16 + 10 + 18 + 69 = 113 Python tests, all mapped. The Go port MUST NOT reduce coverage of the invariants marked **yes**/bolded above (single-unauthenticated-request guarantees, redirect rejection, TLS verification preserved, 403-only free-endpoint fallback, strict TOML rejection, zero-is-a-valid-win merge semantics, and the models.dev provider/benchmark architectural independence) — these are the invariants a careless port is most likely to silently regress, since none of them affect a "happy path" run against well-formed test fixtures.

### 9.5 Additional cases required by the usage toggle and pluggable scoring

Not ports of Python tests — new coverage the §0, §4.0, and §7.1a additions demand:

| Case | Package | Kind | Invariant |
|---|---|---|---|
| Full catalog + ranking suite passes under `-tags nousage` | all catalog/pick packages | build-matrix CI job | §0 usage-independence. Without this job the independence claim rots the first time someone adds a convenience import. MUST run on every change, not nightly |
| `which-model pick --no-usage` reproduces `rank_models.py` byte-for-byte on the committed scores CSV | `internal/pick` | **golden** | Degraded mode is *exactly* the legacy ranker (master plan §6.3). This is M1's acceptance criterion doubling as the degraded-mode criterion — one artifact, two guarantees |
| Default `Normalizer`/`Aggregator` reproduce the committed scores CSV | `internal/catalog/score` | **golden** | §4.0 defaults are behaviour-preserving; proves the interface indirection changed nothing |
| Normalizer/aggregator name recorded in artifact metadata and evidence | `internal/catalog/score`, `internal/pick` | unit | §4.0 traceability. A score whose method is unrecorded is not evidence |
| Absolute columns byte-match the raw CSV for the same identity | `internal/catalog/csvstore` | unit | §4.0a — absolute values are carried through unmodified, so raw and scores are diffable |
| Relative columns all carry the `_score` suffix; no bare `benchmark:<name>` holds a normalized value | `internal/catalog/csvstore` | unit | §4.0a — closes the header-collision trap where identical headers meant different quantities |
| Route derivation with usage disabled emits exactly ONE reduced-source warning | `internal/routing` | unit | §7.1a — per-route warnings across the catalogue would bury the signal |
| Route derivation with usage disabled performs zero credential reads and zero provider network calls | `internal/routing` | unit (injected fs/http spies) | §7.1a skip-without-attempting; the spy asserting *no call* is the whole test |
| Reduced source set yields fewer routes without raising the `Ambiguous` hard error | `internal/routing` | unit | §7.1a — absence and ambiguity are different failures |
| `Provenance` set correctly per source; `routes verify` counts match | `internal/routing` | unit | §7.1a — a table built without credentialed confirmation must be distinguishable from one built with it |
| Derive is byte-reproducible from a fixed raw CSV with no network | `internal/catalog/score` | unit (injected http spy) | §2.0/§4.0 — Derive against a fixed `(raw CSV, benchmarks.toml)` pair is a pure function; the spy asserting zero HTTP calls is the whole test |
| `--refresh-benchmarks` under `--offline` is rejected | Annex D CLI | unit | Annex D §1.6 rule 4 — argument error, exit `2`, since Collect cannot possibly succeed offline |
| Raw-CSV hash mismatch produces exactly one staleness warning, not a hard error | `internal/catalog/csvstore` | unit | §6.2a — mirrors §7.2's route-table staleness; `which-model` still functions against a stale scores CSV |
| `catalog workflow --check` detects drift when config and committed YAML disagree | `internal/catalog` workflow generator | unit (golden YAML fixture) | §8.2 — `--check` renders the same function as `--write` and diffs against the committed file; a manually edited or stale workflow MUST fail the check |
| `enabled = false` emits no workflow (and removes an existing one) | `internal/catalog` workflow generator | unit (temp-dir filesystem) | §8.6 — `catalog workflow --write` MUST NOT emit `.github/workflows/refresh-model-data.yml` and MUST delete it if present |
