---
kind: feature-contracts
version: "1.0"
feature: F11-usage-types
project: which-model
---

# F11 — Usage Types: Contracts

Package: `internal/usage` (Layer 1b). Import boundary (global CONTRACTS §8): MAY import `internal/config`, `internal/security`, `internal/httpkit`; MUST NOT import `internal/catalog`, `internal/routing`, `internal/pick`.

Build tags: `types.go` — no tag (compiles in both variants). `credential.go`, `descriptor.go`, `registry.go` — `//go:build !nousage`. The inverse-tag stub `internal/usage/disabled.go` is owned by F21-usage-toggle and mirrors this exported surface.

---

## 1. Canonical types — `internal/usage/types.go` (no build tag)

Defined **verbatim** from `specs/global/CONTRACTS.md` §1.1–§1.6 (same package, same fields, same JSON tags). Not repeated here; the global file is canonical. Types: `Unit` (+7 constants), `Source` (+6 constants), `Kind` (iota enum, 4 constants), `Window`, `Snapshot`, `Failure`.

Additional member in `types.go`:

```go
// String renders the Kind for humans ("subscription", "api_key_billing", "gateway", "local_tool", "unknown").
func (k Kind) String() string
```

## 2. AuthKind — `internal/usage/credential.go` (`//go:build !nousage`)

```go
// AuthKind discriminates an AuthSource's populated fields / a Credential's origin.
// Exactly one kind-tagged sub-spec on an AuthSource is non-nil for a given Kind value.
type AuthKind int

const (
    AuthEnvVar AuthKind = iota
    AuthFile
    AuthKeychainGeneric
    AuthKeychainInternet
    AuthBrowserCookie
    AuthCLIShellOut
    AuthSubprocessRPC
    AuthOAuthDeviceFlow
    AuthOAuthRefreshGrant
    AuthAWSSigV4
    AuthVolcengineAKSK
    AuthGRPCWebToken // Connect-RPC / gRPC-Web / raw-protobuf token carriers
)

func (k AuthKind) String() string
```

## 3. Credential — `internal/usage/credential.go` (`//go:build !nousage`)

```go
// Credential is the resolved secret handed to a FetchFunc. It never
// round-trips through logs, errors, or Failure.Message (global SPEC §6 invariant 5).
type Credential struct {
    Token  string            // opaque bearer/API token, already ValidateOpaqueToken-validated
    Extra  map[string]string // secondary fields: account_id, project_id, cookie header, ...
    Source AuthKind          // which AuthSource kind in the chain produced it
    Mode   uint32            // credential file POSIX mode, when Source == AuthFile (0 otherwise)
}

// String returns a redacted rendering; NEVER contains Token or any Extra value.
// e.g. `Credential{source=file, token=<redacted>}`.
func (c Credential) String() string
```

## 4. FailureError — `internal/usage/credential.go` (`//go:build !nousage`)

```go
// FailureError carries a stable Failure.Code through the error path.
// Every resolver, the fetch layer, and providers construct/consume this type.
type FailureError struct {
    Failure Failure
}

func (e *FailureError) Error() string // "<code>: <message>"

// NewFailureError builds an error carrying the given canonical Failure.Code.
// message MUST be sanitised (no credential material) at the call site.
func NewFailureError(code, message string) error

// AsFailure extracts a Failure from err; ok=false when err is not (or does not
// wrap) a *FailureError.
func AsFailure(err error) (Failure, bool)
```

## 5. Descriptor types — `internal/usage/descriptor.go` (`//go:build !nousage`)

```go
package usage

import (
    "context"
    "net/http"
    "time"
)

// WindowSpec is descriptor-time metadata: which window IDs/labels/units a
// provider MAY report, and which models each window's quota applies to.
// It is NOT a runtime reading — that is Window (global CONTRACTS §1.4).
type WindowSpec struct {
    ID          string
    Label       string
    Unit        Unit
    Optional    bool     // provider may omit this window depending on plan/quota shape
    ModelScope  []string // model IDs this window's quota applies to (F18 BindWindowIDs, annex-b §7.3)
}

type KeychainSpec struct {
    Service string // e.g. "Claude Code-credentials"
    Account string // "" = match any account for the service
    Server  string // internet-password only, e.g. "zed.dev"
}

type CookieSpec struct {
    Domains     []string // exact cookie-jar domains to query, never a wildcard
    CookieNames []string // strict-name pass first; empty = accept any non-empty set for Domains
}

type ShellSpec struct {
    Command string
    Args    []string
    Timeout time.Duration
}

type RPCSpec struct {
    Command     string
    Args        []string
    InitTimeout time.Duration
    CallTimeout time.Duration
    Method      string // JSON-RPC method invoked after initialize, e.g. "account/rateLimits/read"
}

type OAuthSpec struct {
    ClientID        string
    ClientSecret    string        // empty for public clients (Codex, Claude, Copilot)
    DeviceCodeURL   string        // device flow only
    TokenURL        string
    Scope           string
    VerificationURI string        // device flow: EXACT match for the server-provided verification_uri (F12)
}

// AuthSource is one ordered link in a provider's credential-resolution chain.
// Descriptor.Auth is walked in order (F12 ResolveChain); the first entry that
// both resolves a candidate AND (when Validate is set) passes validation wins.
type AuthSource struct {
    Kind AuthKind

    EnvVar    string   // AuthEnvVar
    FilePaths []string // AuthFile: ordered candidate paths, first existing+valid wins
    JSONPath  string   // AuthFile: dotted path to the token field, e.g. "tokens.access_token"
    ExpiryPath string  // AuthFile: dotted path to the expiry epoch/date; "" = no expiry check

    Keychain *KeychainSpec // AuthKeychainGeneric / AuthKeychainInternet
    Cookie   *CookieSpec   // AuthBrowserCookie
    Shell    *ShellSpec    // AuthCLIShellOut
    RPC      *RPCSpec      // AuthSubprocessRPC
    OAuth    *OAuthSpec    // AuthOAuthDeviceFlow / AuthOAuthRefreshGrant

    // ExtraPaths: Extra-field name → dotted JSON path inside the credential
    // file, e.g. {"account_id": "tokens.account_id", "base_url": "flat base_url"}.
    ExtraPaths map[string]string
    // EnvExtra: extra env var names copied into Credential.Extra when set
    // (e.g. "OPENAI_PROJECT_ID").
    EnvExtra []string

    // Validate gates candidate acceptance (e.g. Copilot's mandatory GET /user
    // identity check). A candidate that fails Validate is skipped, not fatal.
    Validate func(ctx context.Context, candidate Credential, client *http.Client) error
}

// FetchFunc performs one provider's usage fetch given a resolved credential.
// Provider packages leave Snapshot.Source unset — the fetch layer (F14) owns
// Source population (see specs/features/F14-usage-fetch/CONTRACTS.md §3).
type FetchFunc func(ctx context.Context, cred Credential, client *http.Client) (Snapshot, error)

// Descriptor is exactly the contract type for one provider adapter.
type Descriptor struct {
    ID           string
    DisplayName  string
    Kind         Kind
    Tier         int           // 1 = port first, 2 = second wave, 3 = deferred
    Auth         []AuthSource
    Windows      []WindowSpec
    Timeout      time.Duration // per-fetch timeout; 0 = use global DefaultTimeoutSec (10s)
    CacheTTL     time.Duration // usage cache TTL; 0 = this provider is never cached (F13/F14)
    Fetch        FetchFunc
    LastVerified time.Time     // zero = never verified; populated by F25 auth flows
}
```

## 6. Registry — `internal/usage/registry.go` (`//go:build !nousage`)

```go
package usage

// UnknownProviderError is the typed error returned by Get for unknown IDs.
// Match with errors.As(&usage.UnknownProviderError{}).
type UnknownProviderError struct {
    ID string
}

func (e *UnknownProviderError) Error() string // `unknown provider "<id>"`

// Register adds a Descriptor to the global registry. Called from each
// provider package's init(). PANICS on a duplicate ID (programming error).
func Register(d Descriptor)

// Get returns the Descriptor for id, or *UnknownProviderError when unknown.
func Get(id string) (Descriptor, error)

// All returns every registered Descriptor, sorted lexicographically by ID.
func All() []Descriptor

// IDs returns every registered provider ID, sorted lexicographically.
func IDs() []string
```

## 7. Ownership summary

| Surface | Value |
|---|---|
| Config keys owned | none |
| Flags owned | none |
| Error codes added | none (uses canonical `Failure.Code` set from `specs/global/CONTRACTS.md` §1.6) |
| JSON shapes emitted | none (types only; registry is compile-time) |
| Dependencies added | none (stdlib only) |
| Depends on | F05 (`internal/security` per dependency graph; F11's tasks do not call F05 symbols) |
| Blocks | F12, F13, F14, F18, F19, F21 (per `specs/DEPENDENCY-GRAPH.md` §2) |
