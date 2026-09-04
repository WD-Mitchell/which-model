---
kind: feature-spec
feature: F16-provider-codex
version: "1.0"
project: which-model
---

# F16 — provider-codex: Codex / ChatGPT subscription-usage adapter

## 1. Purpose

`internal/usage/provider/codex` is the Tier-1 adapter that reports Codex usage allowance (primary/secondary rate-limit windows plus the credits balance) from `GET https://chatgpt.com/backend-api/wham/usage`, with the prototype's fallback to a configured, explicitly trusted origin. It is the Go port of `usage-allowance-checks/lib/codex.mjs` (120 lines), normalized into the canonical `usage.Window` contract, keeping its values diffable against the Node prototype's recorded fixtures. It self-registers a `usage.Descriptor` with ID `codex`.

## 2. Behaviour

1. **The credential loader is ported verbatim** (`loadCodexCredential`, `codex.mjs:52-61`): reads `auth.json` (bounded, 1 MiB), token shape `value.tokens ?? value.auth ?? value`, token `access_token ?? accessToken` (opaque check), account ID `tokens.account_id ?? tokens.accountId ?? value.account_id ?? value.chatgpt_account_id` (identifier check, port of `assertIdentifier`: length 1..512, no whitespace/control), configured base URL `value.base_url ?? value.baseUrl ?? value.openai_base_url ?? optionalConfiguredBaseUrl(config.toml)`. The config.toml leg (`optionalConfiguredBaseUrl`, `codex.mjs:37-41`) is **silently absent on any `credential_file` outcome** (missing, unreadable, or oversized config) and rethrows nothing else. Missing `auth.json` → `credential_file` with the verbatim message `Codex credentials were not found; sign in with Codex first.` (`codex.mjs:54`).

2. **`parseCodexConfig` is ported verbatim** (`codex.mjs:8-35`): line-scanned; `[model_providers.<id>]` sections record `base_url` per provider; root `model_provider` records the active provider; root `base_url` is the root fallback; quoted values (single or double), `#` comments, and blank lines handled; other sections ignored. Result: `(activeProvider && providerUrls.get(activeProvider)) || rootBaseUrl`.

3. **Config paths.** `LoadCredential` defaults to `$CODEX_HOME/{auth.json,config.toml}` when `CODEX_HOME` is set, else `~/.codex/{auth.json,config.toml}` (the Codex CLI's own env override; the `.mjs` hardcodes the home path). The declarative Auth chain mirrors the same paths so chain resolution and the loader never disagree.

4. **The Auth chain is a declarative gating twin.** Descriptor `Auth` = ordered `AuthFile` entries over `[$CODEX_HOME/auth.json, ~/.codex/auth.json]`, one entry per tolerated token shape: JSONPaths `tokens.access_token`, `tokens.accessToken`, `auth.access_token`, `auth.accessToken`, `access_token`, `accessToken` (chain rule per F12: missing JSONPath → no candidate → next entry). The chain exists for F12/F14 gating and `auth` display; the Fetch's operational credential always comes from the verbatim loader (D1: `account_id`/`base_url` resolution spans `ExtraPaths`-inexpressible sources — config.toml parsing, root-vs-provider priority — so the loader stays provider-local; the chain never carries `ExtraPaths` for this provider).

5. **Fetch consumes the loader.** `Fetch(ctx, cred, client)`: load the credential; loader errors map 1:1 to `Snapshot.Failure` (`credential_file`, `credential_json`, `unsafe_credential` with the verbatim messages from `codex.mjs`). `cred` is advisory: when the chain resolved a token but the loader failed with `credential_file`, the Fetch fails with `credential_file` (the loader is authoritative — parity with `checkCodexUsage`, `codex.mjs:102-104`).

6. **Request discipline** mirrors F15 (identical helper contract): exact-URL allow-list (`endpoint_refused`), redirect hard-fail (`redirect_refused`), bounded body via F05 `security.ReadResponseBounded`/`MaxResponseBytes` (`response_too_large`), empty/non-object JSON (`response_json`), deadline (`timeout`), transport (`network`). Headers exactly three: `Accept: application/json`, `Authorization: Bearer <token>`, `ChatGPT-Account-Id: <account_id>` — the `.mjs` sends no `User-Agent` (`fetchCodex`, `codex.mjs:83-92`; annex-a §3.1).

7. **Primary call** `GET https://chatgpt.com/backend-api/wham/usage` (allow-list `[CODEX_USAGE_URL]`). Status 200 → normalize. Status ∉ {404, 405, 410, 501} → `statusError("Codex", status)` (`codex.mjs:104-106`). Status ∈ {404, 405, 410, 501}: no configured base URL → `fallback_unavailable` with verbatim message `Codex did not advertise a configured fallback endpoint.` (`codex.mjs:107-110`).

8. **Fallback trust semantics are verbatim** (`validateTrustedBaseUrl`, `core.mjs:108-132`): both the configured base URL and `trustedOrigin` must parse as URLs, be `https:`, have no userinfo/search/hash; `trustedOrigin` pathname must be `/`; origins must be equal; the target is `new URL('api/codex/usage', <base-origin><base-pathname-with-trailing-slash>)` and must stay within the same origin (else `endpoint_refused` with `The configured Codex fallback endpoint was refused.`). Any parse/shape failure → `untrusted_origin` with verbatim message `The configured Codex fallback origin was not explicitly trusted.`. The fallback request allow-list is exactly `[target]`; a non-200 fallback → `statusError("Codex fallback", status)`; 200 → normalize. The trusted origin arrives via a context value `WithTrustedOrigin(ctx, origin)` (F25's `--trust-configured-origin` opt-in per invocation, annex-d); an empty/missing context value is an untrusted origin — never a default.

9. **401/403/429 never fall back** — they are terminal `unauthorized`/`rate_limited` after exactly one HTTP call (`.mjs` test: `calls == 1` for 401 and 429 even with a configured base URL).

10. **Normalization** (`normalizeCodexUsage`, `codex.mjs:63-81`, plus annex-a §3.1): rate-limit object = `value.rate_limit ?? value.rateLimit ?? value`; windows in fixed order: `primary_window`/`primaryWindow` → ID `5h`, Label `primary window`; `secondary_window`/`secondaryWindow` → ID `weekly`, Label `secondary window`; per window: `UsedPercent = finitePercent(used_percent ?? usedPercent)`, skip when absent; `ResetsAt = resetTime(reset_at ?? resetAt)` (epoch seconds, `> 10_000_000_000` is ms); `WindowMinutes = limit_window_seconds/60` when a finite positive integer (annex-a §3.1), else nil; `UsageKnown: true`, `Unit: percent`. Then `credits.balance` (finite non-negative) → ID `credits`, Label `credits`, `Unit: credits`, `Remaining = balance`, `UsageKnown: true`. Zero windows → `unsupported_response` with verbatim message `Codex returned an unsupported usage shape.`.

11. **`additional_rate_limits` (annex-a §3.1; prototype does not consume them).** `value.additional_rate_limits ?? value.additionalRateLimits` array; per entry (field names `limit_name`/`limitName`, `metered_feature`/`meteredFeature`, `rate_limit`/`rateLimit`): window ID `additional:<slug(limitName)>`, Label `limitName`, `Unit: percent`, `ModelScope [meteredFeature]` when present, percent/reset/window-minutes from `rateLimit.primary_window` when present else `rateLimit.secondary_window` (chosen window's `used_percent`/`reset_at`/`limit_window_seconds`); skip the entry when no window yields a valid percent. `slug` = lowercase, runs of non-alphanumerics → `-`, trimmed.

12. **Snapshot assembly.** `Snapshot{Provider:"codex", Windows, UsageKnown: any window is known and non-synthetic, FetchedAt: now UTC, Source: SourceOAuth, Confidence:"live"}`. `Account` is NEVER set — the account ID is a request header only and must not leak into output (`.mjs` test asserts `/canary|acct-synthetic/` absent; annex-a §3.1). `Plan` empty.

13. **Descriptor constants.** `Timeout: 15s`, `CacheTTL: 60s` (annex-a §5 literal / §6 pattern), `Kind: KindSubscription`, `Tier: 1`, `DisplayName: "Codex"` (the `.mjs` report header, `codex.mjs:110,116`; supersedes the survey's "Codex / ChatGPT" phrasing). `Windows` descriptor list: `5h` (percent), `weekly` (percent), `credits` (credits, Optional) — all three Optional (emitted only when the response carries them). `init()` registers; duplicate ID panics.

## 3. Error behaviour

All failures return `(zero Snapshot, *Error)` with `Error{Code, Message}` carrying a global `Failure.Code` (§1.6) and a sanitized fixed message. Codes this feature emits: `credential_file`, `credential_json`, `unsafe_credential`, `endpoint_refused`, `untrusted_origin`, `redirect_refused`, `response_too_large`, `response_json`, `timeout`, `network`, `unauthorized`, `rate_limited`, `provider_status`, `fallback_unavailable`, `unsupported_response`. No message or log line ever contains credential material, account IDs, or response bodies (global SPEC §6 item 5; canary-tested).

## 4. Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Operational credential resolution stays provider-local (`LoadCredential`), not declarative `ExtraPaths` | chain = gating twin only | `account_id`/`base_url` resolution spans config.toml parsing and root-vs-provider priority — not expressible as `ExtraPaths` dotted JSON paths (F12 CONTRACTS); the loader is the `.mjs` port and the parity oracle |
| D2 | `CODEX_HOME` overrides `~/.codex` for both loader and chain | `$CODEX_HOME/{auth.json,config.toml}` first | real Codex CLI precedence; keeps chain and loader consistent |
| D3 | No `User-Agent` header on codex requests | exactly 3 headers | `fetchCodex` sends none (`codex.mjs:83-92`); annex-a §3.1 |
| D4 | Fallback statuses exactly {404, 405, 410, 501}; 401/403/429 terminal after 1 call | — | `FALLBACK_STATUSES` (`codex.mjs:6`); `.mjs` test 7 asserts call counts |
| D5 | `trustedOrigin` comes from `WithTrustedOrigin(ctx, origin)`; absent → `untrusted_origin` | never a default | F25's `--trust-configured-origin` is per-invocation opt-in (annex-d); security invariant: no implicit trust |
| D6 | Fallback target = configured base + `api/codex/usage` under the same origin | e.g. `https://trusted.example/v1` → `https://trusted.example/v1/api/codex/usage` | `validateTrustedBaseUrl` (`core.mjs:108-132`); `.mjs` test 6 asserts the exact URL |
| D7 | `Snapshot.Account` never set for codex | account ID is request-header-only | `.mjs` test 6 asserts no `acct-synthetic` leak; annex-a §3.1 |
| D8 | `WindowMinutes = limit_window_seconds/60` when present | else nil | annex-a §3.1 mapping |
| D9 | `additional_rate_limits` windows: percent from primary else secondary rate-limit object | one window per entry, `additional:<slug(limitName)>` | annex-a §3.1; prototype has no additional-window tests, so parity fixtures omit them |
| D10 | Identifier shape ported as `assertIdentifier` (1..512, no whitespace/control) | `unsafe_credential` on failure | `core.mjs:24-31`; F05 pins only opaque tokens, so the account-ID check is provider-local |
| D11 | Window IDs `5h`/`weekly`/`credits`; labels verbatim `primary window`/`secondary window`/`credits` | — | annex-a §3.1 IDs; `.mjs` labels for output parity |
| D12 | `DisplayName: "Codex"`, `Timeout: 15s`, `CacheTTL: 60s` | — | `.mjs` report header; annex-a §5 literal / §6 default |
| D13 | Missing `auth.json` → `credential_file` (exit class 5), not `login_required` | message verbatim `Codex credentials were not found; sign in with Codex first.` | `.mjs` emits `credential_file`; global CONTRACTS §1.6 defines `login_required` as "no credential found, no `--login`" — codex has no `--login` |

## 5. Out of scope

- Codex RPC transport (`/backend-api/codex/check_can_use_model`, `gpt-oss` quota probes) — the prototype has none; annex-a's RPC path is deferred (recorded upstream).
- `--login` device flow for codex — no prototype support; absent credential is a hard `credential_file`.
- Keychain storage of codex credentials — the `.mjs` chain is file-only.
- Build-tag `nousage` stubbing — F21's domain (global SPEC §4); DEPENDENCY-GRAPH §2 lists no `blocks` for F16.
- Text rendering (`formatUsageReport` port) — F24's domain; this feature guarantees data-level parity.

## Snapshot knowledge correction (#182)

Successful fetches set the existing `Snapshot.UsageKnown` field to whether any normalized window has `UsageKnown && !Synthetic` (global CONTRACTS §1.5). A real zero reading, credits-only reading, or unlimited known window counts; synthetic-only and failed snapshots remain false. This corrects an omitted aggregate assignment without changing canonical types or the JSON schema. The aggregate flag survives F14 live fetch, cache serialization/replay, and JSON output.

| Snapshot contents | `usage_known` |
|---|---|
| Real positive or zero reading | `true` |
| Real credits-only or unlimited known reading, when supported | `true` |
| Mixed real and synthetic windows, when supported | `true` |
| Synthetic-only windows, when supported | `false` |
| Provider failure | `false` |
