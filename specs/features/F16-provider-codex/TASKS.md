---
kind: feature-tasks
feature: F16-provider-codex
version: "1.0"
task_count: 8
---

# F16 — provider-codex: Tasks

## Task graph

```mermaid
graph LR
    T1[T1 skeleton + Descriptor] --> T2[T2 config parser]
    T1 --> T3[T3 credential loader]
    T1 --> T4[T4 normalization]
    T2 --> T3
    T3 --> T5[T5 fetch + fallback]
    T4 --> T5
    T5 --> T6[T6 error mapping]
    T5 --> T7[T7 canary]
    T4 --> T8[T8 golden fixtures + parity]
    T5 --> T8
```

## Task F16-T1: Create the package skeleton and register the Codex descriptor

**Depends on:** none
**Files:**
- create `internal/usage/provider/codex/codex.go`
- create `internal/usage/provider/codex/codex_test.go`

**Spec references:** `specs/features/F16-provider-codex/SPEC.md §2.4, §2.13, D11-D12`, `specs/features/F16-provider-codex/CONTRACTS.md §2`, `specs/global/CONTRACTS.md §1.3`, `docs/plan/annex-a-provider-matrix.md §3.1, §5`

**Instructions:**
1. Write the test file first: `usage.Lookup("codex")` must succeed after import.
2. Implement `codex.go`: package doc (port of `usage-allowance-checks/lib/codex.mjs`); `const UsageURL = "https://chatgpt.com/backend-api/wham/usage"`; `var FallbackStatuses = map[int]bool{404: true, 405: true, 410: true, 501: true}`; `WithTrustedOrigin`/`TrustedOriginFrom` as plain context-key helpers (unexported key type); `init()` registering the Descriptor literal copied exactly from `specs/features/F16-provider-codex/CONTRACTS.md §2`.
3. Use canonical types from F11 (`usage.Descriptor`, `usage.AuthSource`, `usage.WindowSpec`, `usage.KindSubscription`, `usage.UnitPercent`, `usage.UnitCredits`); do not define or extend them.
4. Until F16-T5, `var Fetch usage.FetchFunc = func(context.Context, usage.Credential, *http.Client) (usage.Snapshot, error) { return usage.Snapshot{}, nil }`.
5. Do not add other source files in this task.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `usage.Lookup("codex")` | ok |
| 2 | descriptor `ID`, `DisplayName` | `"codex"`, `"Codex"` |
| 3 | descriptor `Kind`, `Tier` | `usage.KindSubscription`, `1` |
| 4 | descriptor `Timeout`, `CacheTTL` | `15 * time.Second`, `60 * time.Second` |
| 5 | `Windows` IDs in order | `["5h", "weekly", "credits"]` |
| 6 | all windows `Optional == true`; `credits` `Unit == usage.UnitCredits` | as stated |
| 7 | `Auth` entries | exactly 6 `AuthFile` entries; each `FilePaths == ["$CODEX_HOME/auth.json", "~/.codex/auth.json"]` |
| 8 | `Auth` JSONPaths in order | `["tokens.access_token", "tokens.accessToken", "auth.access_token", "auth.accessToken", "access_token", "accessToken"]` |
| 9 | `UsageURL` | `"https://chatgpt.com/backend-api/wham/usage"` |
| 10 | `FallbackStatuses` | `{404, 405, 410, 501}` exactly |
| 11 | `WithTrustedOrigin` then `TrustedOriginFrom` | round-trips the origin |
| 12 | `TrustedOriginFrom` without `WithTrustedOrigin` | `""` |

**Acceptance criteria:**
- [ ] `go build ./internal/usage/provider/codex/...` succeeds
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] descriptor literal matches CONTRACTS §2 field-for-field
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/codex/...`

## Task F16-T2: Port the Codex config.toml parser

**Depends on:** F16-T1
**Files:**
- create `internal/usage/provider/codex/codex_config.go`
- create `internal/usage/provider/codex/codex_config_test.go`

**Spec references:** `specs/features/F16-provider-codex/SPEC.md §2.2`, `specs/features/F16-provider-codex/CONTRACTS.md §4`, `docs/plan/research/codexbar-provider-survey.md` §2.3 (config.toml layout)

**Instructions:**
1. Write the test file first, table-driven over input text.
2. Implement `ParseConfig(text string) string` as the verbatim port of `parseCodexConfig` (`codex.mjs:8-35`):
   - Split on `\r?\n`; trim each line; skip blank lines and lines starting with `#`.
   - `^\[model_providers\.([A-Za-z0-9_-]+)\]$` opens a provider section (remember the id); any other `[...]` line opens `__other__` (ignored).
   - Assignment regex: `^(model_provider|base_url)\s*=\s*(["'])(.*?)\2\s*(?:#.*)?$` — quoted value, optional trailing comment. Other lines ignored.
   - Root `model_provider` → active provider; root `base_url` → root fallback; provider-section `base_url` → per-provider map.
   - Result: the active provider's base_url when both exist, else the root base_url, else `""`.
3. No file IO in this task (the loader task uses it).

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `model_provider = "trusted"\n[model_providers.trusted]\nbase_url = "https://trusted.example/v1"` (copy from `usage-allowance-checks/tests/usage-allowance.test.mjs` case 5) | `"https://trusted.example/v1"` |
| 2 | same but `base_url = 'https://trusted.example/v1'` (single quotes) | `"https://trusted.example/v1"` |
| 3 | `[model_providers.trusted]\nbase_url = "https://a.example"` (no active provider) | `""` (provider entry without active selection is ignored) |
| 4 | `base_url = "https://root.example"` (root only) | `"https://root.example"` |
| 5 | `model_provider = "trusted"\nbase_url = "https://root.example"\n[model_providers.trusted]\nbase_url = "https://a.example"\n[model_providers.other]\nbase_url = "https://b.example"` | `"https://a.example"` (active provider wins over root and over other providers) |
| 6 | `model_provider = "other"\n[model_providers.trusted]\nbase_url = "https://a.example"` | `""` (inactive provider) |
| 7 | `model_provider = "trusted"\n[model_providers.trusted]\nbase_url = "https://a.example" # comment\n` | `"https://a.example"` (trailing comment stripped) |
| 8 | `[other_section]\nbase_url = "https://b.example"` | `""` (non-provider section ignored) |
| 9 | `model_provider = "trusted"\n[model_providers.trusted]\nbase_url = "https://a.example"\nbase_url = "https://root.example"` | `"https://a.example"` (root assignment after section still scoped) |
| 10 | `` (empty) / `# only comment` | `""` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] case 1 matches `parseCodexConfig` from `usage-allowance-checks/lib/codex.mjs` for the same input
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/codex/...`

## Task F16-T3: Port the Codex credential loader

**Depends on:** F16-T1, F16-T2
**Files:**
- create `internal/usage/provider/codex/codex_credential.go`
- create `internal/usage/provider/codex/codex_credential_test.go`

**Spec references:** `specs/features/F16-provider-codex/SPEC.md §2.1, §2.3, D1-D2, D10, D13`, `specs/features/F16-provider-codex/CONTRACTS.md §1, §7`, `docs/plan/research/usage-allowance-checks-spec.md` §2.2 (codex credential)

**Instructions:**
1. Write the test file first. Use `t.TempDir()` paths and `t.Setenv("CODEX_HOME", "")` where the home fallback matters (or pass explicit paths).
2. Implement `LoadCredential(authPath, configPath string) (Credential, error)`:
   - `security.ReadBoundedFile(authPath, security.MaxCredentialBytes)`; ENOENT → `Error{Code: "credential_file", Message: "Codex credentials were not found; sign in with Codex first."}`; oversized/unreadable → `credential_file` (`The credential file has an invalid size.` for size violations).
   - Decode object; failure → `credential_json` (`The credential file is not valid JSON.`). `tokens := value.tokens ?? value.auth ?? value` (JSON object values; non-object nested values count as absent → fall through like `.mjs` `??`).
   - `token = tokens.access_token ?? tokens.accessToken`; missing or failing `security.ValidateOpaqueToken` → `unsafe_credential` (`The Codex access token is missing or unsafe.`).
   - `accountID = tokens.account_id ?? tokens.accountId ?? value.account_id ?? value.chatgpt_account_id`; must be a string of length 1..512 with no whitespace/control (port of `assertIdentifier`, `core.mjs:24-31` — implement a private `validateIdentifier`); failure → `unsafe_credential` (`The ChatGPT account identifier is missing or unsafe.`).
   - `configuredBaseURL = value.base_url ?? value.baseUrl ?? value.openai_base_url`; when none: read `configPath` bounded; ENOENT/oversized/unreadable → silently ignore (SPEC D2/§2.1); else `ParseConfig(text)`.
   - `Credential{Token, AccountID, ConfiguredBaseURL}`.
3. Never include file contents in error messages.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `{"tokens":{"access_token":"canary-secret-token-123","account_id":"acct-synthetic"}}` | token/accountID set, no base URL |
| 2 | same with `accessToken` + `accountId` | same values |
| 3 | `{"auth":{"access_token":"canary-secret-token-123"},"account_id":"acct"}` | token from `auth`, accountID from root |
| 4 | flat `{"access_token":"canary-secret-token-123","chatgpt_account_id":"acct"}` | token + accountID |
| 5 | `base_url` on auth.json (with tokens) | `ConfiguredBaseURL == "https://trusted.example/v1"` |
| 6 | no base_url on auth.json; config.toml `model_provider = "trusted"\n[model_providers.trusted]\nbase_url = "https://trusted.example/v1"` | `ConfiguredBaseURL == "https://trusted.example/v1"` (via ParseConfig) |
| 7 | auth.json missing | `credential_file`, message `Codex credentials were not found; sign in with Codex first.` |
| 8 | auth.json `{bad` | `credential_json` |
| 9 | auth.json `{}` | `unsafe_credential` (`The Codex access token is missing or unsafe.`) |
| 10 | token ok, `account_id: "bad id"` (space) | `unsafe_credential` (`The ChatGPT account identifier is missing or unsafe.`) |
| 11 | no base_url; config.toml missing | `ConfiguredBaseURL == ""`, no error |
| 12 | no base_url; config.toml `{bad` | `ConfiguredBaseURL == ""`, no error (malformed config silently absent) |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] field-name and message parity: case 1-6 and 7-10 mirror `loadCodexCredential` from `codex.mjs` and its fixtures (`.mjs` test cases 5-7 use the same shapes)
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/codex/...`

## Task F16-T4: Port the Codex usage normalizer

**Depends on:** F16-T1
**Files:**
- create `internal/usage/provider/codex/codex_normalize.go`
- create `internal/usage/provider/codex/codex_normalize_test.go`

**Spec references:** `specs/features/F16-provider-codex/SPEC.md §2.10, §2.11, D8-D11`, `specs/features/F16-provider-codex/CONTRACTS.md §5, §6`, `docs/plan/research/usage-allowance-checks-spec.md` §2.2 (normalizeCodexUsage)

**Instructions:**
1. Write the test file first, table-driven.
2. Implement `NormalizeUsage(raw []byte) ([]usage.Window, error)`:
   - Decode object; failure → `response_json` (`The provider returned unsupported JSON.`).
   - `rateLimit := value.rate_limit ?? value.rateLimit ?? value` (object values; non-object → fall through).
   - Primary window: `rateLimit.primary_window ?? rateLimit.primaryWindow`; skip when absent/non-object; `usedPercent = finitePercent(used_percent ?? usedPercent)` (number or numeric string, 0..100 finite); skip when undefined. Window: ID `5h`, Label `primary window`, Unit percent, `UsedPercent`, `ResetsAt = resetTime(reset_at ?? resetAt)`, `WindowMinutes = limitWindowSeconds(reset... )` — from `limit_window_seconds ?? limitWindowSeconds` (finite positive integer → value/60, else nil), `UsageKnown: true`.
   - Secondary window: same with `secondary_window`/`secondaryWindow`, ID `weekly`, Label `secondary window`.
   - Credits: `value.credits` object; `balance = finiteNonNegative(credits.balance)`; when defined → window ID `credits`, Label `credits`, Unit credits, `Remaining = balance`, `UsageKnown: true`.
   - Additional (annex-a §3.1): `value.additional_rate_limits ?? value.additionalRateLimits` (array); per entry: `limitName = limit_name ?? limitName` (string; skip entry when empty), `meteredFeature = metered_feature ?? meteredFeature` (string; nil scope when absent), `rl = rate_limit ?? rateLimit` (object; skip when absent); chosen window object = `rl.primary_window ?? rl.primaryWindow` when present else `rl.secondary_window ?? rl.secondaryWindow`; skip when none; percent from the chosen window (`used_percent ?? usedPercent`); skip when invalid. Window: ID `"additional:"+slug(limitName)`, Label `limitName`, Unit percent, `ModelScope []string{meteredFeature}` when present, `UsedPercent`, `ResetsAt`, `WindowMinutes` as above, `UsageKnown: true`. `slug`: lowercase, runs of non-alphanumerics → `-`, trimmed.
   - Zero windows → `unsupported_response` (`Codex returned an unsupported usage shape.`).
3. `finitePercent`/`finiteNonNegative`/`resetTime`/`slug` are private helpers (ports of `core.mjs:202-224`); resetTime accepts epoch numbers (`> 10_000_000_000` is ms) and ISO strings.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `{"rate_limit":{"primary_window":{"used_percent":20,"reset_at":1900000000}}}` (copy from `usage-allowance.test.mjs` case 6) | 1 window: ID `5h`, Label `primary window`, UsedPercent 20, ResetsAt `2030-03-17T17:46:40Z`, WindowMinutes nil |
| 2 | `{"rateLimit":{"secondaryWindow":{"usedPercent":33,"resetAt":1900000000}}}` (camel) | 1 window: ID `weekly`, Label `secondary window`, UsedPercent 33 |
| 3 | `{"rate_limit":{"primary_window":{"used_percent":20},"secondary_window":{"used_percent":50}}}` | 2 windows: `5h` 20, `weekly` 50, in that order |
| 4 | `{"primary_window":{"used_percent":20}}` (top-level fallback) | 1 window: `5h` 20 |
| 5 | `{"rate_limit":{"primary_window":{"used_percent":20}},"credits":{"balance":12.5}}` | 2 windows: `5h` 20, `credits` Remaining 12.5 |
| 6 | `{"rate_limit":{"primary_window":{"used_percent":20},"limit_window_seconds":18000}}` | `5h` WindowMinutes 300 |
| 7 | `{"rate_limit":{"primary_window":{"used_percent":20}},"additional_rate_limits":[{"limit_name":"o1-mini-weekly","metered_feature":"o1-mini","rate_limit":{"primary_window":{"used_percent":55,"reset_at":1900000000},"limit_window_seconds":604800}}]}` | `5h` + `additional:o1-mini-weekly`: UsedPercent 55, ModelScope `["o1-mini"]`, WindowMinutes 10080, ResetsAt `2030-03-17T17:46:40Z` |
| 8 | `{"rate_limit":{"primary_window":{"used_percent":150}}}` | percent out of range → no windows → `unsupported_response` |
| 9 | `{"credits":{"balance":"not-a-number"}}` | no windows → `unsupported_response` |
| 10 | `{"rate_limit":{"primary_window":null}}` | no windows → `unsupported_response` |
| 11 | `not json` | `response_json` |
| 12 | `{}` | `unsupported_response` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] case 1 matches `normalizeCodexUsage` from `codex.mjs` for the same fixture (used 20, label `primary window`, remaining derived 80, reset `2030-03-17T17:46:40Z`)
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/codex/...`

## Task F16-T5: Port the Codex fetch with trusted-origin fallback

**Depends on:** F16-T3, F16-T4
**Files:**
- create `internal/usage/provider/codex/codex_fetch.go`
- create `internal/usage/provider/codex/codex_fetch_test.go`
- edit `internal/usage/provider/codex/codex.go` (replace placeholder `Fetch`)

**Spec references:** `specs/features/F16-provider-codex/SPEC.md §2.5-§2.9, D3-D6`, `specs/features/F16-provider-codex/CONTRACTS.md §3, §7`, `docs/plan/research/usage-allowance-checks-spec.md` §2.2 (`checkCodexUsage`), §3 (fallback trust)

**Instructions:**
1. Write the test file first, with a stub `http.RoundTripper` that records requests (URL + headers) and returns canned responses, exactly like F15-T4's stub. The codex trust helper is provider-local: implement `trustedFallbackURL(configuredBase, trustedOrigin string) (string, error)` as the port of `validateTrustedBaseUrl` (`core.mjs:108-132`):
   - Either URL unparseable → `untrusted_origin` (`The configured Codex fallback origin was not explicitly trusted.`).
   - `configuredBase`: must be `https:`, no userinfo, no query, no fragment. `trustedOrigin`: `https:`, no userinfo/query/fragment, pathname `/`. Origin equality required. Violations → `untrusted_origin` (same message).
   - Target: ensure the base pathname ends with `/`, then `origin + pathname + "api/codex/usage"`; target origin must equal base origin else `endpoint_refused` (`The configured Codex fallback endpoint was refused.`).
2. Implement the private `requestJSON` helper exactly as F15-T4 step 2 (same codes/messages, provider-agnostic).
3. Implement `Fetch(ctx, cred, client)` (replace placeholder):
   - `credential, err := LoadCredential(resolvedAuthPath, resolvedConfigPath)` where paths come from `CODEX_HOME` if set else home + `.codex/{auth.json,config.toml}`; loader error → `Snapshot{Provider:"codex", Failure: &usage.Failure{Code: e.Code, Message: e.Message}, ...}`.
   - Primary request: `GET UsageURL`, allow-list `[UsageURL]`, headers exactly CONTRACTS §3 (`Accept: application/json`, `Authorization: Bearer <token>`, `ChatGPT-Account-Id: <accountID>`).
   - 200 → `NormalizeUsage` → Snapshot (failure on normalize error). Status ∈ `FallbackStatuses`: `ConfiguredBaseURL == ""` → Failure `fallback_unavailable` (verbatim message); else `trustedOrigin := TrustedOriginFrom(ctx)`, `trustedFallbackURL(...)` → Failure on error; fallback request (same headers, allow-list `[target]`); non-200 → `mapStatus("Codex fallback", status)`; 200 → normalize → Snapshot.
   - Status ∉ fallback set and ≠ 200 → `mapStatus("Codex", status)` (401/403 → `unauthorized` `Codex rejected the credential.`; 429 → `rate_limited` `Codex rate-limited the usage request.`; else `provider_status` `Codex usage is unavailable (HTTP <status>).`).
   - Snapshot: `Provider:"codex"`, `Windows`, `UsageKnown: slices.ContainsFunc(windows, func(w usage.Window) bool { return w.UsageKnown && !w.Synthetic })`, `FetchedAt: time.Now().UTC()`, `Source: usage.SourceOAuth`, `Confidence:"live"`, `Account` unset.

On success set `Snapshot.UsageKnown = slices.ContainsFunc(windows, func(w usage.Window) bool { return w.UsageKnown && !w.Synthetic })` before returning. Synthetic-only and failed snapshots remain false. Pin real zero, credits-only, and failed outcomes at the Fetch boundary.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | loader from temp auth.json (`tokens.access_token` = canary, `account_id` = `acct-synthetic`); primary 200 with case-6 fixture | Snapshot: `5h` used 20, resets `2030-03-17T17:46:40Z`; request headers exactly 3: `Accept: application/json`, `Authorization: Bearer canary-secret-token-123`, `ChatGPT-Account-Id: acct-synthetic`; exactly 1 request |
| 2 | same, primary 404; no `configuredBaseUrl` | Failure `fallback_unavailable`, message verbatim; exactly 1 request |
| 3 | same, primary 404; `base_url: "https://trusted.example/v1"`; NO `WithTrustedOrigin` | Failure `untrusted_origin`; exactly 1 request |
| 4 | same, primary 404; `WithTrustedOrigin(ctx, "https://trusted.example")`; fallback 200 with case-6 fixture | requests: `[UsageURL, "https://trusted.example/v1/api/codex/usage"]`; both with 3 headers incl. canary Bearer + `acct-synthetic`; Snapshot `5h` used 20; `Account` unset |
| 5 | primary 401, configured base + trusted origin set | Failure `unauthorized`; exactly 1 request (never falls back) |
| 6 | primary 429 | Failure `rate_limited`; exactly 1 request |
| 7 | primary 404; `base_url: "http://unsafe.example"` (http); `WithTrustedOrigin(ctx, "https://unsafe.example")` | Failure `untrusted_origin` (origin mismatch); 1 request |
| 8 | primary 404; fallback 500 | Failure `provider_status`, message `Codex fallback usage is unavailable (HTTP 500).`; 2 requests |
| 9 | primary 200 body `{bad` | Failure `response_json` |
| 10 | auth.json missing | Failure `credential_file` verbatim; 0 requests |
| 11 | loader ok; primary 302 | Failure `redirect_refused` |
| 12 | primary 200 `{}` | Failure `unsupported_response` |

**Acceptance criteria:**
- [ ] `go build ./internal/usage/provider/codex/...` succeeds
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] fallback URL for case 4 is exactly `https://trusted.example/v1/api/codex/usage` (`.mjs` test 6 asserts the same URL)
- [ ] no file outside the Files list modified (the `codex.go` edit is the declared placeholder replacement)

`go test ./internal/usage/provider/codex/...`

## Task F16-T6: Add the status-to-Failure mapping table tests

**Depends on:** F16-T5
**Files:**
- create `internal/usage/provider/codex/codex_status_test.go`

**Spec references:** `specs/features/F16-provider-codex/CONTRACTS.md §7`, `docs/plan/annex-a-provider-matrix.md` §7, `docs/plan/research/usage-allowance-checks-spec.md` §3

**Instructions:**
1. Table-driven test over `mapStatus` (private, same package) for both provider names `"Codex"` and `"Codex fallback"`, plus the fallback-precondition errors.
2. Cover: 401/403 → `unauthorized`; 429 → `rate_limited`; samples (400, 404, 500, 502, 503) → `provider_status` with `Codex usage is unavailable (HTTP <status>).` / `Codex fallback usage is unavailable (HTTP <status>).`; 3xx handled at the request layer (stub 301/302/307/308 → `redirect_refused`).
3. Add the `trustedFallbackURL` precondition table: valid origin+path; origin mismatch; userinfo; query; fragment; trusted pathname not `/`; unparseable; http scheme; target escaping the origin (base pathname `../../x`).
4. Assert every emitted code is in the global §1.6 set (hard-coded list in the test).

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `mapStatus("Codex", 401)` / 403 | `unauthorized`, `Codex rejected the credential.` |
| 2 | `mapStatus("Codex", 429)` | `rate_limited`, `Codex rate-limited the usage request.` |
| 3 | `mapStatus("Codex", 500)` | `provider_status`, `Codex usage is unavailable (HTTP 500).` |
| 4 | `mapStatus("Codex fallback", 500)` | `provider_status`, `Codex fallback usage is unavailable (HTTP 500).` |
| 5 | `trustedFallbackURL("https://trusted.example/v1", "https://trusted.example")` | ok, `https://trusted.example/v1/api/codex/usage` |
| 6 | `trustedFallbackURL("https://other.example/v1", "https://trusted.example")` | `untrusted_origin` |
| 7 | `trustedFallbackURL("https://trusted.example/v1", "https://trusted.example/")` | ok (pathname `/` accepted) |
| 8 | `trustedFallbackURL("https://user@trusted.example/v1", "https://trusted.example")` | `untrusted_origin` (userinfo) |
| 9 | `trustedFallbackURL("https://trusted.example/v1?q=1", "https://trusted.example")` | `untrusted_origin` (query) |
| 10 | `trustedFallbackURL("http://trusted.example/v1", "https://trusted.example")` | `untrusted_origin` |
| 11 | `trustedFallbackURL("https://trusted.example/../escape", "https://trusted.example")` | `endpoint_refused` (target escapes origin) |
| 12 | 3xx via stub transport | `redirect_refused` |
| 13 | every emitted code | member of the global §1.6 set |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/codex/...`

## Task F16-T7: Canary-test every credential- and origin-touching path

**Depends on:** F16-T5
**Files:**
- create `internal/usage/provider/codex/codex_canary_test.go`

**Spec references:** `specs/features/F16-provider-codex/SPEC.md §3`, `specs/global/SPEC.md §6 item 5`, `docs/plan/research/usage-allowance-checks-spec.md` §9

**Instructions:**
1. Use `security.WithCanary(canary, fn)` around each scenario (fails the test if the canary appears in error text).
2. Canary values: token `"canary-secret-token-123"`, account ID `"acct-synthetic"`, response marker `"canary-body-marker-42"`, origin marker `"canary.example"`.
3. Scenarios: (a) canary token/account ID with 401 response; (b) canary token with network error; (c) canary marker inside a 200 body (absent from windows and errors); (d) canary account ID with successful fetch — the ID appears in request headers only, never in the Snapshot or any error; (e) `untrusted_origin` error when the configured base contains `canary.example`; (f) `fallback_unavailable` path.
4. Mirror the `.mjs` assertion: output must not match `/canary|acct-synthetic/` (test case 6).

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | canary token + `acct-synthetic`, 401 body with canary | error free of canary and account ID |
| 2 | canary token, transport error `errors.New("boom canary-secret-token-123")` | `network`, no canary |
| 3 | 200 body with `canary-body-marker-42` | windows/errors contain no marker |
| 4 | successful fetch with account ID `acct-synthetic` | Snapshot has no account ID anywhere (Account unset, no window carries it) |
| 5 | configured base `https://canary.example/v1`, no trusted origin | `untrusted_origin`, verbatim message (never echoes the origin) |
| 6 | full success path (fixture case 6) | canary free (`.mjs` test 6 parity) |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/codex/...`

## Task F16-T8: Golden fixtures and Node-script output parity

**Depends on:** F16-T4, F16-T5
**Files:**
- create `internal/usage/provider/codex/testdata/usage/codex/codex_basic.json`
- create `internal/usage/provider/codex/testdata/usage/codex/codex_camel.json`
- create `internal/usage/provider/codex/testdata/usage/codex/codex_credits.json`
- create `internal/usage/provider/codex/testdata/usage/codex/codex_additional.json`
- create `internal/usage/provider/codex/testdata/usage/codex/codex_top_level.json`
- create `internal/usage/provider/codex/testdata/usage/codex/codex_unsupported.json`
- create `internal/usage/provider/codex/codex_fixture_test.go`

**Spec references:** `specs/features/F16-provider-codex/SPEC.md §2.10-§2.12`, `docs/plan/annex-a-provider-matrix.md` §8 (golden-file policy)

**Instructions:**
1. Create the fixtures with EXACTLY this content:
   - `codex_basic.json`: `{"rate_limit": {"primary_window": {"used_percent": 20, "reset_at": 1900000000}}}` — copy from `usage-allowance-checks/tests/usage-allowance.test.mjs` case 6.
   - `codex_camel.json`: `{"rateLimit": {"secondaryWindow": {"usedPercent": 33, "resetAt": 1900000000}}}` — constructed (camel variant).
   - `codex_credits.json`: `{"rate_limit": {"primary_window": {"used_percent": 20}}, "credits": {"balance": 12.5}}` — constructed from `codex.mjs` field names.
   - `codex_additional.json`: `{"rate_limit": {"primary_window": {"used_percent": 20}}, "additional_rate_limits": [{"limit_name": "o1-mini-weekly", "metered_feature": "o1-mini", "rate_limit": {"primary_window": {"used_percent": 55, "reset_at": 1900000000}, "limit_window_seconds": 604800}}]}` — constructed from annex-a §3.1.
   - `codex_top_level.json`: `{"primary_window": {"used_percent": 20}}` — constructed (top-level fallback per `codex.mjs:64`).
   - `codex_unsupported.json`: `{"credits": {"balance": "not-a-number"}}` — constructed (fail-closed shape).
2. Table-driven test: read + `NormalizeUsage` per fixture; expected windows:
   - `codex_basic` → `[{5h, primary window, percent, used 20, resets 2030-03-17T17:46:40Z, known}]`
   - `codex_camel` → `[{weekly, secondary window, percent, used 33, resets 2030-03-17T17:46:40Z, known}]`
   - `codex_credits` → `[{5h used 20}, {credits, credits, credits-unit, remaining 12.5, known}]`
   - `codex_additional` → `[{5h used 20}, {additional:o1-mini-weekly, o1-mini-weekly, percent, used 55, minutes 10080, scope [o1-mini], resets 2030-03-17T17:46:40Z, known}]`
   - `codex_top_level` → `[{5h used 20}]`
   - `codex_unsupported` → `unsupported_response`
3. Parity comment in the test: for `codex_basic.json`, `normalizeCodexUsage` in `codex.mjs` yields `{label:"primary window", usedPercent:20, remainingPercent:80, resetAt:"2030-03-17T17:46:40.000Z"}`; the Go window carries the same values (used 20; the F24 renderer derives `80% available`), so rendered text matches the Node script's `- primary window: 20% used; 80% available; resets 2030-03-17T17:46:40Z`.

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | `codex_basic.json` | windows as in step 2 |
| 2 | `codex_camel.json` | `weekly` used 33 |
| 3 | `codex_credits.json` | `5h` + `credits` remaining 12.5 |
| 4 | `codex_additional.json` | `5h` + additional window |
| 5 | `codex_top_level.json` | `5h` used 20 |
| 6 | `codex_unsupported.json` | `unsupported_response` |

**Acceptance criteria:**
- [ ] `go test ./internal/usage/provider/codex/...` passes with the test cases above
- [ ] output matches `usage-allowance-checks` Node script for the same recorded fixture: `codex_basic.json` (`.mjs` case 6) normalized values identical to `normalizeCodexUsage` (used 20, label `primary window`, reset `2030-03-17T17:46:40Z`), and the fallback URL chain exercised in F16-T5 case 4 equals the `.mjs` case-6 calls
- [ ] fixture field names verbatim from the `.mjs`/annex-a — no invented field names
- [ ] no file outside the Files list modified

`go test ./internal/usage/provider/codex/...`
