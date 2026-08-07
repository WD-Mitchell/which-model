---
kind: feature-spec
feature: F04-http
version: "1.0"
project: which-model
---

# F04 — http: SPEC

## Purpose

`internal/httpkit` is the shared HTTP client for every outbound request in the binary: provider usage fetches (F14–F17) and catalog collectors (F08, which imports it directly — no transport wrapper, per the pinned decision). It enforces the transport-level security invariants — exact-URL allow-lists, redirect hard-fail, double-checked response bounding, a fixed `User-Agent` — plus the retry/timeout policy, so no caller can bypass them. It is Layer 0 and imports only the Go standard library.

Source: `docs/plan/annex-a-provider-matrix.md` §6 (fetch orchestration, per-request timeouts); `docs/plan/research/usage-allowance-checks-spec.md` §1 (`requestJson`, `readResponseText`, `statusError`); `docs/plan/annex-b-catalog-port.md` §7 (redirect rejection pattern, static User-Agent); `specs/global/CONTRACTS.md` §1.6 (Failure codes), §7 (constants); `specs/global/SPEC.md` §6 (security invariants 1–5).

## Behaviour

1. **Client construction.** `NewClient(opts ...Option)` builds a `Client` with defaults: `DefaultTimeout` (10s), `DefaultMaxResponseBytes` (262 144), retries 1, 500ms backoff, `User-Agent` `"which-model/dev"`. Options override; the zero-value `Client` is never used directly.
2. **Per-request timeout.** Each `Do` runs under the caller's context; if the caller's context has no deadline, the client's timeout is applied via `context.WithTimeout` (a caller-provided earlier deadline wins). Deadline expiry maps to `Error{Code:"timeout"}` (annex-a §6: each provider fetch runs under its own bounded context; the global `--timeout` flag is wired by F22 and flows down as a context deadline).
3. **Exact-URL allow-list (enforced when set).** `SetAllowList(urls)` stores the list; when non-empty, every request URL must parse, be `https`, carry no userinfo and no fragment, and its **Go-serialized** form (`(*url.URL).String()`) must be an exact member of the list — no prefix, origin, or substring matching (invariant #1). Allow-list entries must be written in Go-normalized form (e.g. `https://api.anthropic.com`; no WHATWG-style trailing slash). A request failing the check returns `Error{Code:"endpoint_refused"}` before any network I/O. When the allow-list is empty (never set), no URL check is applied. The predicate and messages mirror `security.ValidateExactHTTPS` (`specs/features/F05-security/CONTRACTS.md` §1) exactly; httpkit implements it natively and does NOT import `internal/security`, because F04 has no `depends_on` row (`specs/DEPENDENCY-GRAPH.md` §2) and must build before F05 exists.
4. **Redirect hard-fail.** `http.Client.CheckRedirect` returns a sentinel error so no redirect is ever followed (zero followed hops); additionally, any response with status in `[300, 400)` — including a 3xx without a `Location` header, which never triggers `CheckRedirect` — becomes `Error{Code:"redirect_refused"}` (invariant #2).
5. **Response bounding, checked twice.** A present `Content-Length` header greater than the client's maxBytes fails with `response_too_large` before reading; the body is then read through `io.LimitReader(body, maxBytes+1)` and more than maxBytes bytes fail with `response_too_large` (invariant #3, defends against a lying or absent header). Bodies are always closed.
6. **Retry policy.** Retries (default 1 → 2 attempts total, fixed 500ms backoff between attempts, honoring context cancellation) apply ONLY to 5xx statuses and transport `network` errors. Never retried: 4xx statuses, 3xx/`redirect_refused`, `response_too_large`, `timeout`, allow-list refusal. A request is only replayed when it is replayable (`Body == nil` or `GetBody != nil`); otherwise a 5xx is returned without retry. Catalog collectors needing annex-b §7's wider budget use `WithRetries(2)`.
7. **Timeout vs cancellation.** `context.DeadlineExceeded` → `timeout`; `context.Canceled` → `network`.
8. **Status handling.** `Do` returns the bounded body with a nil error ONLY for 2xx statuses. Every status >= 400 returns `*Error` with `StatusCode` set and `Code` mapped: 401/403 → `unauthorized`, 429 → `rate_limited`, any other >= 400 → `provider_status` (retryable 5xx are retried first per `WithRetries`; the mapping applies to the final attempt). The bounded body of a non-2xx response is discarded, never returned. Callers needing finer status discrimination (e.g. F08's 403-only free-endpoint fallback) branch on `Error.StatusCode` via `errors.As` — never on message text. F15/F16/F17 do NOT consume httpkit (their port keeps core.mjs's `requestJson` shape over a raw `*http.Client` per F11's `FetchFunc`); this mapping serves F08/F14/F23.
9. **GetJSON.** `GetJSON(ctx, url, headers, out)` is GET + `Do` + `json.Unmarshal` into `out`; any unmarshal failure (including empty body) maps to `Error{Code:"response_json"}`.
10. **Errors.** `httpkit.Error{Code, StatusCode, Err}` uses the stable code strings from `specs/global/CONTRACTS.md` §1.6 (`endpoint_refused`, `redirect_refused`, `response_too_large`, `timeout`, `network`, `response_json`, `unauthorized`, `rate_limited`, `provider_status`); `StatusCode` is the HTTP status (0 for non-HTTP failures). `Error()` renders a fixed, sanitized message per code (the Node prototype's strings, `usage-allowance-checks-spec.md` §1) — the underlying `Err` text is never included, so credential material cannot leak (invariant #5). `AsError` extracts an `*Error` from any error chain. Because `internal/httpkit` must not import `internal/usage` (boundary §8), callers convert `*httpkit.Error` into `usage.Failure{Code: e.Code, ...}` at their own package boundary (F08, F14).

## Error behaviour

- `Do` returns `*Error` for: allow-list refusal (`endpoint_refused`), any 3xx (`redirect_refused`), oversize body (`response_too_large`), deadline (`timeout`), transport failure or caller cancellation (`network`).
- `GetJSON` additionally returns `response_json` for any JSON unmarshal failure.
- An unparseable `Content-Length` header value is treated as absent; the bounded-reader check still applies.
- F04 adds **no new `Failure.Code` values**, owns **no flags** and **no config keys** (the global `--timeout` flag is wired by F22 and passed down by F08/F14 as a context deadline).

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Defaults | timeout 10s, maxBytes 262 144, retries 1, backoff 500ms, UA `"which-model/dev"` | constants are canonical (`specs/global/CONTRACTS.md` §7); pinned surface |
| D2 | Timeout application | client wraps the context only when the caller's context has no deadline; earlier caller deadline wins | annex-a §6 per-provider bounded contexts; global `--timeout` must remain authoritative |
| D3 | Allow-list enforced only when set | empty (never-set) allow-list disables the check | pinned contract ("if set"); callers that need invariant #1 set it explicitly |
| D4 | Allow-list comparison on Go URL serialization | `(*url.URL).String()`, exact membership | `net/url` is the canonical Go form; entries documented Go-normalized; same rule as F05 |
| D5 | Allow-list check implemented natively in httpkit | no `internal/security` import | F04's `depends_on` is empty (`specs/DEPENDENCY-GRAPH.md` §2); semantics and messages identical to F05's `ValidateExactHTTPS` |
| D6 | 3xx without `Location` still fails | explicit `300 <= status < 400` check after `Do` | `CheckRedirect` only fires when a redirect target exists; the Node original rejects ALL 3xx by status (`requestJson` step 5) |
| D7 | Retry set | only 5xx + `network`; 1 retry / 500ms; never 4xx, 3xx, too-large, timeout, allow-list refusal; only replayable requests | pinned contract; 4xx/3xx failures are deterministic — retrying them is wasted load; annex-b §7's wider budget is an explicit `WithRetries(2)` override |
| D8 | Retry wait honors context | backoff sleep selects on `ctx.Done()` | a canceled/expired context must abort the retry immediately |
| D9 | `context.Canceled` → `network`, `DeadlineExceeded` → `timeout` | distinct codes | Node's abort maps to `timeout`; caller-initiated cancellation is a transport-level failure |
| D10 | Status discrimination IN httpkit for >= 400 | `Do` returns body only for 2xx; 401/403 → `unauthorized`, 429 → `rate_limited`, other >= 400 → `provider_status`, always with `StatusCode` set | F08/F23 need typed 403 detection for the AA free-endpoint fallback (`errors.As` + `StatusCode == 403`); `Do`'s `([]byte, error)` signature cannot surface a status alongside a nil error, so pass-through-on-4xx was unimplementable. Supersedes the earlier "caller discriminates" pin. Providers F15/F16/F17 keep their own `requestJSON` over `*http.Client` (F11 `FetchFunc` seam), so this mapping does not reach them |
| D11 | Fixed UA wins over request headers | client sets `User-Agent` on every request, overriding any caller value | annex-b §7: explicit static UA, never Go's default; per-provider spoofing UAs configured via `WithUserAgent` (annex-a §6) |
| D12 | Backoff is a fixed constant | 500ms, not configurable | pinned surface (`WithRetries` has no backoff parameter); simplicity beats knobs |

## Milestone / dependencies

Milestone M1. `depends_on`: — (none, Wave W1, `specs/DEPENDENCY-GRAPH.md` §2–§3). `blocks`: F08 (collectors), F14 (usage-fetch).

## Out of scope

- JSON parsing beyond `GetJSON`'s unmarshal, `unsupported_response` shape validation → F14-usage-fetch.
- Per-provider status-message wording (the `<Provider> rejected the credential.` family) → F15/F16/F17 own `mapStatus` (their transport is raw `*http.Client`, not httpkit).
- Concurrency fan-out (`errgroup`, `--max-parallel`, partial-failure batches) → F14.
- `Retry-After` cache-TTL coupling and per-provider caching → F13/F14.
- Conversion of `*httpkit.Error` into `usage.Failure` → F08/F14 (import boundary §8).
- The standalone URL/trusted-origin validators and token/file security helpers → F05-security.
- Versioned User-Agent (`which-model/<version>`): the pinned default is the literal `"which-model/dev"`; publishing a real version string into the UA is a F30-publishing concern via `WithUserAgent`.
