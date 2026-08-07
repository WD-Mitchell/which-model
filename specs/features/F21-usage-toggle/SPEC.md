---
kind: feature-spec
version: "1.0"
feature: F21-usage-toggle
project: which-model
---

# F21 — Usage Toggle: SPEC

## 1. Purpose

F21 implements the usage toggle — the three independent levels at which "usage awareness" is turned off (`docs/plan/README.md` §6): per-invocation `--no-usage` (L0), configured `[usage] enabled = false` (L1), and the compile-time `-tags nousage` build (L2). It owns the resolution logic (`internal/usage/toggle`), the `nousage` stub surface that keeps the whole module compiling and credential-free when usage is compiled out, and the degraded-mode candidate assembly that turns `which-model pick` into a pure score ranker when usage is off (`internal/pick/degraded.go`).

## 2. Behaviour

### 2.1 Resolution — `ResolveUsageEnabled`

1. The resolution entry point is `toggle.ResolveUsageEnabled(flagNoUsage bool, cfg *config.Config) (bool, string)` in `internal/usage/toggle` (import path `github.com/WD-Mitchell/which-model/internal/usage/toggle`). The second return is the `usage_disabled_reason` value (`specs/global/CONTRACTS.md` §6, `docs/plan/annex-c-agent-integration.md` §4.6): `"flag" | "config" | "compiled_out" | "no_providers_enabled"`; it is `""` exactly when usage is enabled.
2. Precedence, highest first (`docs/plan/annex-d-cli-reference.md` §3.4): compiled out (L2) beats everything — a `nousage` build always resolves `(false, "compiled_out")` regardless of flags or config; then `--no-usage` (L0, flag beats config); then `[usage] enabled` (L1).
3. Three-state config (`docs/plan/README.md` §6.1, `specs/global/CONTRACTS.md` §5.1): `false` → `(false, "config")`; `"auto"` (default) → enabled iff at least one `[providers.<id>]` has `enabled = true`, else `(false, "no_providers_enabled")`; `true` → `(true, "")` when at least one provider is enabled.
4. **Strict rule** (`docs/plan/README.md` §6.1): `[usage] enabled = true` with zero enabled providers resolves to `(false, "no_providers_enabled")`, and every consuming command (F26 `pick`, F24 `usage`, F25 `auth`) MUST treat that pairing as a configuration error and exit `2` with a message naming the key — it must NOT silently degrade. The resolution function reports the pair; the caller enforces the exit (`docs/plan/annex-d-cli-reference.md` §3.4: all toggle-related refusals use exit 2).
5. Provider enablement is default-deny (`docs/plan/README.md` §6.2, `docs/plan/annex-a-provider-matrix.md` §1a.1): unlisted providers are disabled; the count for `auto`/`true` is the number of entries in `cfg.Providers` with `Enabled == true`.

### 2.2 The `nousage` build — guarantee over promise

6. Under `go build -tags nousage`, the usage subsystem is **not linked**: no credential resolver, cookie reader, keychain call, or provider endpoint constant exists in the binary; `which-model usage` and `which-model auth` are not registered in the command tree (`docs/plan/README.md` §6 L2, `docs/plan/annex-d-cli-reference.md` §4.7). The registry is empty by construction — no `init()` self-registration runs — not filtered at runtime.
7. Every real usage file that touches credentials, endpoints, or the registry carries `//go:build !nousage` (F11/F12/F13/F14 files; cross-feature requirement R2). F21 provides the inverse-tag sibling stubs (decision D5) so the module compiles and every stub entry point returns the sentinel `usage.ErrUsageCompiledOut` (`docs/plan/annex-a-provider-matrix.md` §1a.2).
8. `ErrUsageCompiledOut` is a sentinel defined once in a tag-free file (`internal/usage/errors.go`, decision D4) so both builds can compare with `errors.Is` — the annex's own §1a.2 says "callers compare with errors.Is", which requires the symbol in the default build too.
9. The stub surface (decisions D5, D6) is the annex §1a.2 shape, minus the pieces F21 deliberately moves or trims: `internal/usage/disabled.go` (nousage) declares `Registry()` → `nil`, `Lookup` → `false`, `Fetch` → `ErrUsageCompiledOut`, `CacheDir` → `ErrUsageCompiledOut`, plus the stub-only types `Descriptor{ID, DisplayName string}` and `Options{}` — F11's real `Descriptor`/`Options` live in `!nousage` files, so the stub declares its own minimal shapes (nothing dereferences their fields under nousage; it MUST NOT redeclare any tag-free identifier, see R1); the root stub carries **no** `Compiled` const — that is the toggle package's (decision D8); `internal/usage/credential/disabled.go` declares the `Warning` type needed by the fetch signature (minimal presence stub — nothing else references credential under nousage); `internal/usage/cache/disabled.go` declares `CacheDir()` → `ErrUsageCompiledOut` (direct sentinel, no cross-import); `internal/usage/fetch/disabled.go` declares `FetchAll` + `Options` with the SAME signature as F14's real entry, returning `ErrUsageCompiledOut`.
10. The toggle package itself is tag-split (decision D5): `toggle_usage.go` (`//go:build !nousage`) holds the real `ResolveUsageEnabled` and `const Compiled = true`; `toggle_nousage.go` (`//go:build nousage`) holds `const Compiled = false` and a stub `ResolveUsageEnabled` that ignores both arguments and returns `(false, "compiled_out")` — L2 cannot be re-enabled at runtime by any flag, env var, or config value (`docs/plan/annex-d-cli-reference.md` §3.4).
11. Auditability (`docs/plan/annex-a-provider-matrix.md` §1a.3): the strings-scan audit (`strings which-model | grep -c 'chatgpt.com/backend-api\|api.anthropic.com\|copilot_internal'` → 0) and the module audit (`go version -m` shows no credential-store libraries) are acceptance criteria of the final task, and CI MUST build and test both variants on every change.

### 2.3 Catalog independence

12. `internal/catalog/**` has no dependency on `internal/usage/**` in either direction (`docs/plan/annex-b-catalog-port.md` §0): the catalog packages compile and pass their full test suite under `-tags nousage`, and every `which-model catalog *` command is unaffected by all three toggle levels. Enforced by the build-matrix task and CI job.

### 2.4 Degraded pick assembly

13. When usage is off at any level, `which-model pick` degrades to exactly the `rank_models.py` behaviour — pure profile-based score ranking (`docs/plan/README.md` §6.3): `BandWeight = 1.0` for every candidate, `Band` empty, `[bands]` and `gate_above_used_percent` inert, unknown-usage warnings suppressed, `ProviderWeight` still applies, routing survives (unauthenticated sources only).
14. F21 owns the assembly seam `internal/pick/degraded.go` (decision D7): `DegradedCandidates([]pick.Candidate) []pick.Candidate` returns a copy with `Band = ""` and `BandWeight = 1.0` for every candidate (so `FinalScore == ModelScore`), and the `UsageState` type carries the resolution result (`Enabled`, `DisabledReason`) that the pick flow echoes into the output envelope (`usage_enabled: false`, `usage_disabled_reason`; `docs/plan/annex-c-agent-integration.md` §4.6). The JSON output omits `band`/`band_weight`/`pressure` entirely — never `null`, never a fabricated `"low"` (`docs/plan/annex-c-agent-integration.md` §4.6).
15. A degraded pick still returns a model and still carries a provider name — routing survives. `usage_enabled` is the only evidence of usage-awareness (`docs/plan/README.md` §6.3; F26 output contract).
16. Degraded mode is strictly more deterministic than the enabled path — a pure function of (scores CSV, config) — and MUST be byte-reproducible from the same scores CSV (`docs/plan/annex-d-cli-reference.md` §3.3).

## 3. Error behaviour

| Error | Message | Where |
|---|---|---|
| `usage.ErrUsageCompiledOut` (sentinel, tag-free) | `usage subsystem compiled out (-tags nousage)` | every nousage stub entry point; `errors.Is` comparable from either build |
| Strict-rule violation (`[usage] enabled = true`, zero providers) | exit 2 at the command layer, message naming `[usage] enabled` — enforced by F24/F25/F26 per SPEC §2.1 step 4, not by the toggle package | F26/F24/F25 |

`Failure.Code` values `usage_disabled` / `usage_compiled_out` (exit 2) are global (`specs/global/CONTRACTS.md` §1.6) and used by the command layer, not by the toggle package.

## 4. Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Reason constants | Plain string constants in `toggle`: `ReasonFlag = "flag"`, `ReasonConfig = "config"`, `ReasonCompiledOut = "compiled_out"`, `ReasonNoProvidersEnabled = "no_providers_enabled"` | Must match the canonical `usage_disabled_reason` values (`specs/global/CONTRACTS.md` §6, annex-c §4.6) exactly; constants prevent typos without redefining the global type |
| D2 | Resolution input | `cfg *config.Config` directly (F01 verbatim: `config.Config.Usage.Enabled` of type `config.UsageEnabled`, `config.Config.Providers map[string]config.ProviderConfig` with `.Enabled bool`) | F01 pinned the shape; a subset adapter would add a translation layer for no gain |
| D3 | Strict-rule representation | `(false, "no_providers_enabled")` + documented caller obligation to exit 2 when the configured value is `true` | The assignment fixes the signature `(bool, reason string)`; the caller already holds `cfg` and can distinguish `auto` from `true` |
| D4 | `ErrUsageCompiledOut` placement | Tag-free `internal/usage/errors.go` | Annex-a §1a.2 shows the sentinel inside the stub file, but "callers compare with errors.Is" requires the symbol in the default build too; one tag-free definition is the build-tag pattern's clean form |
| D5 | Stub file names | `internal/usage/disabled.go`, `internal/usage/credential/disabled.go`, `internal/usage/cache/disabled.go`, `internal/usage/fetch/disabled.go`, `internal/usage/toggle/toggle_usage.go`, `internal/usage/toggle/toggle_nousage.go` | Assignment requirement (`toggle_usage.go`/`toggle_nousage.go`) plus annex-a §1a.2's `disabled.go` naming; the per-package siblings mirror F11/F12/F13/F14's `!nousage` files |
| D6 | Stub depth | Root package mirrors the annex §1a.2 surface minus `Compiled`; `credential` is a minimal presence stub (only the `Warning` type the fetch signature needs); `cache` mirrors `CacheDir()` returning the sentinel directly (the annex stub form — no usage↔cache delegation, which would pair the two packages' stubs for zero observable gain); `fetch` mirrors `FetchAll` + `Options` exactly | SddUsageCore: under nousage nothing imports the real symbols, so full mirrors would be dead code; the fetch signature must match F14's because it is the entry point consumers compile against |
| D7 | Degraded assembly location | `internal/pick/degraded.go` owned by F21 (files live in `internal/pick` because candidate assembly happens there and `internal/usage` must not import `internal/pick`, `specs/global/CONTRACTS.md` §8) | F19 confirmed its band package does not claim degraded assembly; F26 confirmed it claims zero `internal/pick` files |
| D8 | `Compiled` placement | `toggle_usage.go`: `const Compiled = true`; `toggle_nousage.go`: `const Compiled = false`. The usage-root stub carries NO `Compiled` (deviation from annex §1a.2, which shows `const Compiled = false` inside the stub — SddUsageCore confirmed the toggle package is the variant marker) | The toggle IS the compiled-in/out switch; placing it here avoids a second tag-split pair at the usage root and every consumer of the variant already imports toggle |
| D9 | `--no-usage` short-circuit | Resolution precedes any credential access; a disabled invocation never touches a credential file, keychain item, cookie DB, or provider CLI | Master plan §6.2: `--no-usage` MUST short-circuit before credential resolution so the disabled path cannot trigger a macOS Keychain authorisation prompt |

## 5. Cross-feature requirements

- **R1 (F11):** `internal/usage/types.go` (Window/Snapshot/Unit/Source/Kind, canonical per `specs/global/CONTRACTS.md` §1) stays tag-free; `descriptor.go`, `registry.go`, `credential.go` carry `//go:build !nousage` (SddUsageCore-pinned). F21's root stub declares its own minimal `Descriptor{ID, DisplayName string}` and `Options{}` and MUST NOT redeclare any identifier that `types.go` declares in both builds (`Snapshot`, `Failure`, `Window`, `Unit`, `Source`, `Kind`, `Unit*` constants).
- **R2 (F12, F13, F14):** every file in `internal/usage/credential`, `internal/usage/cache`, `internal/usage/fetch` carries `//go:build !nousage` (F14 confirmed; F12/F13 same rule), so F21's stub siblings are the only files in those packages under nousage.
- **R3 (F26):** `toggle.ResolveUsageEnabled` + the strict rule (SPEC §2.1 step 4) are F26's consumed import; F26 maps the disable levels to `usage_enabled: false` / `usage_disabled_reason` in the envelope and registers no `usage`/`auth` commands under nousage (F22's command tree).
- **R4 (F24/F25):** `which-model usage`/`auth` exit 2 with `usage_disabled` naming the disabling key under L0/L1; not registered under L2.
- **R5 (CI):** build and test both variants (`go build`, `go build -tags nousage`) on every change, plus the strings-scan audit (`docs/plan/annex-a-provider-matrix.md` §1a.3, `docs/plan/annex-d-cli-reference.md` §4.7).

## 6. Out of scope

- The usage fetch fan-out, cache, credential resolvers, and provider adapters themselves — F11/F12/F13/F14/F15-F17.
- The `usage`/`auth`/`pick` commands and the output envelope rendering — F24/F25/F26/F03.
- Band evaluation and gating — F19 (`internal/pick/band`).
- The six pick strategies — F20 (`internal/pick/strategy`); F21 supplies the degraded candidates and the resolution they consume.
- Config file parsing and validation — F01 (owns `[usage]`/`[providers.*]` typing; resolution semantics are F21's).
- `--no-usage` flag definition — F26; F21 owns its semantics (L0).
