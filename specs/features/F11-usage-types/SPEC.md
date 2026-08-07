---
kind: feature-spec
version: "1.0"
feature: F11-usage-types
project: which-model
---

# F11 — Usage Types: canonical types, Descriptor, registry

## Purpose

F11 is the type layer of the usage subsystem (`internal/usage`). It holds the canonical usage types from `specs/global/CONTRACTS.md` §1 (Unit, Source, Kind, Window, Snapshot, Failure) **verbatim** — F11's `types.go` IS the Go home of those types, so no other feature defines them — plus the descriptor layer the 66-provider matrix needs: `Descriptor`, `AuthSource`, `WindowSpec`, `Credential`, `FailureError`, and the compile-time registry (`Register`/`Get`/`All`/`IDs`).

Everything downstream (F12 credentials, F13 cache, F14 fetch, F15–F17 provider adapters, F18 routing, F21 usage-toggle) imports this surface and nothing else from `internal/usage`.

## Behaviour

1. **Canonical types, verbatim.** `internal/usage/types.go` defines exactly the types in `specs/global/CONTRACTS.md` §1.1–§1.6 (`Unit`, `Source`, `Kind`, `Window`, `Snapshot`, `Failure`), including every JSON tag. No field may be added, renamed, or given a provider-specific variant (global CONTRACTS preamble; source: `docs/plan/README.md` §3.2, `docs/plan/annex-a-provider-matrix.md` §5). F11 also adds `func (k Kind) String() string` with the exact strings from annex-a §5 (`subscription`, `api_key_billing`, `gateway`, `local_tool`, `unknown`).
2. **Credential.** `internal/usage/credential.go` defines `Credential{Token, Extra, Source, Mode}` exactly as annex-a §5, plus `func (c Credential) String() string` returning a redacted rendering (`token=<redacted>`) so `%s`/`%v` on a credential can never leak the token (global SPEC §6 invariant 5; canary-tested in F11-T2). `Source` is an `AuthKind`, not `usage.Source` — it records which AuthSource kind produced the token (annex-a §5).
3. **AuthKind enum.** The 13-kind enum from annex-a §5 (`AuthEnvVar` … `AuthGRPCWebToken`) with `func (k AuthKind) String() string`. F12 implements resolvers for env/file/keychain/cookie/CLI/device-flow kinds only; the remaining kinds are declared here so provider features (F15–F17) can reference them without redefining.
4. **FailureError.** `internal/usage/credential.go` defines the coded-error wrapper `FailureError{Failure Failure}` with `Error()` rendering `"<code>: <message>"`, constructor `NewFailureError(code, message) error`, and matcher `AsFailure(err) (Failure, bool)`. Resolvers (F12), the fetch layer (F14), and providers (F15+) all carry Failure codes through this single wrapper; the message is sanitised and never contains credential material (global SPEC §6 invariant 5).
5. **Descriptor.** `internal/usage/descriptor.go` defines `Descriptor` with fields from annex-a §5: `ID`, `DisplayName`, `Kind`, `Tier`, `Auth []AuthSource`, `Windows []WindowSpec`, `Timeout`, `CacheTTL`, `Fetch FetchFunc`, plus `LastVerified time.Time` (zero value = never verified; populated by F25 auth flows). `Timeout` and `CacheTTL` are consumed by F13 (TTL) and F14 (per-provider timeout) directly from the Descriptor. `FetchFunc` is `func(ctx context.Context, cred Credential, client *http.Client) (Snapshot, error)` (annex-a §5).
6. **AuthSource.** `internal/usage/descriptor.go` defines `AuthSource` with the annex-a §5 fields (`Kind`, `EnvVar`, `FilePaths`, `JSONPath`, `Keychain *KeychainSpec`, `Cookie *CookieSpec`, `Shell *ShellSpec`, `RPC *RPCSpec`, `OAuth *OAuthSpec`, `Validate func(ctx, Credential, *http.Client) error`) plus three declarative extensions: `ExtraPaths map[string]string` (Extra field name → dotted JSON path inside the credential file, e.g. `"account_id" → "tokens.account_id"`), `ExpiryPath string` (dotted JSON path to the token's expiry epoch/date; F12 checks it), and `EnvExtra []string` (extra env var names → `Credential.Extra`). The chain semantics — walk in order, first candidate that resolves AND passes `Validate` wins — are F12's `ResolveChain`; F11 only declares the shape.
7. **WindowSpec.** `internal/usage/descriptor.go` defines `WindowSpec{ID, Label, Unit, Optional, ModelScope}` — annex-a §5 plus `ModelScope []string`, the descriptor-time declaration of which models a window's quota applies to. F18 routing's `BindWindowIDs` (annex-b §7.3) matches routes against `ModelScope`; runtime values land in canonical `Window.ModelScope` (global CONTRACTS §1.4).
8. **Registry.** `internal/usage/registry.go` implements the database/sql-style registry (annex-a §5): `Register(d Descriptor)` called from each `internal/usage/provider/<id>/<id>.go` `init()`; **panics** on a duplicate ID (a programming error caught at binary startup, never a runtime condition — decision D1). `Get(id) (Descriptor, error)` returns a typed `*UnknownProviderError` for unknown IDs (decision D2). `All() []Descriptor` and `IDs() []string` return results **sorted lexicographically by provider ID** (decision D3), independent of registration order. No mutex: registration happens only in `init()` (and in tests, before parallel use — documented in F14-T2).
9. **Provider package layout rule.** Every provider adapter lives in exactly one package `internal/usage/provider/<id>/` with one `init()` calling `usage.Register(usage.Descriptor{...})` (annex-a §5). `cmd/which-model/main.go` blank-imports every provider package — the only place the provider list is enumerated for linking (annex-a §5). A provider that is not imported does not exist in the binary.
10. **Build tags.** `internal/usage/types.go` carries **no** build tag (the F21 `nousage` stub file references `Snapshot`; annex-a §1a.2). `credential.go`, `descriptor.go`, and `registry.go` carry `//go:build !nousage` (they touch credentials and the registry; annex-a §1a.2). The inverse-tag stub file `internal/usage/disabled.go` is owned by F21-usage-toggle, which mirrors this feature's exported surface.

## Error behaviour

- `Register` with a duplicate `ID` → panic with message `usage: duplicate provider id "<id>"` (annex-a §5).
- `Get` with an unknown ID → `*UnknownProviderError` (implements `error`; use `errors.As`); it wraps no other error.
- `FailureError.Error()` renders `"<code>: <message>"`; `AsFailure` returns `ok=false` for non-`FailureError` errors.
- No other error paths exist in F11 (pure types + registry).

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Duplicate registration | `panic` | Annex-a §5: a duplicate ID is a programming error caught at binary-startup time (database/sql pattern), never something a running command can trigger. Assignment: "decide + record". |
| D2 | `Get` on unknown ID | typed `*UnknownProviderError` | Assignment requires a typed error for Get-unknown; `errors.As`-friendly. Annex-a §5's bool-returning `Lookup` is not exported. |
| D3 | `All()`/`IDs()` ordering | sorted lexicographically by ID | Assignment requires sorted IDs; deterministic output for golden tests regardless of init order. Overrides annex-a §5's registration-order preservation. |
| D4 | Descriptor field names | `DisplayName`, `Auth` (annex-a §5) | The assignment summary says "Name"/"AuthSources"; annex-a §5 is the exact declaration providers (F15–F17) cite. |
| D5 | `LastVerified time.Time` | added to Descriptor | Required by assignment; zero = never verified; populated by F25 auth verification flows. No F11–F14 task sets it. |
| D6 | `WindowSpec.ModelScope` | added | Assignment's "model-scope matcher"; F18's `BindWindowIDs` (annex-b §7.3) and canonical `Window.ModelScope` (global CONTRACTS §1.4) require the descriptor-time signal. |
| D7 | AuthSource extensions | `ExtraPaths`, `ExpiryPath`, `EnvExtra` | Declarative credential chains for F12/F15–F17: Codex's `account_id`, Claude's `expires_at`, OpenAI Platform's `OPENAI_PROJECT_ID` all become AuthSource config instead of provider-local loaders (per `docs/plan/research/usage-allowance-checks-spec.md` §2.1–§2.3). |
| D8 | Credential lives in F11 | `usage.Credential`; F12 re-exports `type Credential = usage.Credential` | Annex-a §5 puts it in package `usage`; `FetchFunc` references it, so defining it in F12 would create an F11↔F12 import cycle. |
| D9 | `types.go` build tag | none | F21's `nousage` stub (`internal/usage/disabled.go`) returns `[]Snapshot`, so `Snapshot` must exist in both builds (annex-a §1a.2). Other F11 files carry `//go:build !nousage`. |
| D10 | Registry synchronization | none (lock-free) | Registration is `init()`-time only (annex-a §5). Tests register uniquely-named fakes before parallel use (F14-T2 documents this). |

## Out of scope

- Credential resolution and the chain walker (`ResolveChain`) — F12.
- On-disk cache — F13.
- Fetch orchestration (`FetchAll`) — F14.
- Provider adapters (`internal/usage/provider/<id>/*`) — F15–F17.
- The `nousage` stub file `internal/usage/disabled.go` and package-level stubs for `internal/usage/credential|cache|fetch` — F21-usage-toggle.
- JSON serialization of Descriptor/AuthSource (no consumer requires it; provider registry is compile-time, not a data file).
