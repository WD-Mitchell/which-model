---
kind: global-contracts
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
---

# which-model — Global Contracts

All types in this file are **canonical**. Feature specs MUST use them verbatim. No feature may add fields, rename fields, or create provider-specific variants.

Source: `docs/plan/README.md` §3.2, Annex A §5, Annex B §5, Annex D §1.5.

---

## 1. Usage types (`internal/usage`)

### 1.1 Unit

```go
package usage

type Unit string

const (
    UnitPercent   Unit = "percent"
    UnitTokens    Unit = "tokens"
    UnitCredits   Unit = "credits"
    UnitUSD       Unit = "usd"
    UnitRequests  Unit = "requests"
    UnitEnergyKWh Unit = "kwh"
    UnitNone      Unit = "none"
)
```

### 1.2 Source

```go
type Source string

const (
    SourceOAuth Source = "oauth"
    SourceAPI   Source = "api"
    SourceCLI   Source = "cli"
    SourceWeb   Source = "web"
    SourceLocal Source = "local"
    SourceCache Source = "cache"
)
```

### 1.3 Kind

```go
type Kind int

const (
    KindSubscription   Kind = iota // Claude, Codex, Copilot, Cursor, etc.
    KindAPIKeyBilling              // OpenAI platform, DeepInfra, etc.
    KindGateway                    // OpenRouter, LiteLLM, ClawRouter, etc.
    KindLocalTool                  // Ollama, JetBrains (presence-only)
)
```

### 1.4 Window

```go
type Window struct {
    ID            string     `json:"id"`
    Label         string     `json:"label"`
    Unit          Unit       `json:"unit"`
    UsedPercent   *float64   `json:"used_percent,omitempty"`
    Used          *float64   `json:"used,omitempty"`
    Limit         *float64   `json:"limit,omitempty"`
    Remaining     *float64   `json:"remaining,omitempty"`
    Unlimited     bool       `json:"unlimited,omitempty"`
    WindowMinutes *int       `json:"window_minutes,omitempty"`
    ResetsAt      *time.Time `json:"resets_at,omitempty"`
    ResetHint     string     `json:"reset_hint,omitempty"`
    ModelScope    []string   `json:"model_scope,omitempty"`
    Synthetic     bool       `json:"synthetic,omitempty"`
    UsageKnown    bool       `json:"usage_known"`
}
```

### 1.5 Snapshot

```go
type Snapshot struct {
    Provider   string    `json:"provider"`
    Account    string    `json:"account,omitempty"`
    Plan       string    `json:"plan,omitempty"`
    Windows    []Window  `json:"windows"`
    FetchedAt  time.Time `json:"fetched_at"`
    Source     Source    `json:"source"`
    Confidence string    `json:"confidence"` // "live" | "cached" | "estimated"
    UsageKnown bool      `json:"usage_known"` // at least one window carries a real reading (see DEFERRED D5)
    Stale      bool      `json:"stale,omitempty"`
    Failure    *Failure  `json:"error,omitempty"`
}
```

### 1.6 Failure

```go
type Failure struct {
    Code    string `json:"code"`
    Message string `json:"message"` // sanitised; NEVER contains credential material
}
```

**Stable `Failure.Code` values** (from `docs/plan/annex-a-provider-matrix.md` §7):

| Code | Meaning | Exit code |
|---|---|---|
| `unauthorized` | Provider rejected credential (401/403) | 5 |
| `rate_limited` | Provider rate-limited (429) | 1 |
| `provider_status` | Non-2xx not covered by a specific code | 1 |
| `expired_credential` | Known expiry in the past | 5 |
| `unsupported_response` | JSON parsed but wrong shape | 1 |
| `login_required` | No credential found, no --login | 5 |
| `endpoint_refused` | URL failed exact allow-list | 1 |
| `untrusted_origin` | Fallback origin not explicitly trusted | 1 |
| `redirect_refused` | Server attempted 3xx | 1 |
| `response_too_large` | Body exceeded 256 KiB | 1 |
| `timeout` | Request/subprocess exceeded deadline | 1 |
| `network` | Transport failure (DNS/TCP/TLS) | 1 |
| `response_json` | Empty or unparseable JSON | 1 |
| `credential_file` | Missing/unreadable/oversized | 5 |
| `credential_json` | Not valid JSON or not an object | 5 |
| `unsafe_credential` | Failed opaque-token shape check | 5 |
| `access_denied` | User denied/cancelled OAuth | 5 |
| `device_expired` | Device-flow deadline passed | 5 |
| `fallback_unavailable` | No fallback configured/cached | 1 |
| `usage_disabled` | Usage off by flag/config | 2 |
| `usage_compiled_out` | Binary built with `-tags nousage` | 2 |
| `keychain_unavailable` | Keychain API error (locked/denied) | 1 |
| `cookie_unavailable` | Browser-cookie extraction failed | 5 |
| `signing_failed` | AWS SigV4 / Volcengine signing error | 5 |
| `rpc_protocol` | Subprocess RPC framing malformed | 1 |

---

## 2. Catalog types (`internal/catalog`, `internal/pick`)

### 2.1 ScoreRow

```go
package catalog

import "github.com/shopspring/decimal"

type ScoreRow struct {
    Model      string
    Reasoning  string // minimal|low|medium|high|xhigh|max|default
    Tier1      map[string]decimal.Decimal // intelligence, cost, speed
    Categories map[string]decimal.Decimal // 12 category composites
    Benchmarks map[string]decimal.Decimal // dynamic benchmark columns
}
```

### 2.2 Normalizer / Aggregator interfaces

```go
package score

import "github.com/shopspring/decimal"

type Normalizer interface {
    Normalize(raw decimal.Decimal, min, max decimal.Decimal) decimal.Decimal
}

type Aggregator interface {
    Aggregate(values []decimal.Decimal, weights []decimal.Decimal) (decimal.Decimal, bool)
}
```

Default implementations: `MinMaxLinear` and `WeightedArithmeticMean`.
`Aggregate`'s bool reports an empty denominator (SPEC D1 blank-exclusion:
never zero-impute); direction is applied by the derive layer BEFORE
`Normalize` via the reflection v' = min + max − v (F09 CONTRACTS §2.2).
Signatures pinned by F09 — see DEFERRED D9.

---

## 3. Routing types (`internal/routing`)

### 3.1 Route

```go
package routing

type Provenance string

const (
    ProvenanceProviderLive Provenance = "provider_live"
    ProvenanceModelsDev    Provenance = "models_dev"
    ProvenanceUserDeclared Provenance = "user_declared"
)

type Route struct {
    Provider   string     `json:"provider"`
    ModelID    string     `json:"model_id"`
    Model      string     `json:"model"`
    Reasoning  string     `json:"reasoning"`
    WindowIDs  []string   `json:"window_ids"`
    Provenance Provenance `json:"provenance"`
}
```

---

## 4. Selection types (`internal/pick`)

### 4.1 Candidate

```go
package pick

import "github.com/shopspring/decimal"

type Candidate struct {
    Route          routing.Route
    ModelScore     decimal.Decimal
    Band           string          // band name, omitted when usage disabled
    BandWeight     decimal.Decimal // 1.0 when usage disabled
    ProviderWeight decimal.Decimal
    FinalScore     decimal.Decimal
    Warnings       []string
}
```

### 4.2 Strategy enum

```go
type Strategy string

const (
    StrategyPriority       Strategy = "priority"
    StrategyRoundRobin     Strategy = "round-robin"
    StrategyLeastUsed      Strategy = "least-used"
    StrategyMostUsed       Strategy = "most-used"
    StrategyClosestToReset Strategy = "closest-to-reset"
)
```

### 4.3 Profile

```go
package catalog // Go file: internal/catalog/types.go (DEFERRED D9)

import "github.com/shopspring/decimal"
type Profile struct {
    Name         string
    Tier1Share   decimal.Decimal
    Tier2Share   decimal.Decimal
    Tier1Weights map[string]decimal.Decimal
    Tier2Weights map[string]decimal.Decimal
}
```

---

## 5. Config types (`internal/config`)

### 5.1 UsageToggle

```go
type UsageEnabled string

const (
    UsageAuto  UsageEnabled = "auto"  // enabled iff ≥1 provider enabled
    UsageTrue  UsageEnabled = "true"
    UsageFalse UsageEnabled = "false"
)
```

---

## 6. Output envelope

Every `--json` output carries:

```go
type OutputEnvelope struct {
    SchemaVersion        string `json:"schema_version"` // "2.0"
    UsageEnabled         bool   `json:"usage_enabled"`
    UsageDisabledReason  string `json:"usage_disabled_reason,omitempty"` // flag|config|backend_off|compiled_out|no_providers_enabled
}
```

---

## 7. Constants

```go
const (
    MaxCredentialBytes = 1_048_576  // 1 MiB
    MaxResponseBytes   = 262_144   // 256 KiB
    DefaultTimeoutSec  = 10
    SchemaVersion      = "2.0"
)
```

---

## 8. Dependency constraints

These are **compile-time-enforced** import boundaries:

| Package | MAY import | MUST NOT import |
|---|---|---|
| `internal/company` | standard library, platform `x/sys` protection APIs | other `internal/` packages |
| `internal/securestore` | standard library, pinned native OS-store/D-Bus libraries | all other `internal/` packages |
| `internal/config` | `BurntSushi/toml`, `shopspring/decimal`, platform `x/sys` file replacement, `internal/company` | other `internal/` packages |
| `internal/decimal` | `shopspring/decimal` | anything in `internal/` |
| `internal/security` | `internal/config` | `internal/usage`, `internal/catalog` |
| `internal/catalog/*` | `internal/config`, `internal/decimal`, `internal/httpkit`, `internal/security`, `internal/company`, `internal/securestore` | `internal/usage`, `internal/routing`, `internal/pick` |
| `internal/usage/*` | `internal/config`, `internal/company`, `internal/security`, `internal/httpkit`, `internal/securestore` | `internal/catalog`, `internal/routing`, `internal/pick` |
| `internal/skills`, `internal/hooks` | standard library, `internal/company` | provider implementations |
| `internal/routing` | `internal/catalog/identity`, `internal/usage` (types only) | `internal/pick` |
| `internal/pick` | `internal/catalog`, `internal/routing`, `internal/usage` (types only) | `cmd/` |
| `pkg/whichmodel` | any `internal/` | — |
| `cmd/which-model` | `pkg/whichmodel` | direct `internal/` (goes through `pkg/`) |
| `cmd/which-model-score-only` | `pkg/scoreonly` | full CLI `pkg/whichmodel`, direct `internal/` |
| `pkg/scoreonly` | embedded `data`, `internal/catalog/score`, `internal/pick`, `internal/output` | configuration loaders, provider adapters, integrations, catalog collectors |

## 9. Tracked-path portability check

`python3 scripts/check_tracked_paths.py` validates paths from the Git index.
`--stdin` accepts the same NUL-delimited format for historical trees and tests.
Exit 0 prints `tracked-paths: OK (N files)`; exit 1 prints deterministic, escaped
path diagnostics to stderr; exit 2 reports failure to obtain the path list.
The conservative collision key is Unicode case-folding of every path prefix.

Pinned cases:

| Input paths | Outcome |
|---|---|
| `.github/ci.yml`, `src/CONSOLE.go` | accepted |
| `xd:/lsp`, a component containing newline or backslash | rejected |
| `nul.txt`, `COM1.log`, `LPT²`, `NUL .txt` | rejected |
| `a.`, `a /file`, `a//b`, `a/../b` | rejected |
| `Src/a.go` and `src/b.go` | rejected directory collision |
| `A` and `a/file` | rejected file/directory collision |
| `src/a.go` and `src/b.go` | accepted shared directory |

## 10. Release evidence and verification (#288)

`release-manifest.json` schema 1 contains `version`, `source_digest` (40 lowercase
hex), `source_ref` (`refs/heads/...` or `refs/tags/...`), and `artifacts`. Each
artifact entry has `name`, `sha256`, `sbom` and `sbom_sha256`; filenames are simple
basenames with no traversal. The build also emits `<binary>.cdx.json`,
`checksums.txt` and a Sigstore `provenance.jsonl` bundle.

The npm launcher includes `release-policy.json` containing the same manifest;
its version must match package.json before a fallback download. The verifier
pins repository, signer workflow, OIDC issuer, source ref/digest, SLSA v1 predicate
and hosted runner identity. A checksum match without successful cryptographic
verification is rejected. Direct verification uses an explicitly supplied version,
source ref and full source digest; mirror-supplied metadata cannot choose trust.

Pinned cases: changed bytes, absent/malformed bundle, incorrect repository,
workflow, source ref or source digest, mismatched package version, verifier
absence/timeout/failure and interrupted installation expose no new runnable
fallback. Optional-package success and download opt-out perform no fallback
network or verifier calls. A fallback receipt binds version/source/digest and is written only after successful
verification. The launcher refuses missing/mismatched receipts and changed bytes.
Existing personal configuration, credential and harness defaults are unchanged.

Restricted releases additionally require `score_only` on schema-1 manifests:
`{"capabilities":"which-model-score-only-capabilities.json","sha256":"<64 hex>"}`.
The verifier requires this entry whenever a restricted binary is listed, verifies
the capability file's signature/digest and source/version, and checks each
restricted SBOM's `bundled-catalog`/`bundled-profiles` components against its input
hashes. Historical full-only manifests remain valid. Embedded catalog checkout
uses LF bytes on every OS; restricted builds use trimpath, no build-date stamp and
`-buildvcs=false`, with full release commit supplied explicitly to the command.


## 11. Protected company policy (#282)

The [F01 policy contract](../features/F01-config/MANAGED-POLICY.md) owns the separate
administrator schema, fixed platform origins, precedence, protection checks and
inspection output. It adds no mutable authority to canonical Config or desktop DTOs.
Policy failures are exit 2; desktop mapping uses existing `validation_failed`.
Credential/provider/executable checks apply before effects, including direct callers.
Optional enrollment preserves personal defaults. The restricted score-only artifact
continues to exclude policy/configuration loading entirely.

## 12. Native secure-store outcomes (#283)

The internal leaf `securestore.Store` exposes Get/Set/Delete with typed missing,
locked, denied, unavailable and too_large outcomes. Only known messages escape
native adapters. Credential resolution maps these to the existing canonical
failure/sentinel contract; usage.Credential and public usage DTOs do not change.
Native implementations require `!nousage`; the restricted offline command does
not link the package. See [F12](../features/F12-credentials/SECURE-STORES.md).


## 13. Company data minimization and retention (#284)

`internal/privacy` may import company/config, the existing gofrs/flock dependency
and standard library. It imports no usage adapter, credential, HTTP or execution
package. Cache, service, hooks and CLI may call it; the restricted standalone
score-only binary does not link it. Canonical usage/selection/desktop DTOs do not
change. Internal persisted records carry `privacy_version:1` and the whitelist in
[F13 managed retention](../features/F13-usage-cache/MANAGED-RETENTION.md).

`privacy.Report` counts retained_records, removed_records, scrubbed_records,
deleted_files and failed_files. A failed write/delete produces failed_files and
never reports uncommitted removals as success. `privacy.Summary` is
`{managed,categories:{<category>:Report}}`; canonical category names are
usage_snapshots, pick_history, audit_records and launch_logs. Fixed errors expose
only category/operation, never paths or payloads. Bounded JSONL (64 MiB), record
(4 MiB), directory inventory (1,024 entries), and lock wait (2 seconds) prevent
unbounded individual operations; limits produce deletion of oversized owned data
or explicit incomplete-work errors, as specified in F13.
