---
kind: feature-contracts
feature: F05-security
version: "1.0"
project: which-model
---

# F05 — security: CONTRACTS

Package `internal/security`, files under `internal/security/`. Imports: Go stdlib only (`os`, `io`, `net/url`, `net/http`, `unicode`, `strings`, `fmt`, `errors`, `io/fs`) — Layer 0 (`specs/global/SPEC.md` §3). The exported surface below is **pinned**; no additional exported symbols may be added without a plan-level decision. All functions are safe for concurrent use (no shared mutable state).

## 1. Exported surface (pinned)

File: `internal/security/security.go`

```go
package security

import (
    "io/fs"
    "net/http"
)

// MaxCredentialBytes bounds credential files (specs/global/CONTRACTS.md §7).
const MaxCredentialBytes = 1_048_576 // 1 MiB

// MaxResponseBytes bounds HTTP response bodies (specs/global/CONTRACTS.md §7).
const MaxResponseBytes = 262_144 // 256 KiB

// Error is a domain error carrying a stable Failure.Code string from
// specs/global/CONTRACTS.md §1.6 and a fixed, sanitized message.
type Error struct {
    Code    string // unsafe_credential | credential_file | endpoint_refused | untrusted_origin | response_too_large
    Message string // fixed constant; never interpolates input or underlying errors
}

// Error renders "<code>: <message>".
func (e *Error) Error() string
```

File: `internal/security/token.go`

```go
// ValidateOpaqueToken ports assertOpaque (core.mjs:16-25): accepts a non-empty
// token of at most 8192 bytes with no whitespace, no control characters, and
// no DEL. Any violation returns Error{Code:"unsafe_credential",
// Message:"The credential is missing or unsafe."} — the token itself is never
// echoed. Returns nil on success.
func ValidateOpaqueToken(token string) error
```

File: `internal/security/files.go`

```go
// ReadBoundedFile ports readBoundedFile (core.mjs:37-60). The file must be a
// regular file whose size is >= 1 and <= maxBytes (checked via stat, then
// re-checked on the actually-read bytes); the bytes and the file's mode are
// returned so the caller can warn on broad permissions. Missing/non-regular
// -> Error{Code:"credential_file", Message:"The credential file was not found."}
// Size violations -> Error{Code:"credential_file",
// Message:"The credential file has an invalid size."}  Any other I/O failure
// -> Error{Code:"credential_file",
// Message:"The credential file could not be read safely."}  Underlying error
// details are never surfaced. Never modifies the file.
func ReadBoundedFile(path string, maxBytes int64) ([]byte, fs.FileMode, error)

// HasBroadPermissions ports hasBroadPermissions (core.mjs:72-74): true when
// any group or other rwx bit is set, i.e. mode.Perm() & 0o077 != 0.
// The caller warns; nothing here ever chmod's the file.
func HasBroadPermissions(mode fs.FileMode) bool
```

File: `internal/security/url.go`

```go
// ValidateExactHTTPS ports validateExactHttpsUrl (core.mjs:76-91): rawURL must
// parse, be https, carry no userinfo and no fragment, have a non-empty host,
// and its Go-serialized form ((*url.URL).String()) must be an exact member of
// allowed — no prefix/origin/substring matching. Returns the canonical
// serialized URL. Parse failure -> Error{Code:"endpoint_refused",
// Message:"The provider endpoint is not a valid URL."}; any other rejection
// -> Error{Code:"endpoint_refused", Message:"The provider endpoint was refused."}
func ValidateExactHTTPS(rawURL string, allowed []string) (string, error)

// ValidateTrustedBaseURL ports validateTrustedBaseUrl (core.mjs:93-116):
// base must be https with no userinfo/query/fragment; trustedOrigin must be
// https with no userinfo/query/fragment and a bare path ("" or "/"); base's
// origin (scheme://host incl. port) must equal trustedOrigin's origin exactly.
// Returns the fallback target URL:
// base origin + base path (trailing slash ensured) + "api/codex/usage".
// Any violation -> Error{Code:"untrusted_origin",
// Message:"The configured Codex fallback origin was not explicitly trusted."}
func ValidateTrustedBaseURL(rawURL string, trustedOrigin string) (string, error)
```

File: `internal/security/response.go`

```go
// ReadResponseBounded ports readResponseText (core.mjs:118-144): fails with
// Error{Code:"response_too_large", Message:"The provider response exceeded the
// safe size limit."} when resp.ContentLength (the parsed Content-Length
// header) exceeds maxBytes, or when the body read through a maxBytes+1
// limited reader exceeds maxBytes — checked twice. Does not close resp.Body.
// Non-oversize read errors are returned unwrapped.
func ReadResponseBounded(resp *http.Response, maxBytes int64) ([]byte, error)
```

File: `internal/security/canary.go`

```go
// ErrCanaryLeak is returned by WithCanary when the canary string appears in
// the returned error's text. Its message is fixed and never echoes the canary
// or the offending error.
var ErrCanaryLeak = errors.New("security: canary token leaked into error text")

// WithCanary runs fn. If fn returns an error whose text contains canary,
// WithCanary returns ErrCanaryLeak (detectable via errors.Is); otherwise it
// returns fn's error unchanged (nil stays nil).
func WithCanary(canary string, fn func() error) error
```

## 2. Fixed sanitized messages

| Code | Message |
|---|---|
| `unsafe_credential` | `The credential is missing or unsafe.` |
| `credential_file` (missing/non-regular) | `The credential file was not found.` |
| `credential_file` (size) | `The credential file has an invalid size.` |
| `credential_file` (other I/O) | `The credential file could not be read safely.` |
| `endpoint_refused` (parse) | `The provider endpoint is not a valid URL.` |
| `endpoint_refused` (other) | `The provider endpoint was refused.` |
| `untrusted_origin` | `The configured Codex fallback origin was not explicitly trusted.` |
| `response_too_large` | `The provider response exceeded the safe size limit.` |

All codes are stable values from `specs/global/CONTRACTS.md` §1.6; F05 adds none. Messages are ported verbatim from `usage-allowance-checks/lib/core.mjs`.

## 3. Ownership

- **Flags owned:** none. (`--trust-configured-origin` is F16-provider-codex's per-invocation flag; `--show-identity` is F22's global flag.)
- **Config keys owned:** none.
- **Error codes added:** none (`specs/global/CONTRACTS.md` §1.6 table is unchanged).
- **JSON shapes emitted:** none.
- **Import boundary:** `internal/security` MAY import `internal/config` and MUST NOT import `internal/usage` or `internal/catalog` (`specs/global/CONTRACTS.md` §8); the pinned surface imports stdlib only.
