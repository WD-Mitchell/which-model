---
kind: feature-contracts
version: "1.0"
feature: F21-usage-toggle
project: which-model
---

# F21 — Usage Toggle: CONTRACTS

## 1. Files owned

| File | Build tag | Contents |
|---|---|---|
| `internal/usage/errors.go` | none (tag-free) | `ErrUsageCompiledOut` sentinel |
| `internal/usage/toggle/toggle_usage.go` | `//go:build !nousage` | `ResolveUsageEnabled`, `Compiled = true`, reason constants |
| `internal/usage/toggle/toggle_nousage.go` | `//go:build nousage` | stub `ResolveUsageEnabled` → `(false, "compiled_out")`, `Compiled = false` |
| `internal/usage/disabled.go` | `//go:build nousage` | usage-root stub surface (SPEC D5/D6) |
| `internal/usage/credential/disabled.go` | `//go:build nousage` | minimal presence stub: `Warning` type |
| `internal/usage/cache/disabled.go` | `//go:build nousage` | `CacheDir()` stub → `ErrUsageCompiledOut` |
| `internal/usage/fetch/disabled.go` | `//go:build nousage` | `FetchAll` + `Options` (same signature as F14), returns `ErrUsageCompiledOut` |
| `internal/pick/degraded.go` | none | `DegradedCandidates`, `UsageState` |
| `internal/pick/degraded_test.go` | none | tests for degraded assembly |

Real counterparts (F11/F12/F13/F14) carry `//go:build !nousage` per R1/R2; under nousage the files above are the ONLY files in `internal/usage/{root,credential,cache,fetch}`.

## 2. Exported surface — `internal/usage/toggle`

```go
package toggle // import path: github.com/WD-Mitchell/which-model/internal/usage/toggle

// Reason constants — MUST equal the canonical usage_disabled_reason values
// (specs/global/CONTRACTS.md §6).
const (
	ReasonFlag               = "flag"                 // --no-usage (L0)
	ReasonConfig             = "config"               // [usage] enabled = false (L1)
	ReasonCompiledOut        = "compiled_out"         // -tags nousage (L2)
	ReasonNoProvidersEnabled = "no_providers_enabled" // [usage] auto/true with zero enabled providers
)

// Compiled reports whether the usage subsystem is linked into this binary.
// true in the default build, false under -tags nousage. Same value for the
// whole process (build-time constant; never toggled at runtime).
const Compiled bool = true // !nousage file; toggle_nousage.go: const Compiled bool = false

// ResolveUsageEnabled resolves the three toggle levels (docs/plan/README.md §6,
// docs/plan/annex-d-cli-reference.md §3.4) with precedence
// compiled_out > flag > config.
//
// Returns (enabled, disabledReason). enabled == false ⇒ reason is one of the
// four constants; enabled == true ⇒ reason == "".
//
// Strict rule (SPEC §2.1 step 4): when the result is
// (false, ReasonNoProvidersEnabled) AND cfg.Usage.Enabled == config.UsageTrue,
// the caller (command layer) MUST exit 2 with a message naming
// "[usage] enabled" — it must NOT degrade.
//
// The stub build (nousage) ignores both arguments and always returns
// (false, ReasonCompiledOut) — L2 cannot be re-enabled at runtime.
func ResolveUsageEnabled(flagNoUsage bool, cfg *config.Config) (bool, string)
```

`cfg` is F01's verbatim `*config.Config`: `cfg.Usage.Enabled` (type `config.UsageEnabled`, constants `UsageAuto`/`UsageTrue`/`UsageFalse`) and `cfg.Providers map[string]config.ProviderConfig` with `.Enabled bool` (default-deny: unlisted providers count as disabled). The toggle package imports `internal/config` only (`specs/global/CONTRACTS.md` §8: `internal/usage/*` MAY import `internal/config`).

## 3. Exported surface — `internal/usage` root (nousage stub file)

Real entry points are F11's (tag-free canonical types in `types.go`; `!nousage` descriptor/registry/credential files). Under nousage, `internal/usage/disabled.go` provides the annex §1a.2 shape with the F21 trims (no `Compiled` — decision D8; stub-owned `Descriptor`/`Options` — R1):

```go
package usage

import "context"

// Descriptor is a nousage-only minimal stub shape. F11's real Descriptor lives
// in descriptor.go (//go:build !nousage); nothing dereferences this stub's
// fields under nousage, so only the identity fields exist. MUST NOT be declared
// if F11 ever makes Descriptor tag-free (R1 guards types.go).
type Descriptor struct {
	ID          string
	DisplayName string
}

// Options is the nousage-only minimal stub for the fetch options. F11's real
// Options type lives in a !nousage file; the stub needs only the type identity.
type Options struct{}

// Registry mirrors F11's Registry: returns nil in the compiled-out build —
// the binary contains no provider adapters at all (SPEC §2.2 step 6).
func Registry() []Descriptor

// Lookup mirrors F11's Lookup: always false in the compiled-out build.
func Lookup(id string) (Descriptor, bool)

// Fetch mirrors F11's Fetch (context, []string providers, Options): always the
// sentinel error. Callers compare with errors.Is (SPEC §2.2 step 8).
func Fetch(context.Context, []string, Options) ([]Snapshot, error)

// CacheDir returns the usage cache directory; the compiled-out build has no
// cache, so it returns the sentinel directly (annex §1a.2 form — no
// usage↔cache delegation, decision D6).
func CacheDir() (string, error)
```

`Snapshot` is the canonical tag-free type from `specs/global/CONTRACTS.md` §1; `Context` is stdlib.

## 4. Exported surface — `internal/usage/fetch` (nousage stub file)

Signature-identical to F14's real `internal/usage/fetch/fetch.go` (pinned by SddUsageCore; do not rename fields):

```go
package fetch // internal/usage/fetch; file disabled.go carries //go:build nousage

import (
	"context"
	"time"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

type Options struct {
	Refresh      bool
	Offline      bool
	MaxAge       time.Duration
	ShowIdentity bool
	Enabled      map[string]bool
	Timeout      time.Duration
	MaxParallel  int
}

// FetchAll in the compiled-out build always returns the sentinel error.
func FetchAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []credential.Warning, error)
// → returns (nil, nil, usage.ErrUsageCompiledOut)
```

Requires `credential.Warning` to exist under nousage — provided by the credential stub (§5).

## 5. Exported surface — `internal/usage/credential` (nousage stub file)

Minimal presence stub (SPEC D6; SddUsageCore: nothing else in the credential package is referenced under nousage):

```go
package credential // internal/usage/credential; file disabled.go carries //go:build nousage

type Warning struct{ Message string }
```

The full real surface (Resolvers, `ResolveChain`, keychain, device flow, `ParseExpiry`, `CheckExpired`, `ErrNotFound`, `MaxCLIOutputBytes`) is F12's `internal/usage/credential/credential.go` and is intentionally NOT mirrored.

## 6. Exported surface — `internal/usage/cache` (nousage stub file)

```go
package cache // internal/usage/cache; file disabled.go carries //go:build nousage

import "github.com/WD-Mitchell/which-model/internal/usage"

// CacheDir mirrors F13's CacheDir; the compiled-out build has no cache.
func CacheDir() (string, error) // → ("", usage.ErrUsageCompiledOut)
```

## 7. Exported surface — `internal/pick/degraded.go`

```go
package pick // internal/pick; NO build tag — present in both builds

// UsageState is the resolution result the pick flow consumes and echoes
// (SPEC §2.4 step 14). Constructed by the command layer from
// toggle.ResolveUsageEnabled.
type UsageState struct {
	Enabled        bool   // resolved usage_enabled
	DisabledReason string // usage_disabled_reason; "" when Enabled
}

// DegradedCandidates rewrites candidates for usage-disabled picks: every
// candidate gets Band = "" and BandWeight = 1.0 (so FinalScore == ModelScore);
// all other fields are copied unchanged; the input slice is not modified.
// [bands] and gate_above_used_percent are inert by construction (SPEC §2.4
// step 13). UsageState with Enabled == true is not this function's concern —
// callers apply it only on the degraded path.
func DegradedCandidates(candidates []pick.Candidate) []pick.Candidate
```

## 8. Error codes / sentinels added

```go
// internal/usage/errors.go (tag-free, both builds)
var ErrUsageCompiledOut = errors.New("usage subsystem compiled out (-tags nousage)")
```

No numeric exit codes are added (F21 is not a command layer); the strict-rule exit 2 and `usage_disabled`/`usage_compiled_out` failure codes are F24/F25/F26's per `specs/global/CONTRACTS.md` §1.6.

## 9. Config keys / flags owned

None — F01 owns the `[usage]`/`[providers.*]` schema (F21 only reads it), F26 owns the `--no-usage` flag definition. F21 owns their resolution semantics (SPEC §2.1) and the audit commands in TASKS F21-T7.

## 10. Build matrix

| Build | toggle.Compiled | ResolveUsageEnabled | usage-root stub | credential/cache/fetch stubs | pick degraded |
|---|---|---|---|---|---|
| `go build ./...` | `true` | real resolution (L0/L1) | none (F11 real files) | none (F12/F13/F14 real files) | available (unused) |
| `go build -tags nousage ./...` | `false` | `(false, "compiled_out")` | `disabled.go` active | disabled.go stubs active | available (used for every pick) |

## 11. Cross-feature requirements (consumed contract)

- F26 (pick): imports `github.com/WD-Mitchell/which-model/internal/usage/toggle`, calls `ResolveUsageEnabled(noUsageFlag, cfg)`; enforces the strict rule (exit 2 naming `[usage] enabled`); uses `DegradedCandidates` + `UsageState` when disabled; emits `usage_enabled`/`usage_disabled_reason` per `specs/global/CONTRACTS.md` §6.
- F24 (usage) / F25 (auth): exit 2 with `usage_disabled` naming the disabling key under L0/L1; not registered under L2.
- F11/F12/F13/F14: R1/R2 (`!nousage` tags) — the stub files above only compile correctly if the real files do not leak symbols into the nousage build.
- `internal/catalog/**`: must not import `internal/usage/**` (SPEC §2.3); catalog tests run under both builds.
