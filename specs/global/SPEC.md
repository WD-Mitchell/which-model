---
kind: global-spec
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
binary: which-model
aliases: [wm, wmodel, whichm]
go_version: "1.25"
---

# which-model — Global Specification

## 1. Purpose

`which-model` is a Go CLI that answers: **given this task, which exact model and reasoning effort should I dispatch to, on which provider, given what allowance is left?**

It merges two prototypes:

1. `usage-allowance-checks/` — provider usage/allowance reporting (3 providers, Node)
2. `available-model-data-export/` — model scoring and ranking (Python pipeline + ranker)

The merge artifact is the **route** — `(provider, model_id) → (catalog_model, reasoning)` — which neither prototype has. Without it, usage and scores are two unrelated datasets.

## 2. Architecture layers

```
Layer 0: Foundation    — config, decimal, output, http, security
Layer 1a: Catalog      — csvstore, identity, collectors, scoring, ranking
Layer 1b: Usage        — types, credentials, cache, fetch, provider adapters
Layer 2: Routing       — provider ↔ model join
Layer 3: Selection     — bands, strategies, usage toggle
Layer 4: CLI           — cobra commands
Layer 5: Integration   — agent skills, hooks, publishing
```

**Dependency rule:** `pick → routing → {usage, catalog}`, never upward. `usage` and `catalog` do not know about each other; `routing` is the only place they meet.

## 3. Canonical Go packages

| Package | Layer | Purpose |
|---|---|---|
| `internal/company` | 0 | Protected machine enrollment and independent policy authority |
| `internal/config` | 0 | TOML config, env, flag resolution |
| `internal/decimal` | 0 | `shopspring/decimal` wrappers, `ROUND_HALF_UP` |
| `internal/output` | 0 | JSON/text/schema renderers |
| `internal/httpkit` | 0 | Shared HTTP: retries, redirect rejection, body bounding |
| `internal/security` | 0 | Token validation, bounded file I/O, canary harness |
| `internal/catalog/csvstore` | 1a | Atomic CSV read/write/merge/backup |
| `internal/catalog/identity` | 1a | Model name cleaning, identity keys, effort parsing |
| `internal/catalog/fetch` | 1a | AA v2, models.dev, AA page collectors |
| `internal/catalog/score` | 1a | Normalizer/Aggregator, category composites |
| `internal/pick` | 1a | Profiles, ranking, tier1/tier2 combination |
| `internal/usage` | 1b | Window/Snapshot types, Descriptor, registry |
| `internal/usage/credential` | 1b | File, env, keychain, cookie, CLI resolvers |
| `internal/usage/cache` | 1b | Per-provider TTL cache |
| `internal/usage/fetch` | 1b | Concurrent fan-out, partial failure |
| `internal/usage/provider/<id>` | 1b | One package per provider adapter |
| `internal/routing` | 2 | Route production, provenance, staleness |
| `internal/pick/band` | 3 | Pressure, band evaluation, gating |
| `internal/pick/strategy` | 3 | Six strategies + state file |
| `cmd/which-model` | 4 | Cobra command tree |
| `pkg/whichmodel` | — | Public library surface (future GUI) |

## 4. Build variants

| Tag | Effect |
|---|---|
| (default) | Full binary, all features |
| `nousage` | Usage subsystem compiled out; `internal/usage/**` replaced by stubs returning `ErrUsageCompiledOut` |

## 5. Exit codes (fixed, every command)

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Runtime error |
| 2 | Argument/config error |
| 3 | No viable candidate after filtering |
| 4 | All eligible providers band-gated |
| 5 | Authentication required |

## 6. Security invariants

Inherited from `docs/plan/research/usage-allowance-checks-spec.md` §9. Non-negotiable:

1. Exact HTTPS endpoint allow-lists (no prefix/origin matching)
2. Redirects hard-fail (never followed)
3. Bodies bounded: 1 MiB credentials, 256 KiB responses (checked twice)
4. Opaque-token validation: length-bounded, single-line, no control chars
5. No credential material in any error, log, or output (canary-tested)
6. Permission warnings, never auto-remediation
7. Identity display opt-in (`--show-identity`) only
8. Configured-fallback origins require exact per-invocation trust
9. No background polling unless explicitly started

## 7. Testing strategy

- **TDD:** every task writes tests before implementation
- **Golden files:** `testdata/` fixtures for CSV, JSON response shapes, CLI output
- **Canary tokens:** every credential-touching path tested with a canary that must never appear in output
- **Table-driven:** Go `testing` with subtests, no external test framework
- **Build-matrix CI:** both default and `-tags nousage` on every change

## 8. Plan references

| Document | Path |
|---|---|
| Master plan | `docs/plan/README.md` |
| Provider matrix | `docs/plan/annex-a-provider-matrix.md` |
| Catalog port | `docs/plan/annex-b-catalog-port.md` |
| Agent integration | `docs/plan/annex-c-agent-integration.md` |
| CLI reference | `docs/plan/annex-d-cli-reference.md` |
| Usage checker spec | `docs/plan/research/usage-allowance-checks-spec.md` |
| Pipeline spec | `docs/plan/research/model-data-pipeline-spec.md` |
| Provider survey | `docs/plan/research/codexbar-provider-survey.md` |

## 9. npm fallback distribution (#161)

When the platform optional package is unavailable, postinstall may fetch the version-matched release binary. Decode only `checksums.txt` as UTF-8 and parse sha256sum LF/CRLF records (including the binary marker). Hash and write the binary as unchanged bytes, with executable mode on Unix. Missing, malformed or mismatched checksums and download failures must leave no installed fallback and warn without failing npm installation. Existing optional binaries and `WHICH_MODEL_SKIP_DOWNLOAD=1` perform no fallback requests. Validate with `node --test npm/which-model/install.test.js` and the existing npm smoke test.

## 10. Source checkout portability (#280)

Every tracked path must be portable to a normal Windows checkout. The CI path
check rejects reserved characters/control bytes, invalid Unicode, empty or
relative components, trailing periods/spaces, reserved Windows device names
(including extensions and numbered COM/LPT names), and case-insensitive
file/directory collisions at every path component. The check examines Git's
index with NUL-delimited filenames; it does not inspect untracked local files.

CI must perform an actual Windows source checkout, build the default and
`nousage` CLI variants there, and run each variant's version/help commands.
Linux cross-compilation remains part of release packaging but does not replace
this native checkout/build gate. This requirement concerns the CLI; it does not
expand desktop Windows feature support.

Correction: #280 removes the accidentally tracked `xd:/lsp` editor diagnostic
artifact and adds the missing portability gate. It changes no CLI runtime
behaviour or provider credential policy.

## 11. Company identity boundary (#281)

Company deployments retain the local OS user and provider account identity. No
separate which-model login, credential broker or product RBAC is introduced. The
[company identity decision](../../docs/security/company-identity.md) records the
actors, observed provider grants, same-user bypass limits and required external
controls. Provider identity validation is not company membership validation, and
reading allowance data does not reduce the credential's effective privileges.

The optional administrator profile is follow-on work (#282); it preserves
personal-user defaults and governs operations inside an approved installation.
Endpoint and provider controls remain responsible for enforcement outside that
process. Quota/authentication evidence and audit-write failures are advisory and
do not themselves block otherwise authorised launches (decision for #286). This
decision does not remove native permissions or approved-executable requirements.

Clarification: this section defines the deployment trust boundary; it changes no
credential resolution, runtime quota handling or canonical API contract in #281.
Provider permission and company acceptance are separate evidence requirements.

## 12. Verified CLI release distribution (#288)

Correction to §9: fallback installation additionally requires signed GitHub/Sigstore
SLSA v1 provenance. Checksums alone no longer authorise installation. The trusted
repository is `WD-Mitchell/which-model`, the signer is its
`.github/workflows/npm-release.yml`, and the source ref and full commit must match
the release policy stamped into the npm launcher. GitHub-hosted runners and the
GitHub Actions OIDC issuer are required. GitHub CLI 2.97.0 or later supplies the
cryptographic verifier; an absent/failing verifier leaves no runnable fallback.
The optional npm platform package remains verified through npm registry integrity
and provenance. No company signing service is added.

The build emits a CycloneDX 1.6 SBOM for each CLI binary from its embedded Go
build information, with the artifact digest, linked modules and Go runtime. The
repository-owned generator is versioned with the source; no guessed licences or
transitive dependency graph are emitted. Release provenance covers binaries,
SBOMs, checksum list and release manifest. A signed manifest binds the version,
full source revision/ref and artifact/SBOM digests. Release jobs independently
verify the downloaded artifacts before publishing or packaging.

Release tests/builds use pinned Go 1.26.8 (the scanner requires Go 1.26+).
Release evidence records `govulncheck` v1.8.0 binary scans and exact npm package
provenance assessment. Scanner errors/findings stop release publication; this is
a release-integrity gate, not a runtime quota or audit launch gate. Manual
verification-only runs may produce signed candidate artifacts and evidence but
never create tags, publish GitHub releases or publish npm packages.

Installers stage candidate bytes outside the launcher path (0600 on POSIX;
Windows uses the package directory ACL), never execute the candidate, verify
checksum, source identity and signature, then atomically expose the executable. A matching verification receipt and
current digest are required when the npm launcher uses a local fallback; stale
or unverified leftovers are refused.
Failures clean staging and preserve the existing best-effort npm-install exit
behaviour with an actionable warning. Offline verification requires an approved
source revision/ref, the bundle and independently obtained Sigstore trust roots;
unavailable evidence is not a reason to skip verification.

## 13. Restricted offline assets (#290)

In addition to the five full CLI artifacts, the same verified release emits five
`which-model-score-only-<os>-<arch>[.exe]` artifacts. F21 SPEC §7 governs their
restricted command boundary. They embed the reviewed repository catalog and
built-in ranking profiles and carry no runtime catalog-update mechanism.
The full npm packages remain the full product; this pilot adds separate GitHub
release assets, not a change to npm's default executable.

The signed `which-model-score-only-capabilities.json` is digest-pinned in the
release manifest. Each restricted SBOM also identifies the embedded catalog and
profiles by SHA-256. Verification checks these bindings before installation.
Catalog source/licensing approval and company acceptance are release-owner
decisions distinct from artifact integrity. The build pipeline is capable of
producing candidates; a passing candidate is not permission to distribute them.


## 14. Optional administrator-managed policy (#282)

Enrolled installations use the [F01 managed-policy contract](../features/F01-config/MANAGED-POLICY.md).
Machine policy is independent of user/project/environment/flag configuration and is
reloaded at sensitive operation boundaries. Required enrollment with missing or
untrusted policy refuses restricted operations. Personal installations retain their
existing behavior; the separate offline artifact still has no policy/config I/O.
This foundation records retention and executable approvals; #283–#285 and #287
complete secure-store, persistence and verified-execution consumers. Quota/audit
failures remain advisory under the user decision for #286.


## 14. Product maturity and release classification (#289)

The product remains pre-release until a maintainer explicitly approves stable
promotion against [the readiness record](../../docs/releases/readiness.md). A
numeric tag alone does not change maturity. All newly published GitHub releases
are marked pre-release and are not promoted to GitHub Latest. npm retains its
existing latest/numeric and beta/suffixed distribution channels; package
descriptions and installation guidance identify pre-release product maturity.
Historical releases/dist-tags are not rewritten by this change. Stable promotion
requires a reviewed spec, documentation and metadata change together.

The existing security-fix focus is main and the newest published release, with
older users potentially required to upgrade. No new SLA/support contract or
response/remediation deadline is introduced. The readiness checklist requires
exact-revision/artifact evidence and records unresolved release decisions; company
acceptance and human maintainer sign-off remain separate. CI or PR creation is
not that sign-off. The module floor follows go.mod; release tooling remains as
pinned in §12.

### Deviations and corrections

#289 supersedes the release workflow's former assumption that every plain numeric
tag denotes stable maturity. The existing README pre-release statement remains
authoritative until explicit maintainer promotion. The `go_version` frontmatter
is corrected from the stale 1.23 value to the existing go.mod floor, 1.25; this
does not raise the module requirement.
