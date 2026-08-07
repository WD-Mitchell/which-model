# available-model-data-export — Reimplementation Spec

## 0. Directory map
```
available-model-data-export/
  providers.toml                                       # provider allow-list + excluded_models
  benchmarks.toml                                      # benchmark group->name selection
  available_model_raw_values.csv                       # generated raw metrics (176 lines incl header = 175 data rows)
  .centree-agentic-framework/available_model_scores.csv# generated normalized scores (40 lines incl header = 39 data rows)
  .github/workflows/update-available-model-data.yml    # nightly cron GH Action
  .github/workflows/update_available_model_data/
    http_client.py        # shared HTTP transport (retries, TLS, redirect-block)
    model_config.py        # TOML loaders for providers.toml / benchmarks.toml
    model_types.py          # dataclasses, clean_model_name, UpdateError
    get_provider_models.py  # models.dev/api.json -> per-provider model list
    get_benchmarks.py        # models.dev/models.json -> benchmark attach
    get_aa_api_values.py       # Artificial Analysis v2 API (authenticated)
    get_aa_page_values.py       # AA public model page scrape (opt-in only)
    csv_store.py                # merge/render/atomic-replace raw CSV
    update_raw_values.py         # orchestrator entry point (raw CSV refresh)
    generate_scores.py            # orchestrator entry point (scores CSV generation)
  .agents/skills/meta-orchestration-model-selection/scripts/rank_models.py  # standalone ranker CLI
  tests/
    test_generate_scores.py (16 tests)
    test_model_ranking.py (10 tests)
    test_model_source_boundaries.py (18 tests)
    test_update_raw_values.py (69 tests)
```

---

## 1. Data sources — exact endpoints, auth, pagination

### 1.1 models.dev provider catalogue (`get_provider_models.py`)
- URL: `MODELS_DEV_PROVIDER_URL = "https://models.dev/api.json"` — `get_provider_models.py:26`.
- **No authentication.** Single unauthenticated GET, no pagination (`fetch_provider_models`, `get_provider_models.py:143-155`; confirmed by test `test_provider_collection_is_one_unauthenticated_request`, `test_model_source_boundaries.py:48`).
- Response shape: JSON object keyed by provider id (`anthropic`, `openai`, `github-copilot`, …). Each provider record: `{"id": <provider>, "models": {<model_id>: {"id":..., "name":..., "status": "deprecated"|other|null, "base_model": str|null, "reasoning": bool, "reasoning_options": [{"type":"effort","values":[...]}]|null}}}` — validated at `get_provider_models.py:88-140`.
- Consumed fields per model: `id`, `name`, `status`, `base_model`, `reasoning` (bool), `reasoning_options[].type=="effort"` -> `.values` (effort labels, `"none"`/`"default"` normalized to `"high"`; must be a subset of `REASONING_LEVELS`). Function `_reasoning_levels` (`get_provider_models.py:29-71`).
- Filtering: models with `status == "deprecated"` OR `model_id in exclusions` are dropped (`get_provider_models.py:127`). `name` is passed through `clean_model_name`.

### 1.2 models.dev benchmark catalogue (`get_benchmarks.py`)
- URL: `MODELS_DEV_BENCHMARK_URL = "https://models.dev/models.json"` — `get_benchmarks.py:23`.
- **No authentication.** Single unauthenticated GET, deliberately a *different* URL/domain path from 1.1 (test `test_source_urls_are_distinct_and_current` asserts `MODELS_DEV_PROVIDER_URL != MODELS_DEV_BENCHMARK_URL`, `test_model_source_boundaries.py:43-46`). This module has **no** dependency on `get_provider_models` (asserted by `test_benchmark_module_has_no_provider_collector_dependency`, checking source text excludes `"get_provider_models"` / `"MODELS_DEV_PROVIDER_URL"`, `test_model_source_boundaries.py:345`).
- Response shape: JSON object keyed by **canonical model id** (`"<provider>/<slug>"`), each record: `{"id":..., "name":..., "benchmarks":[{"name": str, "score": number, "variant": str|null, "version": str|null}, ...]}` — validated `get_benchmarks.py:141-246`.
- Matching a `ProviderModel` (from source 1.1) to a canonical record (`get_benchmarks.py:178-217`):
  1. If `entry.canonical_id` (models.dev `base_model`) is set, use it directly (must exist or raise `UpdateError`).
  2. Else try `f"{provider}/{entry.model_id}"` directly.
  3. Else fall back to lookup tables by **id suffix** (`canonical_id.split("/",1)[-1]`) or by **cleaned display name**, merging ALL matches (`by_suffix`, `by_name` dicts) rather than treating multiple hits as ambiguous — evidence from every matching canonical record is merged using "keep highest value" per benchmark name.
- Effort scoping: `_effort(record)` (`get_benchmarks.py:42-56`) parses `record["variant"]` against regex `(?P<effort>minimal|low|medium|high|xhigh|max)(?: effort| reasoning)?(?:, (?:context compaction|with tools))?` or `reasoning effort (?P<effort>none|minimal|low|medium|high|xhigh|max)(?:, ...)?`; `"none"` -> `"default"`. A record with no parseable variant becomes a **default (non-effort-scoped)** value; a record with a parseable effort becomes an **override** scoped to that effort.
- Duplicate resolution: `_resolve()` (`get_benchmarks.py:82-90`) — **always keeps the maximum numeric score** across all records sharing the same `(benchmark name, effort)` bucket, ignoring harness/version metadata entirely. `BENCHMARK_HARNESS_PRIORITY = ("Codex","Claude Code","Terminus-2","Mini-SWE-Agent","Cursor CLI")` (`get_benchmarks.py:24-26`) is a **dead/legacy constant** — comment explicitly says resolution is "intentionally source-agnostic" and uses highest value, not harness priority.
- Version validation `_version()` (`get_benchmarks.py:59-79`) accepts `v?\d+(\.\d+)*` or `"Month YYYY"` strings; used only to validate shape, never to pick a winner.
- Only benchmark names in the caller-supplied `selected_names` (from `benchmarks.toml`) are extracted (`by_name` dict pre-seeded with `{name: [] for name in selected_names}`, `get_benchmarks.py:104`).

### 1.3 Artificial Analysis v2 API (`get_aa_api_values.py`) — the ONLY authenticated source
- Primary URL: `API_URL = "https://artificialanalysis.ai/api/v2/language/models"` — `get_aa_api_values.py:29`.
- Fallback URL (only on HTTP 403, i.e. key lacks paid access): `FREE_API_URL = "https://artificialanalysis.ai/api/v2/language/models/free"` — `get_aa_api_values.py:30`. Fallback logic `fetch_all_models()` (`get_aa_api_values.py:150-160`): catches `UpdateError`, re-raises unless `"HTTP 403"` is in the message, then retries against the free URL. Any other error (429, 5xx, malformed body) is NOT retried against the free endpoint (test `test_artificial_analysis_falls_back_to_free_endpoint_only_when_forbidden`, `test_update_raw_values.py:163`).
- Auth: header `x-api-key: <API_KEY>` (`get_aa_api_values.py` `_fetch_all_models_from_url`, uses `client.get_text(url, headers={"x-api-key": api_key}, ...)`). Env var name: `API_KEY_NAME = "ARTIFICIAL_ANALYSIS_API"` (`get_aa_api_values.py:31`).
  - Key resolution order (`load_named_secret`, `get_aa_api_values.py:55-82`; `load_api_key`, `:85-93`): (1) `os.environ["ARTIFICIAL_ANALYSIS_API"]` if non-blank after strip; (2) else parse `.env` file at `DEFAULT_ENV_PATH = REPOSITORY_ROOT/".env"` — simple `KEY=VALUE` line parser skipping blank/`#` lines, strips matching surrounding quotes, raises `UpdateError` if the key is defined more than once in `.env`. If neither yields a non-blank value -> `UpdateError("missing ARTIFICIAL_ANALYSIS_API; set it in the environment or in {env_path}")`.
  - In CI, the workflow injects it as `env: ARTIFICIAL_ANALYSIS_API: ${{ secrets.ARTIFICIAL_ANALYSIS_API }}` (`.github/workflows/update-available-model-data.yml:31-33`).
- Pagination (`_fetch_all_models_from_url`, `get_aa_api_values.py:110-147`): GET `{api_url}?page={n}` starting at page 1. Response envelope: `{"data": [...], "pagination": {"page": int, "has_more": bool, "total_pages": int}}`. Loop invariants strictly enforced: `pagination.page == requested page`; `1 <= total_pages <= MAX_PAGES(=100)`; when `has_more==false`, `page` MUST equal `total_pages` or raise; when `has_more==true`, `page < total_pages` or raise. JSON is parsed with `parse_float=Decimal` (no float rounding).
- Consumed fields per model item (validated `_selected`, `get_aa_api_values.py:267-313`):
  - `slug` (str), `evaluations.artificial_analysis_intelligence_index`, `evaluations.artificial_analysis_coding_index`, `evaluations.artificial_analysis_agentic_index` (all required keys, value may be `null`),
  - `performance.median_end_to_end_response_time_seconds` (required key, may be null; if present and non-null must be `>= 0`),
  - `artificial_analysis_intelligence_index_cost` (required top-level key; may be `null`; if present must be a dict containing key `cost_per_task` (may be null); if present, `cost_per_task` must be a dict containing key `total_cost` -> the cost value, must be `>= 0`),
  - `evaluations` benchmark fields per `AA_BENCHMARK_FIELDS` (`get_aa_api_values.py:37-52`), reproduced verbatim:
```python
AA_BENCHMARK_FIELDS = (
    ("artificial_analysis_coding_index", "Artificial Analysis Coding Index", False),
    ("artificial_analysis_agentic_index", "Artificial Analysis Coding Agent Index", False),
    ("tau_banking", "τ3 Banking", True),
    ("tau3_banking", "τ3 Banking", True),
    ("tau2_banking", "τ3 Banking", True),
    ("terminalbench_v2_1", "Terminal-Bench", True),
    ("terminalbench_hard", "Terminal-Bench Hard", True),
    ("scicode", "SciCode", True),
    ("ifbench", "IFBench", True),
    ("ifeval", "IFEval", True),
    ("hle", "Humanity's Last Exam", True),
    ("gpqa_diamond", "GPQA Diamond", True),
    ("mmmu_pro", "MMMU Pro", True),
    ("gdpval_aa_normalized", "GDPval-AA", True),
)
```
  Tuple = `(api_field, generated_benchmark_column_name, is_fraction)`. When `is_fraction=True` and `0<=value<=1`, value is multiplied by 100 to become a percentage (`_aa_benchmarks`, `get_aa_api_values.py:234-251`). When multiple `AA_BENCHMARK_FIELDS` rows map to the same column name (e.g. the 3 `tau*_banking` variants -> `τ3 Banking`), the **highest** value wins.
- Name/effort parsing: `_name_parts(name)` (`get_aa_api_values.py:167-177`) splits `"Model Name (config)"` via regex `(.+)\s+\(([^()]*)\)`; the parenthetical is only treated as a reasoning-effort annotation if it matches `\b(?:minimal|low|medium|high|xhigh|extra[- ]high|max(?:imum)?|thinking|adaptive|non[- ]reasoning|reasoning|effort)\b` (case-insensitive), else the whole name (parens included) goes through `clean_model_name`.
- `_reasoning(configuration, slug)` (`get_aa_api_values.py:202-226`) maps configuration text to one of `minimal|low|medium|high|xhigh|max` via regex alternation, supports a `"<Model> <N> fallback"` suffix appended in parens to the label, raises `UpdateError` on ambiguous (>1 distinct alias matched) or unrecognized configuration. `"non-reasoning"` -> `None` (routed to `"default"` row later). `"thinking"`/bare `"reasoning"` -> `"thinking"`; `"adaptive"`/`"adaptive reasoning"` -> `"adaptive"`.
- Family grouping / dedup across dated slugs: `_root_slug(slug)` strips a trailing `-YYYYMMDD` / `-YYYY-MM-DD` date suffix then a trailing `-latest` (case-insensitive) (`get_aa_api_values.py:195-199`). `match_provider_models` groups AA API entries by name token-key, requires all slugs in a name-group to resolve to exactly ONE `_root_slug()` value or the whole group is silently skipped (`len(roots) != 1: continue`).
- Matching a provider model (from 1.1/1.2) to an AA family: `_provider_keys(model)` (`get_aa_api_values.py:180-192`) builds token sets from the cleaned display name and from `model_id` with any `-picker` suffix stripped; if a provider model's key set matches >1 AA family -> `UpdateError` ("ambiguously matches multiple ... families"). Matching a provider model to a *previously seen* aggregate (across providers) also checks `canonical_id` equality; conflicting matches raise `UpdateError`.
- Output rows: one `SelectedModel` per aggregate × per reasoning level observed across providers/AA. If an AA model exists for that exact level it's used as the base (benchmarks merged with any provider-supplied benchmark cells, taking the max per name); if not, the closest AA model whose base effort (before any fallback suffix) matches is used if unambiguous; otherwise no AA financials are attached and only benchmark evidence populates the row.

### 1.4 Artificial Analysis public model page (`get_aa_page_values.py`) — OPT-IN ONLY, not used by the scheduled workflow
- URL template: `MODEL_PAGE_URL = "https://artificialanalysis.ai/models/{slug}"` (`get_aa_page_values.py:16`), slug URL-quoted.
- **No authentication, no custom headers** (test `test_page_cli_requests_only_public_slug_pages_without_headers`, `test_model_source_boundaries.py:211`).
- Parsing: regex-scans raw HTML/JS for `"currentModel":\s*\{` markers, extracts the balanced `{...}` JSON object following each marker (custom brace-matching `_balanced_object`, handles quoted strings/escapes), rejects duplicate keys within an object via `object_pairs_hook`. Only the object whose `.slug` field equals the requested slug is used; >1 match for that slug -> `UpdateError` (ambiguous); markers present but none match the slug -> `UpdateError` (slug mismatch); no markers at all -> `PublicPageMetrics(None, None)`.
- Consumed fields: `intelligenceIndexTimePerTask` (-> `time_per_intelligence_index_task_seconds`, must be `>=0`), and (only when `require_fallback_cost=True`) `intelligenceIndexCostPerTask.cost.total` (-> fallback cost, must be `>=0`).
- Invocation in the pipeline: `update_raw_values.py`'s `OPTIONAL_SOURCES = frozenset({"aa_page"})` (`update_raw_values.py:96`); only triggered via CLI `--add aa_page`. The scheduled `update-available-model-data.yml` workflow never passes `--add`, so this source is never called in production (comment `update_raw_values.py:1-7`; test `test_default_row_collection_never_calls_public_page_collector`).

### 1.5 Rate limiting / retry / transport (`http_client.py`)
- `REQUEST_TIMEOUT_SECONDS = 20`, `MAX_RETRIES = 2` (`http_client.py:14-15`) — so up to 3 attempts total per request.
- Retries on HTTP `429` or `500..599` only; delay = `min(parsed Retry-After, else exponential 2**attempt)` via `_retry_delay` (`http_client.py:33-41`): if `Retry-After` header parses as a float in `[0,10]`, use it verbatim; else `2**attempt` seconds (1,2,4,...). Also retries on `urllib.error.URLError`/`TimeoutError` with plain `2**attempt` backoff, no cap check on Retry-After.
- Non-retryable HTTP errors raise `UpdateError` with a status-specific message: `401` -> "API key was rejected (HTTP 401)"; `403` -> "API access was forbidden for this key (HTTP 403)"; `429` (exhausted retries) -> "rate limit exceeded (HTTP 429)"; `3xx` -> "redirects are not allowed (HTTP {status})"; `5xx` (exhausted retries) -> "server error persisted (HTTP {status})"; else generic `HTTP {status}`.
- **Redirects are always rejected** — `RejectRedirectHandler` overrides `http_error_301/302/303/307/308` to raise instead of following, specifically to prevent an authenticated `x-api-key` header leaking cross-origin (`http_client.py:18-30`).
- TLS: `ssl.create_default_context()`; if the interpreter exposes `ssl.VERIFY_X509_STRICT`, that flag is explicitly cleared (loosens strict X.509 profile checking) but default cert/hostname verification (`CERT_REQUIRED`, `check_hostname=True`) is otherwise preserved (test `test_default_tls_context_preserves_certificate_and_hostname_verification`).
- Request headers sent: `Accept: application/json, text/html;q=0.9`, `User-Agent: centree-model-metrics-updater/1.0`, plus any caller-supplied headers merged in (e.g. `x-api-key` for AA).

---

## 2. Provider model discovery & `providers.toml` application

- `providers.toml` (verbatim):
```toml
[providers.anthropic]
excluded_models = []

[providers.openai]
excluded_models = []

[providers.github-copilot]
excluded_models = ["grok-4.5"]
```
- Loader: `load_provider_config()` (`model_config.py:84-114`). Strict schema: top-level TOML document MUST be exactly `{"providers": {...}}`; each provider table's only allowed key is `excluded_models` (list of non-blank, trimmed, de-duplicated strings) — any other key, blank entry, or duplicate raises `UpdateError`.
- Provider *discovery* itself is entirely delegated to **models.dev/api.json** (source 1.1) — there is no per-provider bespoke endpoint (no direct Anthropic Console API, no OpenAI models API, no GitHub Copilot API call). `providers.toml` keys (`anthropic`, `openai`, `github-copilot`) are looked up directly as top-level keys of the models.dev payload; each must have `record["id"] == provider` or `UpdateError` ("models.dev has no valid provider ...") — `get_provider_models.py:88-93`.
- `resolve_provider_ids(config, providers=None)` (`model_config.py:117-132`): with no explicit `--provider` CLI flags, ALL provider keys in `providers.toml` are selected (`anthropic`, `openai`, `github-copilot`, deterministic dict order = declaration order); duplicates removed, blanks rejected, unknown provider ids rejected against the configured set.
- `excluded_models` application (`get_provider_models.py:118-127`): for each provider, a model is dropped from the discovered set when EITHER `model["status"] == "deprecated"` OR `model_id` (the models.dev dict key, e.g. `"grok-4.5"`) is a member of that provider's `excluded_models` set. Exclusion IDs that no longer exist in the current models.dev catalogue are reported (not fatal) via the `reporter` callback: `f"provider {provider} exclusions absent from current models.dev catalogue: {stale}"`.
- CLI: `python3 get_provider_models.py [--provider-config PATH] [--provider NAME ...repeatable]`; prints JSON `{provider: [{"id":..., "name":..., "reasoning": [...]}]}` (`get_provider_models.py:158-186`).

---

## 3. CSV schemas

### 3.1 `available_model_raw_values.csv`

**Row count today: 175 data rows (176 lines incl. header).** Columns are `CORE_CSV_COLUMNS` (fixed) + dynamic `benchmark:<name>` columns whose set/order is derived from `benchmarks.toml` at generation time (see §3.3 — the on-disk column order below reflects the file as it currently exists on disk, which is **stale**: it contains only the 24 `software_engineering` benchmarks; the *current* `benchmarks.toml` (11 groups) would expand this to 51 benchmark columns on the next `update_raw_values.py` run — see §3.3).

Currently-persisted header (verbatim, 32 columns):
```
model,reasoning,intelligence_index,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index,benchmark:SWE-Bench Verified,benchmark:SWE-Bench Pro,benchmark:SWE-Bench Multilingual,benchmark:SWE-Bench Multimodal,benchmark:DeepSWE,benchmark:Terminal-Bench,benchmark:Terminal-Bench Hard,benchmark:Aider Polyglot,benchmark:SciCode,benchmark:SWE-Atlas Codebase QnA,benchmark:SWE-Atlas Test Writing,benchmark:SWE-Atlas Refactoring,benchmark:FrontierCode,benchmark:FrontierSWE,benchmark:NL2Repo,benchmark:Program Bench,benchmark:SWE Marathon,benchmark:LiveCodeBench,benchmark:LiveCodeBench Pro,benchmark:MCP Atlas,benchmark:Artificial Analysis Coding Index,benchmark:Artificial Analysis Coding Agent Index,benchmark:Toolathlon,benchmark:AutomationBench
```
Core columns, types/units/direction (`CORE_CSV_COLUMNS`, `model_types.py:10-19`; `NONNEGATIVE_RAW_COLUMNS`, `csv_store.py:34-38`; `REQUIRED_TIER1_METRICS`, `generate_scores.py:57-61`):
| column | type/unit | higher-is-better | nullable | notes |
|---|---|---|---|---|
| `model` | string (cleaned display name) | n/a | no | identity part 1 |
| `reasoning` | string enum: `minimal,low,medium,high,xhigh,max` or `default` | n/a | no | identity part 2; `EFFORT_ORDER` (`model_types.py:20-23`) |
| `intelligence_index` | decimal 0-100ish (AA intelligence index) | higher | yes | rendered to 1 dp (`_format(...,"0.1")`, `csv_store.py:212-213,226`); Tier-1 mandatory in scoring |
| `time_per_intelligence_index_task_seconds` | seconds | LOWER is better | yes | must be `>=0`; rendered to 0 dp (`"1"`); optional metric, not mandatory for scoring |
| `cost_per_intelligence_index_task_usd` | USD | LOWER is better | yes | must be `>=0`; rendered 2 dp (`"0.01"`); Tier-1 mandatory |
| `median_end_to_end_response_time_seconds` | seconds | LOWER is better | yes | must be `>=0`; rendered 0 dp; Tier-1 mandatory |
| `artificial_analysis_coding_index` | decimal | higher | yes | rendered 1 dp; optional metric |
| `artificial_analysis_agentic_index` | decimal | higher | yes | rendered 1 dp; optional metric |
| `benchmark:<name>` (dynamic) | decimal (percentage-like) | higher | yes | rendered 1 dp; every dynamic column is treated `higher_is_better=True` |

All numeric cells: `Decimal`, rendered `ROUND_HALF_UP` at the quantum shown, blank string for `None`. Row identity = `(model, reasoning)` unique (`validate_complete_rows`, `csv_store.py:50-75`).

3 verbatim example rows (as populated field=value pairs; blanks omitted):
```
model=Claude Opus 5, reasoning=max, intelligence_index=63.1, time_per_intelligence_index_task_seconds=465, cost_per_intelligence_index_task_usd=2.34, median_end_to_end_response_time_seconds=61, artificial_analysis_coding_index=78.0, artificial_analysis_agentic_index=59.2, benchmark:SWE-Bench Verified=96.0, benchmark:SWE-Bench Pro=79.2, benchmark:SWE-Bench Multilingual=89.5, benchmark:SWE-Bench Multimodal=59.4, benchmark:DeepSWE=68.8, benchmark:AutomationBench=26.0

model=Kimi K2.7 Code, reasoning=default, intelligence_index=43.0, cost_per_intelligence_index_task_usd=0.22, median_end_to_end_response_time_seconds=67, artificial_analysis_coding_index=60.8, artificial_analysis_agentic_index=30.3, benchmark:Program Bench=53.6, benchmark:MCP Atlas=76.0

model=GPT-5.6 Sol, reasoning=max, intelligence_index=60.9, time_per_intelligence_index_task_seconds=269, cost_per_intelligence_index_task_usd=1.23, median_end_to_end_response_time_seconds=149, artificial_analysis_coding_index=77.4, artificial_analysis_agentic_index=57.8, benchmark:SWE-Bench Pro=64.6, benchmark:DeepSWE=72.7, benchmark:Terminal-Bench=88.8, benchmark:Artificial Analysis Coding Agent Index=80.0, benchmark:Toolathlon=58.0
```

### 3.2 `.centree-agentic-framework/available_model_scores.csv`

**Row count today: 39 data rows (40 lines incl. header).** Only rows with ALL 3 `REQUIRED_TIER1_METRICS` populated survive (`generate_scores.py:397-403`), which is why 175 raw rows -> 39 score rows.

Currently-persisted header (verbatim, 44 columns — mirrors the stale 24-benchmark raw CSV; would be 12 metric-score cols + 12 category cols + 51 benchmark cols = 75 columns once regenerated against the current `benchmarks.toml`):
```
model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score,reasoning_score,knowledge_score,research_score,planning_capability_score,instruction_following_score,software_engineering_score,ui_visual_score,agentic_tools_score,finance_score,evidence_capture_score,security_score,data_ml_score,benchmark:SWE-Bench Verified,benchmark:SWE-Bench Pro,benchmark:SWE-Bench Multilingual,benchmark:SWE-Bench Multimodal,benchmark:DeepSWE,benchmark:Terminal-Bench,benchmark:Terminal-Bench Hard,benchmark:Aider Polyglot,benchmark:SciCode,benchmark:SWE-Atlas Codebase QnA,benchmark:SWE-Atlas Test Writing,benchmark:SWE-Atlas Refactoring,benchmark:FrontierCode,benchmark:FrontierSWE,benchmark:NL2Repo,benchmark:Program Bench,benchmark:SWE Marathon,benchmark:LiveCodeBench,benchmark:LiveCodeBench Pro,benchmark:MCP Atlas,benchmark:Artificial Analysis Coding Index,benchmark:Artificial Analysis Coding Agent Index,benchmark:Toolathlon,benchmark:AutomationBench
```
Column groups, order fixed by `generate()` (`generate_scores.py:391-461`): `(model, reasoning)` + `{metric}_score` for each non-benchmark metric in `CORE_METRICS`/`OPTIONAL_METRICS` order + `CATEGORY_SCORE_COLUMNS` (fixed 12, `generate_scores.py:67-80`, verbatim: `reasoning_score, knowledge_score, research_score, planning_capability_score, instruction_following_score, software_engineering_score, ui_visual_score, agentic_tools_score, finance_score, evidence_capture_score, security_score, data_ml_score`) + raw `benchmark:<name>` columns (now holding the PER-BENCHMARK normalized score, not the raw value) in the same order as the input CSV's dynamic columns.
All score cells: integer `0..100` string (min-max normalized then `ROUND_HALF_UP` to quantum `1`), or blank when insufficient evidence. Not nullable per-se — blank means "not computable", never zero-imputed.

3 verbatim example rows:
```
model=Claude Fable 5, reasoning=max, intelligence_index_score=98, cost_per_intelligence_index_task_usd_score=0, median_end_to_end_response_time_seconds_score=49, artificial_analysis_coding_index_score=97, artificial_analysis_agentic_index_score=95, software_engineering_score=80, benchmark:SWE-Bench Verified=91, benchmark:SWE-Bench Pro=100, benchmark:Terminal-Bench=98, benchmark:AutomationBench=0

model=Claude Opus 4.7, reasoning=max, intelligence_index_score=83, cost_per_intelligence_index_task_usd_score=29, median_end_to_end_response_time_seconds_score=89, artificial_analysis_coding_index_score=93, artificial_analysis_agentic_index_score=78, software_engineering_score=79, benchmark:SWE-Bench Pro=35, benchmark:Terminal-Bench=65, benchmark:SWE-Atlas Codebase QnA=100, benchmark:SWE-Atlas Refactoring=100, benchmark:Artificial Analysis Coding Agent Index=32

model=Claude Opus 4.8, reasoning=max, intelligence_index_score=88, cost_per_intelligence_index_task_usd_score=35, median_end_to_end_response_time_seconds_score=88, artificial_analysis_coding_index_score=94, artificial_analysis_agentic_index_score=83, software_engineering_score=71, benchmark:SWE-Bench Verified=31, benchmark:SWE-Bench Pro=79, benchmark:Terminal-Bench=67
```

### 3.3 Full benchmark column set implied by CURRENT `benchmarks.toml` (11 groups, dedup via `dict.fromkeys`, group order per `[benchmark_selection].groups`)
Group order: `software_engineering, reasoning, knowledge, research, instruction_following, agentic_tools, evidence_capture, ui_visual, security, data_ml, finance`. Expansion algorithm: `dict.fromkeys([name for group in selected_groups for name in configured[group]] + direct_benchmarks)` (`model_config.py:70-78`). Computed result (51 unique names, in this exact order — duplicates across groups like `Terminal-Bench`, `Toolathlon`, `MCP Atlas`, `MMMU Pro`, `OSWorld-Verified` are de-duplicated at FIRST occurrence):
```
1 SWE-Bench Verified          14 FrontierSWE                 27 ARC-AGI-2                40 OmniDocBench
2 SWE-Bench Pro               15 NL2Repo                     28 AIME                     41 CyberGym
3 SWE-Bench Multilingual      16 Program Bench               29 HMMT                     42 CTI-REALM
4 SWE-Bench Multimodal        17 SWE Marathon                30 Humanity's Last Exam     43 DSBench-FullStack
5 DeepSWE                     18 LiveCodeBench               31 MMLU-Pro                 44 DSBench-Hard
6 Terminal-Bench              19 LiveCodeBench Pro           32 MMMU Pro                 45 MLE-Bench
7 Terminal-Bench Hard         20 MCP Atlas                   33 BrowseComp               46 SpreadsheetBench
8 Aider Polyglot              21 Artificial Analysis Coding Index   34 DeepSearchQA        47 Finance Agent
9 SciCode                     22 Artificial Analysis Coding Agent Index  35 WideSearch      48 FinanceAgent
10 SWE-Atlas Codebase QnA     23 Toolathlon                  36 IFBench                  49 τ3 Banking
11 SWE-Atlas Test Writing     24 AutomationBench             37 IFEval                   50 GDPval
12 SWE-Atlas Refactoring      25 GPQA Diamond                38 OSWorld-Verified         51 GDPval-AA
13 FrontierCode               26 FrontierMath                39 BabyVision
```
**Important for reimplementation:** the raw/scores CSV column set is NOT hardcoded — it is entirely `benchmarks.toml`-driven and regenerated on every `update_raw_values.py` run. The committed CSVs on disk right now only reflect the `software_engineering` group (24 names) because the other 10 groups were added to `benchmarks.toml` after the CSVs were last regenerated (mtimes: `benchmarks.toml` 9h old vs `available_model_raw_values.csv` 11h old at investigation time). A reimplementation must treat the header as fully dynamic, not copy the 32/44-column snapshot verbatim as a fixed schema.

---

## 4. Model identity & merging

### 4.1 `clean_model_name(value)` (`model_types.py:27-59`)
Strips balanced `()`, `[]`, `{}` groups (including nested, and even when malformed/mismatched — an unmatched opener just suppresses the remainder), then collapses/trims whitespace via `" ".join("".join(kept).split())`. E.g. `"Claude Opus 4.5 [claude-opus-4-5-20251101]"` -> `"Claude Opus 4.5"`; `"Claude Haiku 4.5 (latest)"` -> `"Claude Haiku 4.5"`.

### 4.2 Identity key
Everywhere in the pipeline, model identity = `(model, reasoning)` tuple AFTER: (a) `clean_model_name(model)`, (b) `reasoning == "default"` collapsed to `"high"` in raw-CSV merge contexts (`csv_store._collapse_default_reasoning`, `csv_store.py:124-178`) OR collapsed the same way in score generation (`generate_scores._merge_input_rows`, `generate_scores.py:163-204`) — both use `"high" if row.reasoning == "default" else row.reasoning`. `rank_models.py` and the raw-CSV writer do NOT collapse `default`; they store whatever identity a row already carries, but `load_score_rows` (rank_models.py:174-209) rejects duplicate `(model,reasoning)` identities outright rather than merging.

### 4.3 `_merge_input_rows` (`generate_scores.py:163-204`) — used by `generate_scores.py` when reading the raw CSV
Groups rows by the collapsed `(clean_model_name, reasoning-with-default->high)` identity, in first-seen order. For each subsequent row sharing an identity: for every non-identity column, if the ALREADY-STORED value is `None`, adopt the new value; else if BOTH the stored and new values are `Decimal` AND the column is a `benchmark:` column, keep `max(current, value)` — otherwise the first non-null value wins and later duplicates are silently discarded (no data loss because upstream `_collapse_default_reasoning` already did most of the same merge on the raw CSV itself).

### 4.4 `csv_store.merge_rows` / `merge_partial_refresh` (`csv_store.py:181-209`)
`merge_rows(fresh, current)`: both inputs first passed through `_collapse_default_reasoning`. For each fresh row, if a current row shares its identity, every core metric column takes `fresh value if not None else current value` (fresh always wins when non-null — this is a "refresh" merge, not a max-merge). Benchmark cells: for each fresh benchmark cell that is `None` AND the benchmark name is NOT in `fresh.authoritative_benchmarks` (i.e., NOT a benchmark this refresh explicitly re-scoped/cleared), fall back to the current CSV's value for that cell — this preserves stale-but-still-true benchmark evidence across partial refreshes while allowing an explicit "clear" (`benchmark_clears`, set when an override table exists for that model but doesn't cover a name) to blank a cell instead of resurrecting an old value.
`merge_partial_refresh(fresh, current, refreshed_families, preserve_unselected)`: after `merge_rows`, if `preserve_unselected` (true only when `--provider` explicitly narrows the provider set below the full configured set — `update_raw_values.py update(): providers is not None and set(selected_providers) != set(provider_config.excluded_models_by_provider)`) then rows for models NOT among `refreshed_families` names are appended unchanged from `current_rows` so an unrefreshed provider's data survives a partial run.

### 4.5 `_benchmark_key` alias map (`generate_scores.py:118-133`) — verbatim
```python
def _benchmark_key(value: str) -> str:
    """Return a stable key used to deduplicate benchmark aliases/variants."""

    normalized = unicodedata.normalize("NFKC", value).casefold()
    normalized = normalized.replace("\u2019", "'").replace("`", "'")
    compact = "".join(character for character in normalized if character.isalnum())
    # These are known models.dev aliases/variants. They are one evidence
    # source, not separate votes in a category mean.
    return {
        "financeagent": "financeagent",
        "gdpval": "gdpval",
        "gdpvalaa": "gdpval",
        "humanityslastexam": "humanityslastexam",
        "artificialanalysiscodingindex": "artificialanalysiscodingindex",
        "artificialanalysiscodingagentindex": "artificialanalysiscodingagentindex",
    }.get(compact, compact)
```
(NB: the literal `’` U+2019 right single quote is normalized to ASCII `'` first.) Effect: `"Finance Agent"` and `"FinanceAgent"` collapse to the same key `financeagent`; `"GDPval"` and `"GDPval-AA"` collapse to `gdpval` — both pairs count as ONE evidence source in category averaging (§5), never two.

---

## 5. Scoring algorithm (`generate_scores.py`)

### 5.1 Normalization formula (`generate_scores.py:282-286`)
```python
def normalized_score(value, minimum, maximum, higher_is_better):
    numerator = value - minimum if higher_is_better else maximum - value
    return ONE_HUNDRED * numerator / (maximum - minimum)
```
Min/max computed PER COLUMN over `eligible_rows` only (rows with all 3 Tier-1 metrics present, see 5.3) via `ranges()` (`generate_scores.py:257-279`). A column with a degenerate range (`min==max`) is a hard error UNLESS the column is "optional" (`OPTIONAL_METRICS` or any `benchmark:` column), in which case its score column is entirely blank for every row. A column with zero populated values is a hard error UNLESS optional (blank column).

### 5.2 Rounding/quantum
`SCORE_QUANTUM = Decimal("1")`; every score value is `.quantize(SCORE_QUANTUM, rounding=ROUND_HALF_UP)` -> integer string `0`..`100` (`score()`, `generate_scores.py:289-294`). Category composites and `planning_capability_score` use the same `ROUND_HALF_UP` to quantum `1` (`_rounded_average`, `generate_scores.py:306-312`; `_category_score`, `generate_scores.py:347-388`).

### 5.3 Tier-1 mandatory metrics (`generate_scores.py:57-61`)
```python
REQUIRED_TIER1_METRICS = (
    "intelligence_index",
    "median_end_to_end_response_time_seconds",
    "cost_per_intelligence_index_task_usd",
)
```
A raw-CSV row lacking ANY of these three (after `_merge_input_rows`) is dropped entirely from the scores CSV (`generate_scores.py:397-403`); if this leaves zero eligible rows, `InputError`.

All `CORE_METRICS` with `higher_is_better` flag verbatim (`generate_scores.py:36-42`):
```python
CORE_METRICS = {
    "intelligence_index": True,
    "time_per_intelligence_index_task_seconds": False,
    "cost_per_intelligence_index_task_usd": False,
    "median_end_to_end_response_time_seconds": False,
    "artificial_analysis_coding_index": True,
    "artificial_analysis_agentic_index": True,
}
```
`OPTIONAL_METRICS = {"time_per_intelligence_index_task_seconds", "artificial_analysis_coding_index", "artificial_analysis_agentic_index"}` — these 3 (of the 6 core metrics) may be entirely absent and just leave their `_score` column blank; the other 3 are Tier-1-mandatory.

### 5.4 Tier-2 category composites — `CATEGORY_MINIMUM_EVIDENCE` verbatim (`generate_scores.py:99-111`)
```python
CATEGORY_MINIMUM_EVIDENCE = {
    "reasoning_score": 2,
    "knowledge_score": 2,
    "research_score": 2,
    "instruction_following_score": 2,
    "software_engineering_score": 2,
    "ui_visual_score": 2,
    "agentic_tools_score": 2,
    "finance_score": 2,
    "evidence_capture_score": 2,
    "security_score": 1,
    "data_ml_score": 2,
}
```
(`planning_capability_score` is not in this dict — it has its own fixed-composition rule below and is never a benchmark-group average.) Comment (`generate_scores.py:95-98`): a composite backed by one populated benchmark is too fragile; `security_score` is the sole exception at 1, because it currently has only 2 candidate benchmarks total (`CyberGym`, `CTI-REALM`) so requiring 2 would make the category almost never computable.

`CATEGORY_GROUPS` maps each `*_score` column to its `benchmarks.toml` group id (`generate_scores.py:81-93`, verbatim mapping — `planning_capability_score` intentionally absent):
```python
CATEGORY_GROUPS = {
    "reasoning_score": "reasoning",
    "knowledge_score": "knowledge",
    "research_score": "research",
    "instruction_following_score": "instruction_following",
    "software_engineering_score": "software_engineering",
    "ui_visual_score": "ui_visual",
    "agentic_tools_score": "agentic_tools",
    "finance_score": "finance",
    "evidence_capture_score": "evidence_capture",
    "security_score": "security",
    "data_ml_score": "data_ml",
}
```
Computation `_category_score()` (`generate_scores.py:347-388`) for a non-planning column: look up the group's benchmark name list from `benchmarks.toml`; call `_source_scores(output_row)` to get a `{alias_key: normalized_score}` map (see 5.5); for each name in the group (in `benchmarks.toml` order), dedup via `_benchmark_key`, collect the score if present; if `len(collected) < CATEGORY_MINIMUM_EVIDENCE[column]` -> blank string; else the category score = **unweighted arithmetic mean** of the collected per-benchmark normalized scores, `ROUND_HALF_UP` to integer.

### 5.5 `_source_scores()` — AA-index-preferred-over-models.dev rule (`generate_scores.py:315-344`, verbatim structure)
```python
def _source_scores(output_row: Mapping[str, str]) -> dict[str, Decimal]:
    result: dict[str, Decimal] = {}
    for column, source_name in (
        ("artificial_analysis_coding_index_score", "Artificial Analysis Coding Index"),
        ("artificial_analysis_agentic_index_score", "Artificial Analysis Coding Agent Index"),
    ):
        value = _decimal_score(output_row.get(column))
        if value is not None:
            result[_benchmark_key(source_name)] = value
    for column, value in output_row.items():
        if not column.startswith(BENCHMARK_COLUMN_PREFIX):
            continue
        score_value = _decimal_score(value)
        if score_value is None:
            continue
        name = column.removeprefix(BENCHMARK_COLUMN_PREFIX)
        result.setdefault(_benchmark_key(name), score_value)
    return result
```
The two AA index columns (`artificial_analysis_coding_index_score`, `artificial_analysis_agentic_index_score`, already min-max normalized as Tier-1 optional metrics) are inserted FIRST under alias keys `artificialanalysiscodingindex` / `artificialanalysiscodingagentindex`. The subsequent loop over dynamic `benchmark:` columns uses `.setdefault(...)` — so if models.dev ALSO published a benchmark literally named `"Artificial Analysis Coding Index"` or `"...Coding Agent Index"`, its value is IGNORED (the AA-index-derived score already claimed that alias key) — this is the "AA-index-preferred-over-models.dev" rule; it prevents the same underlying signal from being counted twice in a category mean.

### 5.6 `planning_capability_score` — fixed weighted formula (`generate_scores.py:352-368`, verbatim)
```python
components = (
    ("reasoning_score", Decimal("0.40")),
    ("knowledge_score", Decimal("0.30")),
    ("agentic_tools_score", Decimal("0.20")),
    ("research_score", Decimal("0.10")),
)
values = [_decimal_score(output_row.get(column)) for column, _ in components]
if any(value is None for value in values):
    return ""
weighted = sum(value * weight for value, (_, weight) in zip(values, components, strict=True))
return str(weighted.quantize(SCORE_QUANTUM, rounding=ROUND_HALF_UP))
```
All 4 component category scores (`reasoning_score`, `knowledge_score`, `agentic_tools_score`, `research_score` — themselves already computed via §5.4, meaning they already individually satisfied `CATEGORY_MINIMUM_EVIDENCE`) must be non-blank or `planning_capability_score` is blank (no partial credit / no imputation).

### 5.7 Output row assembly order (`generate_scores.py:427-461`)
For each eligible row: identity columns, then per-metric `_score` columns in `metrics` dict order (Tier-1 core metrics first, using `metric_ranges`), then all 12 `CATEGORY_SCORE_COLUMNS` (computed from the metric scores just written — this is why `_category_score` reads from `output_row`, not from raw values), then each dynamic `benchmark:<name>` column re-scored via `score(value, *metric_range, higher_is_better=True)`.

---

## 6. Ranking algorithm (`rank_models.py`)

### 6.1 `PROFILES` — VERBATIM (`rank_models.py:124-171`)
```python
TIER1_COLUMNS = {
    "intelligence": "intelligence_index_score",
    "cost": "cost_per_intelligence_index_task_usd_score",
    "speed": "median_end_to_end_response_time_seconds_score",
}
CATEGORY_NAMES = (
    "reasoning", "knowledge", "research", "planning_capability",
    "instruction_following", "software_engineering", "ui_visual",
    "agentic_tools", "finance", "evidence_capture", "security", "data_ml",
)
CATEGORY_COLUMNS = {name: f"{name}_score" for name in CATEGORY_NAMES}

PROFILES = {
    "simple_implementation": _profile(
        "simple_implementation", 80, 20, {"intelligence": 1, "cost": 5, "speed": 5}, {"instruction_following": 5}
    ),
    "simple_action_execution": _profile(
        "simple_action_execution", 65, 35, {"intelligence": 1, "cost": 5, "speed": 5},
        {"instruction_following": 5, "evidence_capture": 5, "agentic_tools": 3, "software_engineering": 2},
    ),
    "balanced_implementation": _profile(
        "balanced_implementation", 70, 30, {"intelligence": 3, "cost": 3, "speed": 3},
        {"software_engineering": 5, "instruction_following": 3, "agentic_tools": 2},
    ),
    "complex_implementation": _profile(
        "complex_implementation", 60, 40, {"intelligence": 5, "cost": 1, "speed": 1},
        {"software_engineering": 5, "planning_capability": 4, "instruction_following": 2},
    ),
    "ui_ux": _profile(
        "ui_ux", 60, 40, {"intelligence": 3, "cost": 2, "speed": 3},
        {"ui_visual": 5, "software_engineering": 4, "instruction_following": 3, "evidence_capture": 2},
    ),
    "complex_action_execution": _profile(
        "complex_action_execution", 60, 40, {"intelligence": 4, "cost": 2, "speed": 2},
        {"agentic_tools": 5, "instruction_following": 4, "evidence_capture": 2},
    ),
    "financial_work": _profile(
        "financial_work", 60, 40, {"intelligence": 5, "cost": 1, "speed": 2},
        {"finance": 5, "knowledge": 4, "reasoning": 4, "research": 3, "instruction_following": 3},
    ),
    "research": _profile(
        "research", 60, 40, {"intelligence": 4, "cost": 2, "speed": 2},
        {"research": 5, "knowledge": 4, "reasoning": 3, "instruction_following": 2, "agentic_tools": 2},
    ),
    "planning": _profile(
        "planning", 60, 40, {"intelligence": 5, "cost": 1, "speed": 1},
        {"planning_capability": 5},
    ),
    "orchestration": _profile(
        "orchestration", 60, 40, {"intelligence": 5, "cost": 5, "speed": 4},
        {
            "planning_capability": 5,
            "instruction_following": 5,
        },
    ),
    "review": _profile(
        "review", 65, 35, {"intelligence": 4, "cost": 3, "speed": 2},
        {"instruction_following": 5, "software_engineering": 4, "reasoning": 4, "security": 3, "evidence_capture": 2},
    ),
}
```
`_profile(name, tier1_share, tier2_share, tier1: dict[str,int], tier2: dict[str,int])` (`rank_models.py:106-121`) wraps every int in `Decimal` and immediately runs `validate_profile` — an invalid built-in profile would crash at import time, so all 11 above are guaranteed valid by construction.

### 6.2 `validate_profile()` rules — verbatim (`rank_models.py:80-103`)
1. `tier1_share > 0` and `tier2_share >= 0`, else `RankingError("tier 1 share must be positive and tier 2 share cannot be negative")`.
2. `tier1_share + tier2_share == 100` exactly, else `RankingError("tier 1 and tier 2 shares must sum to 100")`.
3. `set(tier1_weights) == {"intelligence","cost","speed"}` exactly — missing or unknown keys both raise `RankingError` naming which.
4. Every tier-1 weight must be `0 < w <= 5`.
5. Every tier-2 category name must be a subset of `CATEGORY_NAMES` (12 names above) — unknown categories raise.
6. Every tier-2 weight must be `0 < w <= 5` (tier-2 categories may simply be OMITTED from the dict — that's how a profile skips a category — but any category present must have positive weight; a category can't be weighted 0).

### 6.3 Tier1/Tier2 combination arithmetic (`rank_models.py:378-435`)
For each candidate row:
1. `tier1_values = {name: score(row, col) for name,col in TIER1_COLUMNS}` (parses `Decimal` or `None`).
2. If ANY of the 3 Tier-1 scores is `None` -> row EXCLUDED with reason `"missing_tier1:<comma-joined-missing-names>"` — no imputation, hard cut.
3. `tier1_score = Σ(tier1_values[name] * profile.tier1_weights[name] for name in TIER1_COLUMNS) / Σ(profile.tier1_weights.values())` — a WEIGHTED AVERAGE over exactly the 3 fixed tier-1 axes (intelligence/cost/speed), independent of tier1_share.
4. For each category in `profile.tier2_weights`: fetch `{category}_score`; if missing, add to `missing_optional` and append a warning `"missing optional category scores: <comma-joined>"` but do NOT exclude the row.
5. If ANY category values were found: `tier2_score = Σ(category_values[name]*weight for name in category_values) / Σ(weight for name in category_values)` — i.e. the weighted average is renormalized over ONLY the categories that had data (missing categories don't count as zero, they're simply excluded from both numerator and denominator).
6. If NO category values were found at all (every requested tier-2 category was blank for this row) AND the profile declares any tier-2 weights: `tier2_score = None`, warning `"no optional task-category scores available; Tier 1 score used"`.
7. Final combination:
   - If `tier2_score is None`: `total_score = tier1_score`; `tier1_contribution = tier1_score`; `tier2_contribution = 0`.
   - Else: `tier1_contribution = tier1_score * tier1_share / 100`; `tier2_contribution = tier2_score * tier2_share / 100`; `total_score = tier1_contribution + tier2_contribution`.
   (Note: when `tier2_score` exists, `total_score` is a true percentage-weighted blend on the 0-100 scale since shares sum to 100. When it's absent, `total_score` falls back to the raw un-shared `tier1_score`, which can be numerically HIGHER than a shared blend for an otherwise-identical row that did have tier-2 data — this is an intentional "don't punish missing data" design, not a bug, but reimplementers should preserve it exactly since it affects ranking order.)

### 6.4 Tie-breaking (`rank_models.py:439-449`)
Sort key (all descending except the two case-folded name tiebreaks which are ascending):
```python
key=lambda c: (
    -c["total_score"], -c["_tie_intelligence"], -c["tier2_contribution"],
    -c["_tie_speed"], -c["_tie_cost"],
    c["model"].casefold(), c["reasoning"].casefold(),
)
```
Order: total_score DESC, raw intelligence tier1 score DESC, tier2_contribution DESC, raw speed tier1 score DESC, raw cost tier1 score DESC, model name (casefold) ASC, reasoning (casefold) ASC.

### 6.5 Exclusion rules (in application order)
1. Missing any Tier-1 score column value -> excluded with `reasons: ["missing_tier1:..."]` (this happens BEFORE availability filtering; §6.6).
2. Availability filter (if supplied) applied LAST, after every complete row has already been scored — a row whose identity is not in the `available` set is excluded with `reasons: ["not_live_available"]` (comment at `rank_models.py:407-409` explicitly states this ordering is deliberate).
3. If zero candidates remain after both filters: `RankingError("no candidates remain after live model-and-effort availability and Tier 1 filtering")` if an availability filter was applied, else `RankingError("no candidates contain all mandatory Tier 1 scores")`.

### 6.6 Warning generation
Warnings are per-row, NOT fatal, and accumulate in the row's `warnings: list[str]` output field:
- `"missing optional category scores: <names>"` — one or more (but not all) requested tier-2 categories were blank for this row.
- `"no optional task-category scores available; Tier 1 score used"` — ALL requested tier-2 categories were blank for this row (only when `profile.tier2_weights` is non-empty).

### 6.7 `--available` file/JSON formats & matching semantics
- `_identity(value)` (`rank_models.py:212-222`): given a single string, tries separators `"|"`, `"::"`, `","`, `"/"` in that priority order via `rsplit(sep, 1)` (splits on the LAST occurrence of the first separator found present); both halves must be non-blank after `.strip()` or `RankingError`.
- `_availability_values(path)` (`rank_models.py:225-262`) accepts EITHER:
  1. **JSON** (tried first via `json.loads`): a JSON array whose elements can each independently be:
     - a plain string -> parsed via `_identity()`,
     - an object `{"model": str, "reasoning": str}` -> used directly (no separator parsing),
     - a 2-element array of two strings `[model, reasoning]` -> used directly.
     Anything else -> `RankingError("invalid availability entry: ...")`. Top-level payload must be a `list` or `RankingError`.
  2. **Plain text / CSV-like** (fallback when JSON parse fails): one identity per non-blank, non-`#`-comment line; a first line that case/space-insensitively equals `"model,reasoning"` or `"model|reasoning"` is treated as a header and skipped; every other line goes through `_identity()`.
  Empty result (no identities found) -> `RankingError`.
- `parse_availability(paths, identities)` (`rank_models.py:265-275`): returns `None` (= no filtering) only when BOTH `--available` paths and `--available-identity` values are empty; otherwise unions all file-derived identities with all `--available-identity` CLI values (each parsed via `_identity()`), and requires the union to be non-empty.
- Matching in `rank_models()` is EXACT tuple membership `(model, reasoning) in available` — no fuzzy/substring/case-insensitive matching, no cleaning of the model name a second time (the score CSV's `model` value is assumed already clean).

### 6.8 CLI flags (`parse_args`, `rank_models.py:497-510`)
| flag | type/default | notes |
|---|---|---|
| `--scores` | Path, default `.centree-agentic-framework/available_model_scores.csv` (resolved from `REPOSITORY_ROOT = Path(__file__).resolve().parents[4]`) | |
| `--profile` | choice from sorted `PROFILES` keys, default `"balanced_implementation"` | |
| `--weights-json` | str | inline JSON; mutually exclusive with `--profile` selection logic (see below) and with `--tier1-weight`/`--tier2-weight` (both raise if combined) |
| `--tier1-weight` | repeatable `NAME=VALUE` | e.g. `--tier1-weight intelligence=3` |
| `--tier2-weight` | repeatable `NAME=VALUE` | |
| `--tier1-share` | str, default `"100"` | only used with explicit `--tier1-weight`/`--tier2-weight` |
| `--tier2-share` | str, default `"0"` | ditto |
| `--available` | repeatable Path | file(s) of available identities |
| `--available-identity` | repeatable `MODEL|REASONING` string | inline identities |
| `--top` | int, default `5` | must be `>0` (`RankingError("top-n must be positive")`) |
| `--pretty` | flag | `json.dumps(..., indent=2)` vs compact |
Selection precedence in `main()` (`rank_models.py:513-542`): if `--weights-json` OR (`--tier1-weight` or `--tier2-weight`) given (`explicit=True`) -> build a custom `"custom"`-named profile (error if BOTH `--weights-json` AND repeatable flags given); else use `PROFILES[args.profile]`.

### 6.9 Output JSON schema (`rank_models`, `rank_models.py:472-484`, `_json_safe`, `:487-494`)
Top-level object:
```jsonc
{
  "profile": "<profile name, or \"custom\">",
  "recommendation": { /* one ranked-candidate object, see below */ },
  "alternatives": [ /* ranked-candidate objects, ranks 2..top_n */ ],
  "excluded": [ {"model": str, "reasoning": str, "reasons": [str, ...]}, ... ],  // sorted by (model.casefold(), reasoning.casefold())
  "candidate_count": <int, number of non-excluded ranked candidates>,
  "availability_filter_applied": <bool>
}
```
Each ranked-candidate object (public fields only — `_tie_*` keys are stripped by the `public()` helper, `rank_models.py:452-458`):
```jsonc
{
  "model": str, "reasoning": str,
  "total_score": number,        // Decimal -> float via _json_safe
  "tier1_score": number,
  "tier2_score": number | null,
  "tier1_contribution": number,
  "tier2_contribution": number,
  "category_scores": { "<category>": number, ... },  // only categories with data
  "warnings": [str, ...]
}
```
On CLI failure: prints `f"error: {error}"` to stderr, exit code `1`. Success: prints the JSON object to stdout (unindented unless `--pretty`), exit code `0`.

---

## 7. Error taxonomy

| Exception | Defined | Raised for | Surfaces as |
|---|---|---|---|
| `UpdateError(RuntimeError)` | `model_types.py:62-63` | Any raw-data-collection/config/CSV-IO failure across `http_client.py`, `model_config.py`, `get_provider_models.py`, `get_benchmarks.py`, `get_aa_api_values.py`, `get_aa_page_values.py`, `csv_store.py`, `update_raw_values.py` — malformed TOML/JSON, invalid schema, HTTP failures (auth/rate-limit/5xx/redirect), missing secret, negative/nonfinite metrics, duplicate identities, concurrent-modification of the output CSV during collection, missing output file | `update_raw_values.py main()` catches `UpdateError`, prints `f"error: {error}"` to stderr, `return 1` (process exit code 1); success path prints confirmation and `return 0`. `get_provider_models.py`/`get_benchmarks.py`/`get_aa_api_values.py`/`get_aa_page_values.py` each have identical own `main()` catch->stderr->exit(1) patterns for standalone CLI use. |
| `InputError(ValueError)` | `generate_scores.py:114-115` | Raised only inside `generate_scores.py` — raw-CSV read/parse errors (bad columns, blank required cell, non-numeric, negative time/cost/response-time), degenerate metric ranges, zero eligible rows, unwritable output path. Also re-raised by wrapping any `UpdateError` from `load_benchmark_config` inside `generate()` (`generate_scores.py:397-399`). | `generate_scores.py main()` catches `(InputError, UpdateError)`, prints `f"error: {error}"` to stderr, `return 1`; success `return 0`. |
| `RankingError(ValueError)` | `rank_models.py:43-44` | Raised only inside `rank_models.py` — malformed profile weights/shares, unreadable/malformed score CSV, missing required columns, duplicate identity in score CSV, out-of-range score values, malformed `--available` file/identity, invalid `--weights-json`, non-positive `--top`, zero candidates after filtering. | `rank_models.py main()` catches `RankingError`, prints `f"error: {error}"` to stderr, `return 1`; success prints JSON, `return 0`. |

The scheduled GH Action (`.github/workflows/update-available-model-data.yml`) runs `update_raw_values.py` then `generate_scores.py` as separate `run:` steps with default `bash` `set -e` semantics — a non-zero exit from either script fails that step and the whole job (job `timeout-minutes: 15`); it then runs `python3 -m unittest discover -s tests -v` as a THIRD gating step before any `git add`/commit/push — a test failure blocks the data commit entirely. The commit/push steps are additionally gated on `steps.changes.outputs.changed == 'true'` (computed via `git diff --cached --quiet` after `git add`), so a no-op refresh does not create an empty commit.

---

## 8. Tests — every test function and the invariant it protects

### 8.1 `tests/test_generate_scores.py` (16 tests, class `GenerateAvailableModelScoresTests`)
| test | invariant |
|---|---|
| `test_dynamic_benchmarks_normalize_independently_with_exact_headers` | Dynamic `benchmark:` columns produce their OWN min-max-normalized score column with the exact expected header set/order. |
| `test_committed_raw_values_map_endpoints_and_current_optional_coverage` | Running the generator against the CHECKED-IN `available_model_raw_values.csv` succeeds and produces expected optional-metric coverage (guards against silent schema drift in the committed fixture). |
| `test_every_populated_score_is_rendered_as_a_whole_integer` | Every non-blank score cell in generated output parses as an integer string (no stray decimals from rounding). |
| `test_rows_without_all_tier_one_metrics_are_omitted` | A row missing any of `REQUIRED_TIER1_METRICS` is excluded from the scores CSV entirely (not zero-filled). |
| `test_committed_scores_are_deterministically_regenerated` | Running the generator twice on the same input yields byte-identical output (no ordering/hash nondeterminism). |
| `test_rejects_non_numeric_populated_values` | A non-numeric string in a populated numeric cell raises `InputError`. |
| `test_rejects_negative_time` | Negative `time_per_intelligence_index_task_seconds` raises `InputError`. |
| `test_rejects_negative_cost` | Negative `cost_per_intelligence_index_task_usd` raises `InputError`. |
| `test_rejects_malformed_rows` | A row with a mismatched cell count (too few/many CSV fields) raises `InputError`. |
| `test_rejects_degenerate_metric_ranges` | A mandatory metric column where every value is identical (min==max) raises `InputError` rather than dividing by zero. |
| `test_rejects_degenerate_mandatory_response_time_range` | Same degenerate-range rule specifically enforced for the mandatory `median_end_to_end_response_time_seconds` column. |
| `test_optional_benchmarks_normalize_partial_coverage_and_leave_singletons_blank` | An optional/benchmark column with only 1 populated value (degenerate range) yields a blank score column instead of erroring. |
| `test_dynamic_benchmark_allows_non_percentage_scales` | Benchmark values outside 0-100 (e.g. `100.1`) are accepted and normalized like any other numeric metric — no implicit percentage-range validation. |
| `test_artificial_analysis_optional_metrics_use_correct_direction_and_singletons_blank` | `artificial_analysis_coding_index`/`agentic_index` are scored higher-is-better and blank out correctly when under-populated. |
| `test_rejects_negative_median_response_time` | Redundant/explicit negative-value guard on `median_end_to_end_response_time_seconds`. |
| `test_individual_missing_median_response_time_and_cost_omit_that_row` | A row missing EITHER median response time or cost (both Tier-1-mandatory) is dropped, verified per-metric independently. |

### 8.2 `tests/test_model_ranking.py` (10 tests, class `ModelRankingTests`)
| test | invariant |
|---|---|
| `test_all_profiles_have_positive_mandatory_tier_one_weights` | Every built-in `PROFILES` entry has exactly the 3 `TIER1_COLUMNS` keys, all with weight `>0`. |
| `test_planning_profile_has_exact_rebalanced_composite_weights` | `"planning"` profile's tier2 weights equal exactly `{"planning_capability": 5}`, and cross-checks against `generate_scores._category_score`'s planning composite math. |
| `test_orchestration_profile_has_researched_weights_without_double_counting` | `"orchestration"` profile has `tier1_share=60`, `tier2_share=40`, and its category weights don't double-count planning_capability's own already-blended inputs. |
| `test_alias_variants_are_not_counted_twice_in_a_category` | Two benchmark columns that alias to the same `_benchmark_key` (e.g. `"Finance Agent"` and its variant) contribute ONE value to a category mean, not two. |
| `test_missing_tier_one_excludes_a_row_without_imputation` | A row missing a Tier-1 score is excluded with no substitute/imputed value used in ranking. |
| `test_missing_tier_two_warns_and_uses_tier_one`| A row/profile combo with zero available tier-2 category data falls back to the raw Tier-1 score and emits the exact fallback warning. |
| `test_optional_values_are_weighted_and_top_n_is_deterministic` | Weighted tier-2 averaging over only-present categories is correct, and `--top` truncation/ordering is stable/deterministic across runs. |
| `test_live_availability_filters_exact_identity_without_substitution` | `--available`/live filtering removes exactly the non-matching `(model,reasoning)` identities with no fuzzy substitution. |
| `test_custom_json_and_repeated_weights_require_tier_one` | Both `--weights-json` and repeated `--tier1-weight`/`--tier2-weight` custom-profile paths still enforce the tier-1 completeness/validation rules. |
| `test_cli_returns_machine_readable_recommendation` | End-to-end CLI invocation returns valid JSON matching the documented output schema. |

### 8.3 `tests/test_model_source_boundaries.py` (18 tests, class `SourceBoundaryTests`)
| test | invariant |
|---|---|
| `test_source_urls_are_distinct_and_current` | Provider URL (`models.dev/api.json`) and benchmark URL (`models.dev/models.json`) are exact-match and distinct constants. |
| `test_provider_collection_is_one_unauthenticated_request` | Provider discovery issues exactly ONE HTTP request with no auth headers. |
| `test_benchmark_collection_is_one_unauthenticated_request` | Benchmark attachment issues exactly ONE HTTP request with no auth headers. |
| `test_benchmark_model_name_matching_ignores_provider_annotations` | Canonical-id matching by cleaned display name ignores provider-appended annotations (dated IDs, `(latest)`). |
| `test_benchmark_cli_requests_only_models_json_without_authentication` | The `get_benchmarks.py` standalone CLI hits only `models.json`, never the provider URL, never sends credentials. |
| `test_artificial_analysis_is_the_only_authenticated_source` | Across the whole pipeline, only the AA API request carries the `x-api-key` header. |
| `test_artificial_analysis_falls_back_to_free_endpoint_only_when_forbidden` | The `/free` fallback triggers ONLY on HTTP 403, never on 429/5xx/malformed body. |
| `test_api_cli_requests_only_authenticated_v2_api` | `get_aa_api_values.py` CLI never calls the free endpoint or public page endpoint. |
| `test_page_cli_requests_only_public_slug_pages_without_headers` | `get_aa_page_values.py` CLI sends no auth headers, hits only the `models/{slug}` template. |
| `test_default_row_collection_never_calls_public_page_collector` | Default `collect_rows()` (no `--add aa_page`) never invokes the page scraper. |
| `test_malformed_payloads_fail_at_their_own_boundary` | Malformed provider/benchmark JSON payloads raise `UpdateError` naming the specific catalogue ("provider catalogue"/"benchmark catalogue"), not a generic error. |
| `test_each_source_script_has_a_direct_json_cli` | Every collector module (`get_provider_models.py`, `get_benchmarks.py`, etc.) is independently runnable as a script producing JSON. |
| `test_workflow_references_existing_renamed_entry_points` | The checked-in GH Actions YAML references entry-point paths that actually exist on disk (catches stale renames). |
| `test_score_csv_is_tracked_scope_while_user_memory_remains_ignored` | `.centree-agentic-framework/available_model_scores.csv` is git-tracked (not gitignored) while other paths under that directory (user memory) remain ignored. |
| `test_old_scripts_tree_and_long_entry_point_names_are_absent` | The legacy `scripts/` directory and old monolithic script names no longer exist (migration completeness guard). |
| `test_default_paths_resolve_to_repository_root` | `DEFAULT_BENCHMARK_CONFIG_PATH`/`DEFAULT_PROVIDER_CONFIG_PATH`/`DEFAULT_ENV_PATH` all resolve relative to the true repo root regardless of CWD. |
| `test_artificial_analysis_modules_are_source_pure_and_independent` | `get_aa_api_values.py`/`get_aa_page_values.py` source text contains no `"get_models_dev"` references — architectural isolation from the models.dev collectors. |
| `test_benchmark_module_has_no_provider_collector_dependency` | `get_benchmarks.py` source text contains neither `"get_provider_models"` nor `"MODELS_DEV_PROVIDER_URL"` — the two models.dev collectors are decoupled modules. |

### 8.4 `tests/test_update_raw_values.py` (69 tests, class `UpdateAvailableModelRawValuesTests`, plus helpers `RecordingClient`/`FakeResponse`)
Grouped by concern (all 69 names, file:line):
- **Config loading/validation** (`:260,310,326,350,358,374,393,410,437`): `test_checked_in_provider_config_has_exact_verified_memberships` (providers.toml/benchmarks.toml match exact expected shape), `test_benchmark_groups_custom_direct_order_and_deduplication` (group+direct benchmark ordering/dedup), `test_benchmark_config_rejects_unknown_groups_and_invalid_entries`, `test_missing_configs_report_their_independent_owners` (distinct error messages per missing config file), `test_provider_union_is_deduplicated_and_deterministic` (default = all 3 providers in declared order), `test_missing_or_empty_exclusions_mean_all_known_model_families`, `test_provider_sections_themselves_determine_default_selection`, `test_provider_config_rejects_invalid_shape_keys_and_exclusions`, `test_list_providers_uses_config_without_update_or_network` (`--list-providers` never touches the network).
- **models.dev parsing/matching** (`:456,501,519,530,583,615,649,681,710,738,752,766,819,858,877,946,967,984,1006,1020,1047`): reasoning/effort option parsing incl. exclusions and deprecated filtering, single-fetch-no-credentials guarantee for both models.dev sources, warn-on-missing-selected-benchmark, malformed-record rejection, invalid effort options rejected, annotated-name normalization + duplicate-provider-name merge, benchmark extraction scoped to exactly the target names, malformed/conflicting benchmark records rejected, "with tools" variant preference resolved by highest value (not harness priority), month/year version strings accepted, `base_model` association populates every effort row, benchmarks stay scoped to their originating effort, live effort forms are an exact bounded closed set, `xhigh` effort scoping doesn't leak into other effort rows, `default` reasoning collapses/merges into the `high` row, default+high fresh rows collapse with max-benchmark merge, benchmark merge preserves selected cells and drops removed ones, a new higher benchmark value can correct/override an existing lower cell, unmatched efforts included + overlapping evidence deduplicated, provider display-name vs id conflicts rejected as ambiguous.
- **Fail-fast ordering / safety** (`:1060,1086,1925,1961`): `test_unknown_provider_fails_before_network_backup_or_replacement`, `test_invalid_provider_config_fails_before_secret_network_or_backup`, `test_malformed_current_csv_fails_before_network_or_backup`, `test_concurrent_output_change_fails_without_backup_or_replacement` — validation ordering guarantees no network call or file mutation happens before all config/local validation passes, and a CSV that changed underneath the process (compare-and-swap) aborts without touching the backup/output.
- **HTTP/TLS/security** (`:1117,1124,1148`): `test_default_tls_context_preserves_certificate_and_hostname_verification`, `test_default_network_path_passes_verified_tls_context`, `test_authenticated_cross_origin_redirect_is_rejected_without_forwarding_key` — the `x-api-key` header is never forwarded across a redirected origin.
- **Secret loading** (`:1200,1211,1220`): `test_environment_key_takes_precedence_over_dotenv`, `test_dotenv_key_is_used_as_safe_fallback`, `test_missing_key_has_clear_error`.
- **AA API mechanics** (`:1227,1261,1305,1333,1389,1438,1464,1518`): pagination-follows-header test, reasoning-variant discovery ordering + null-intelligence preservation, optional-metric extraction preserves explicit nulls, AA+models.dev benchmark merge takes highest value, annotated AA snapshots (dated/latest) merge into one family, required-metric-KEY-presence (distinguishing absent key from explicit `null`) enforced for evaluations, same distinguishing rule for intelligence/cost envelope, malformed metric envelopes/non-numeric values rejected.
- **CSV rendering/merge** (`:1548,1724,1733,1740,1786`): `test_rendering_uses_required_round_half_up_precision`, `test_merge_fresh_non_null_values_including_zero_win_for_every_metric` (zero is a valid "win", not falsy-skipped), `test_merge_fresh_null_values_preserve_every_current_metric`, `test_merge_is_independent_per_cell_within_a_mixed_row`, `test_merge_is_exact_identity_only_and_drops_stale_rows` (no fuzzy identity matching in merge).
- **Public page fallback** (`:1586,1598,1628,1658,1671,1688`): exact-slug time binding, API cost takes precedence over page fallback (both exact-slug-bound), malformed/negative/ambiguous page cost data rejected, page parser never reads an unrelated later `currentModel` block, `collect_rows` only fetches page data when explicitly opted in and without an API key, exactly-the-requested optional source is called (no extras).
- **End-to-end `update()` orchestration** (`:1804,1844,1885,1996,2033`): default run calls only the v2 API and preserves omitted optional metrics from the existing CSV, partial `--provider` refresh updates only selected providers' rows and preserves the rest, missing models.dev benchmarks for a model preserve its exact current CSV values (no accidental blanking), `--add` CLI flag rejects unknown sources and dedups repeats, `--benchmark-config`/`--provider-config` CLI flags plus the checked-in workflow both use only the AA secret (no other credential path).
- **Atomic backup/replace** (`:2054,2072,2086`): backup file exists and is byte-identical to the pre-update content before the atomic replace, colliding backup filenames (same timestamp) don't overwrite an existing backup (`.1.bak` suffixing), a fetch failure leaves the original file untouched and creates no backup.
- **HTTP client retry/error semantics** (`:2111,2139`): `test_http_client_retries_rate_limit_without_exposing_key` (429 retried per `_retry_delay` policy, key never logged/exposed in the retry path), `test_http_client_reports_auth_and_persistent_server_errors` (401 -> "API key was rejected", persistent 500 -> "server error persisted", parameterized over both statuses).

---

## 9. Cross-cutting reimplementation notes
- Every numeric value in the pipeline is arbitrary-precision `Decimal` (Python `decimal` module), never IEEE float — a Go/Rust/TS port MUST use an equivalent exact/decimal type (e.g. Go `shopspring/decimal`, Rust `rust_decimal`, TS a `decimal.js`/big-decimal library) to preserve `ROUND_HALF_UP` quantization semantics exactly (`ROUND_HALF_UP` != banker's rounding != naive float round).
- `providers.toml`/`benchmarks.toml` schemas are STRICT/closed — unknown keys, extra table members, blank/duplicate list entries are all hard errors, not warnings. Reimplement this exact strictness; it is load-bearing for the test suite (`test_provider_config_rejects_invalid_shape_keys_and_exclusions`, `test_benchmark_config_rejects_unknown_groups_and_invalid_entries`).
- The raw-values CSV write path is transactional: write to a temp file in the same directory, fsync, verify the target hasn't changed since it was read (compare-and-swap on file bytes), write a timestamped `.bak` copy (fsync'd, exclusive-create, collision-suffixed), verify again, then `os.replace` (atomic rename) (`csv_store.py:248-273`). Any interruption before `os.replace` leaves the original untouched.
- The scores CSV is entirely a pure function of the raw CSV + `benchmarks.toml`; it is regenerated wholesale (no incremental merge) by `generate_scores.py`, unlike the raw CSV which is incrementally merged against its own prior state.