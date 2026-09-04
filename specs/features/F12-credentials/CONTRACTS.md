---
kind: feature-contracts
version: "1.0"
feature: F12-credentials
project: which-model
---

# F12 — Credentials: Contracts

Package: `internal/usage/credential` (Layer 1b). Import boundary (global CONTRACTS §8): MAY import `internal/config`, `internal/security`, `internal/httpkit`; MUST NOT import `internal/catalog`, `internal/routing`, `internal/pick`. F12's task-level dependency set is F05 (`internal/security`) + F11 (`internal/usage`) only.

Build tags: EVERY file in this package carries `//go:build !nousage` (annex-a §1a.2). The `nousage`-tagged package-presence stub is owned by F21-usage-toggle.

New dependency: `github.com/zalando/go-keyring` (darwin-only, referenced only from `keychain_darwin.go`; see SPEC decision D2).

---

## 1. Core — `internal/usage/credential/credential.go`

```go
package credential

import (
    "context"
    "net/http"

    "github.com/WD-Mitchell/which-model/internal/usage"
)

// Credential is F11's type, re-exported for readable resolver signatures.
// Never log it: String() is redacted (F11).
type Credential = usage.Credential

// Warning is a human-readable, sanitised diagnostic (never credential
// material). Printed by the CLI to stderr (annex-d §1).
type Warning struct {
    Message string
}

// ErrNotFound is the sentinel for "no candidate from this source; continue
// the chain". NOT a *usage.FailureError — compare with errors.Is.
var ErrNotFound = errors.New("credential not found")

// Resolver resolves one credential candidate, or ErrNotFound when this
// source yields no candidate. Any other error is a hard, Failure-coded
// error (usage.NewFailureError) that aborts the chain.
type Resolver interface {
    Resolve(ctx context.Context) (usage.Credential, error)
}

// ResolveChain walks sources in order (SPEC §11). First candidate that
// resolves AND passes Validate wins. Hard errors abort. Exhaustion →
// ErrNotFound (F14 maps it to login_required). Kinds without an F12
// resolver (RPC/refresh/sigv4/volcengine/grpc-web) → ErrNotFound.
func ResolveChain(ctx context.Context, sources []usage.AuthSource, client *http.Client) (usage.Credential, []Warning, error)
```

## 2. Env — `internal/usage/credential/env.go`

```go
// EnvResolver resolves Var from the environment; Extra names are copied
// into Credential.Extra when set. Missing / empty-after-trim / unsafe →
// ErrNotFound. Matching surrounding quotes are stripped (SPEC §2).
type EnvResolver struct {
    Var   string
    Extra []string
}

func (r *EnvResolver) Resolve(ctx context.Context) (usage.Credential, error)
```

## 3. File — `internal/usage/credential/file.go`

```go
// FileResolver reads ordered candidate paths (SPEC §3). Missing path →
// ErrNotFound (next path); unreadable/oversized → credential_file;
// invalid JSON / non-object → credential_json; missing JSONPath value →
// ErrNotFound; unsafe token → unsafe_credential; expired (ExpiryPath) →
// expired_credential. ExtraPaths populate Credential.Extra.
type FileResolver struct {
    Paths      []string
    JSONPath   string
    ExtraPaths map[string]string
    ExpiryPath string
}

func (r *FileResolver) Resolve(ctx context.Context) (usage.Credential, error)

// Warnings returns the permission warnings recorded by the last Resolve
// call (SPEC §4). Empty when the winning file mode is 0600/0700-clean.
func (r *FileResolver) Warnings() []string
```

## 4. CLI — `internal/usage/credential/cli.go`

```go
// MaxCLIOutputBytes caps a shell-out's stdout (prototype maxBuffer 32_768).
const MaxCLIOutputBytes = 32 * 1024

// CLIResolver runs Command+Args; every failure (non-zero exit, timeout,
// output over cap, empty/unsafe output) → ErrNotFound (SPEC §5). Strips
// exactly one trailing \r\n or \n. Secrets are never passed via argv/env;
// future secret-input subprocesses receive secrets on stdin (SPEC D1).
type CLIResolver struct {
    Command        string
    Args           []string
    Timeout        time.Duration
    MaxOutputBytes int64 // <= 0 → MaxCLIOutputBytes
}

func (r *CLIResolver) Resolve(ctx context.Context) (usage.Credential, error)
```

## 5. Keychain — `internal/usage/credential/keychain.go` (+ `keychain_darwin.go`, `keychain_other.go`)

```go
// KeychainStore abstracts the OS keychain so tests use a fake.
type KeychainStore interface {
    Get(service, account string) (string, error)
}

// ManagedKeychainStore adds the mutations used only for credentials written
// by which-model.
type ManagedKeychainStore interface {
    KeychainStore
    Set(service, account, value string) error
    Delete(service, account string) error
}

// KeychainResolver resolves a generic-password item. Not-found →
// ErrNotFound; other store errors → keychain_unavailable.
type KeychainResolver struct {
    Store   KeychainStore
    Service string
    Account string
}

func (r *KeychainResolver) Resolve(ctx context.Context) (usage.Credential, error)

// DefaultKeychain returns DarwinKeychain on darwin (wrapping
// github.com/zalando/go-keyring) and UnavailableKeychain elsewhere.
// Both implement Get/Set/Delete; unavailable operations return ErrNotFound.
// Build-tagged implementations:
//   keychain_darwin.go — //go:build darwin && !nousage
//   keychain_other.go  — //go:build !darwin && !nousage
func DefaultKeychain() ManagedKeychainStore
```

### 5.1 Managed credentials — `internal/usage/credential/managed.go`

```go
// ManagedStore persists credentials created by which-model. Keychain storage
// uses service "which-model" and account <provider>. When disabled or when a
// keychain operation is unavailable, storage falls back to
// <StateDir>/credentials/<provider>.json (atomic mode 0600).
type ManagedStore struct {
    StateDir    string
    Keychain    ManagedKeychainStore
    UseKeychain bool
}

func (s ManagedStore) Path(provider string) string
func (s ManagedStore) Save(provider, token string) error
func (s ManagedStore) Resolve(ctx context.Context, provider string) (usage.Credential, []Warning, error)
// Remove deletes both managed locations regardless of UseKeychain so a
// preference change cannot strand an active credential.
func (s ManagedStore) Remove(provider string) error

// ResolveProvider preserves the descriptor's declared source order, then
// tries managed storage only for a provider with AuthOAuthDeviceFlow and runs
// that source's Validate hook before accepting the managed token.
func ResolveProvider(ctx context.Context, provider string, sources []usage.AuthSource, client *http.Client, store ManagedStore) (usage.Credential, []Warning, error)
```

## 6. Cookie — `internal/usage/credential/cookie.go`

```go
// CookieResolver is the interface browser-cookie extraction will implement
// in M5 (github.com/browserutils/kooky, annex-a §4). F12 ships no
// implementation; ResolveChain treats AuthBrowserCookie as ErrNotFound
// (SPEC D3 — stated explicitly: F12 performs no browser access).
type CookieResolver interface {
    Resolve(ctx context.Context, spec usage.CookieSpec) (usage.Credential, error)
}
```

## 7. OAuth device flow — `internal/usage/credential/deviceflow.go`

```go
// DeviceFlow is the generic device-code state machine (SPEC §9).
// Now and Sleep are injectable test seams; defaults time.Now/time.Sleep.
// ValidateURL defaults to security.ValidateExactHTTPS(rawURL, []string{rawURL});
// tests replace it with a no-op to use httptest servers.
type DeviceFlow struct {
    Spec           usage.OAuthSpec
    HTTPClient     *http.Client // default: redirects hard-fail (CheckRedirect → http.ErrUseLastResponse)
    MaxResponseBytes int64      // <= 0 → security.MaxResponseBytes
    Now            func() time.Time
    Sleep          func(d time.Duration)
    ValidateURL    func(rawURL string) error
}

func NewDeviceFlow(spec usage.OAuthSpec) *DeviceFlow

// DeviceCode carries the validated device-flow state. Only UserCode and
// VerificationURI may ever be displayed; DeviceCode is opaque.
type DeviceCode struct {
    DeviceCode     string
    UserCode       string
    VerificationURI string
    ExpiresIn      time.Duration
    Interval       time.Duration
}

// Start POSTs DeviceCodeURL (form client_id+scope) and validates every
// response field (SPEC §9). Violations → unsupported_response.
func (f *DeviceFlow) Start(ctx context.Context) (DeviceCode, error)

// Poll runs the deadline-checked polling loop (SPEC §9): slow_down →
// interval += 5s; access_denied → access_denied; expired_token/deadline →
// device_expired; unknown → unsupported_response. Success returns the
// validated opaque token.
func (f *DeviceFlow) Poll(ctx context.Context, code DeviceCode) (string, error)
```

## 8. Expiry — `internal/usage/credential/expiry.go`

```go
// ParseExpiry converts a decoded JSON value to a time.Time: number epochs
// with the >10_000_000_000 = milliseconds heuristic (prototype resetText);
// strings parsed as RFC3339/RFC3339Nano or as numeric strings.
func ParseExpiry(v any) (time.Time, error)

// CheckExpired returns nil when exp is after now, else an
// expired_credential *usage.FailureError.
func CheckExpired(exp time.Time, now time.Time) error
```

## 9. Ownership summary

| Surface | Value |
|---|---|
| Config keys owned | none |
| Flags owned | none (F25 cmd-auth owns auth flags; F24 owns usage flags) |
| Error codes added | none — all errors carry canonical codes from `specs/global/CONTRACTS.md` §1.6; device-flow malformed responses map to `unsupported_response` (SPEC D6) |
| JSON shapes emitted | none |
| Dependencies added | `github.com/zalando/go-keyring` (darwin build only) |
| Depends on | F05, F11 (per `specs/DEPENDENCY-GRAPH.md` §2) |
| Blocks | F14, F15, F16, F17, F25 (per `specs/DEPENDENCY-GRAPH.md` §2) |

## Review regression contract (#176, #177)

`FileResolver.Paths` accepts single-pass leading-home and environment expansion before stat/read; missing/empty variables skip only that candidate. `KeychainResolver` recognizes `errors.Is` for both not-found sentinels. No public type changes.

| Input | Required result |
|---|---|
| `~/auth.json`, `$VAR/auth.json`, `${VAR}/auth.json` | Resolve temporary synthetic credential |
| Missing/empty override followed by a valid home path | Skip override, resolve home credential |
| Multiple valid candidates | First candidate wins |
| Literal absolute/relative paths or shell-like text in environment value | Preserve literal path; no execution or recursive expansion |
| Unsupported-platform keychain or wrapped not-found | Continue to file source |
| Locked/denied keychain with secret-bearing error | `keychain_unavailable`, no secret in failure |
