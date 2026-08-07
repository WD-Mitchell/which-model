---
kind: feature-contracts
feature: F04-http
version: "1.0"
project: which-model
---

# F04 — http: CONTRACTS

Package `internal/httpkit`, files under `internal/httpkit/`. Imports: Go stdlib only (`net/http`, `net/url`, `context`, `io`, `time`, `encoding/json`, `fmt`, `errors`) — Layer 0 (`specs/global/SPEC.md` §3). The exported surface below is **pinned**; no additional exported symbols may be added without a plan-level decision. A `Client` is safe for concurrent use once configured (all state is read-only after construction/`SetAllowList`).

## 1. Exported surface (pinned)

File: `internal/httpkit/client.go`

```go
package httpkit

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

const (
    // DefaultTimeout is the per-request timeout applied when the caller's
    // context has no deadline (specs/global/CONTRACTS.md §7 DefaultTimeoutSec).
    DefaultTimeout = 10 * time.Second
    // DefaultMaxResponseBytes bounds every response body (specs/global/CONTRACTS.md §7 MaxResponseBytes).
    DefaultMaxResponseBytes = int64(262_144) // 256 KiB
)

type Option func(*Client)

// WithTimeout overrides the per-request timeout (default DefaultTimeout).
func WithTimeout(d time.Duration) Option
// WithMaxBytes overrides the response body bound (default DefaultMaxResponseBytes).
func WithMaxBytes(n int64) Option
// WithUserAgent overrides the User-Agent sent on every request
// (default "which-model/dev").
func WithUserAgent(s string) Option
// WithRetries sets the retry budget for 5xx statuses and network errors
// (default 1 retry = 2 attempts, 500ms backoff; 4xx is never retried).
func WithRetries(n int) Option

// NewClient builds a configured client with the documented defaults.
func NewClient(opts ...Option) *Client

// SetAllowList stores the exact-URL allow-list. When non-empty, every
// request URL must be https, carry no userinfo/fragment, and match an entry
// exactly under Go URL serialization; a mismatch returns
// Error{Code:"endpoint_refused"} before any network I/O. When empty
// (never set), no URL check is applied.
func (c *Client) SetAllowList(urls []string)

// Do executes req, enforcing in order: exact-URL allow-list (if set),
// redirect rejection (ANY 3xx -> Error{Code:"redirect_refused"}, zero
// followed hops), Content-Length pre-check + io.LimitReader body bound
// (checked twice -> Error{Code:"response_too_large"}), context deadline
// -> Error{Code:"timeout"}, transport failure -> Error{Code:"network"}.
// Success returns the bounded body ONLY for 2xx statuses. Every status
// >= 400 returns *Error with StatusCode set, Code mapped: 401/403 ->
// "unauthorized", 429 -> "rate_limited", any other >= 400 ->
// "provider_status" (retryable 5xx are retried first per WithRetries;
// the mapping applies to the final attempt's status). The client sets
// req's User-Agent on every call, overriding any caller value.
func (c *Client) Do(ctx context.Context, req *http.Request) ([]byte, error)

// GetJSON is GET + Do + json.Unmarshal into out. Any unmarshal failure
// (including an empty body) returns Error{Code:"response_json"}.
func (c *Client) GetJSON(ctx context.Context, url string, headers map[string]string, out any) error
```

File: `internal/httpkit/errors.go`

```go
package httpkit

// Error is a domain error carrying a stable Failure.Code string from
// specs/global/CONTRACTS.md §1.6. StatusCode is the HTTP status (0 when the
// failure is not HTTP-level: network, timeout, parse, allow-list, redirect).
// Err holds the underlying cause for diagnosis; it is never rendered into
// Error(). Callers MUST branch on Code/StatusCode via errors.As — message
// text is sanitized and is NOT a contract (never match on it).
type Error struct {
    Code       string // endpoint_refused | redirect_refused | response_too_large | timeout | network | response_json | unauthorized | rate_limited | provider_status
    StatusCode int    // HTTP status; 0 when not an HTTP-level failure
    Err        error
}

// Error renders a fixed, sanitized message per code. It never includes
// request URLs, headers, tokens, or the underlying Err text.
func (e *Error) Error() string

// AsError extracts an *Error from err (or err itself), reporting whether
// the extraction succeeded.
func AsError(err error) (*Error, bool)
```

## 2. Fixed sanitized messages

`Error()` renders `<code>: <message>` with these fixed messages (ported verbatim from `docs/plan/research/usage-allowance-checks-spec.md` §1, core.mjs):

| Code | Message |
|---|---|
| `endpoint_refused` | `the provider endpoint is not a valid URL` (parse failure) / `the provider endpoint was refused` (any other refusal) |
| `redirect_refused` | `the provider attempted an unsafe redirect` |
| `response_too_large` | `the provider response exceeded the safe size limit` |
| `timeout` | `the provider request timed out` |
| `network` | `the provider request failed` |
| `response_json` | `the provider returned unsupported JSON` |
| `unauthorized` | `the provider rejected the credential` |
| `rate_limited` | `the provider rate-limited the request` |
| `provider_status` | `the provider request failed` |

## 3. Error code mapping (condition → `Error.Code`)

| Condition | Code |
|---|---|
| URL fails allow-list (non-https / userinfo / fragment / not an exact member) | `endpoint_refused` |
| `CheckRedirect` fired (a redirect was attempted) or any 3xx status | `redirect_refused` |
| `Content-Length` > bound, or streamed body > bound | `response_too_large` |
| `context.DeadlineExceeded` | `timeout` |
| `context.Canceled` or any other transport error | `network` |
| `GetJSON` unmarshal failure (incl. empty body) | `response_json` |
| HTTP 401 or 403 (final attempt) | `unauthorized` |
| HTTP 429 (final attempt) | `rate_limited` |
| Any other HTTP >= 400 (final attempt) | `provider_status` |

All codes are stable values from `specs/global/CONTRACTS.md` §1.6; F04 adds none. Callers convert to `usage.Failure{Code: e.Code, Message: <fixed message>}` at their own package boundary (F08, F14) — `internal/httpkit` MUST NOT import `internal/usage` (`specs/global/CONTRACTS.md` §8).

## 4. Ownership

- **Flags owned:** none. Global `--timeout` is declared by F22-cli-skeleton and passed down by F08/F14 as a context deadline.
- **Config keys owned:** none.
- **Error codes added:** none (`specs/global/CONTRACTS.md` §1.6 table is unchanged).
- **JSON shapes emitted:** none (bounded raw bytes out of `Do`; `GetJSON` unmarshals into caller-provided shapes).
- **Retry semantics:** 1 retry / 500ms / 5xx+network only; 4xx, 3xx, `response_too_large`, `timeout`, allow-list refusal never retried; only replayable requests (`Body == nil` or `GetBody != nil`) are replayed; backoff sleep honors context cancellation. An exhausted 5xx retry budget returns `*Error{Code:"provider_status", StatusCode:<status>}`, never a body.
