# Approved CodexBar delegation (#287)

The optional company profile disables CodexBar until an administrator approves a
native image and its configuration. Personal discovery, environment and source
behavior remain unchanged. Native collection remains separately permitted.

## Approval and ordering

Each `codexbar_installations` entry has absolute canonical `path`, lowercase
SHA-256 `sha256`, and required `config: {path, sha256}`. The administrator protects
both files and their ancestors using the same OS ownership/ACL checks as #285.
The configuration is a reviewed input: upstream CodexBar config can select
sources/endpoints, store credentials, and enable external executable hooks.
A binary-only entry is insufficient and invalid. The list is bounded to 16 entries.

Configuration validation checks for enrollment/approval metadata. A fresh cache or
offline read does not execute or disclose credentials. After a cache miss, verify
an approved image/config pair before resolving any delegated credential, then
verify again immediately before invocation. Use the first pair that passes the
OS protection, identity and digest checks; never consult PATH, CODEXBAR_BIN,
working-directory configuration or user environment for installation selection.
Changed/redirected/writable/missing files are refused until administrator repair or
reapproval. Metadata/version output is never proof of identity.

Provider discovery returns the administrator's allowed provider IDs without running
CodexBar `--help`; cached personal discovery cannot grant company providers. Reject
aggregate provider selectors (`all`, `both`) at company invocation boundaries.

## Process contract

Invoke the approved native image directly, in its protected containing directory:
`usage --provider <single allowed id> --format json --json-only --no-color`, plus an
explicit supported source when requested. No shell or extra project-supplied argv.
Use #285's OS-derived environment plus exactly `CODEXBAR_CONFIG=<approved config>`.
Drop all inherited tokens, proxies, executable/config overrides and startup hooks.

The only delegated credential environment key accepted by which-model is
`ANTIGRAVITY_OAUTH_CREDENTIALS_JSON`, for Antigravity with auto/OAuth source. It is
resolved from the separately permitted which-model OS credential store after
preflight and reduced to the canonical Antigravity Credentials fields: access/refresh
token, expiry, client ID/secret and optional ID token/email for authentication and
account routing. Unknown JSON fields are discarded. It is bounded to 64 KiB and
is never placed in argv or a file by which-model.
Other providers receive no which-model credential environment. CodexBar accesses
its own reviewed authentication/configuration and stores under the OS user's identity.
This approval is an explicit delegation boundary; which-model native credential
source and secure-store guarantees do not govern CodexBar's own reads or storage.

Company output is at most 1 MiB; stderr is discarded. The existing 30-second adapter
ceiling and earlier caller deadline remain, with a one-second wait bound for pipes.
Timeout/cancellation/output/parse/process errors have fixed messages. Require the
requested provider; never relabel a different provider's single result. An explicit
source must be compatible with normalized returned provenance as defined below. A nonzero exit cannot produce a
successful snapshot. Duplicate provider results and unknown source labels are
refused. Invalid/future provider timestamps stay stale. Provider error text is replaced with fixed code-based text.
Cache writes, freshness overrides, forced-source cache eligibility and offline
reads retain F13/F14 behavior and #284 minimization/retention.

## Source and cache correction (PR #314 review)

Decision: the requester approved fixes for both review findings on PR #314.
Provider labels describe the strategy actually used, which can differ from the
upstream source selector. The company adapter accepts the canonical live labels
`oauth`, `api`, `web`, `cli`, and `local`, plus these reviewed provider-specific
aliases (case-insensitive labels; provider IDs remain exact):

| Provider | Returned labels | Canonical source | Compatible explicit selector |
|---|---|---|---|
| Antigravity | `app`, `ide` | `local` | `cli` |
| Codex | `pat` | `api` | `api` |
| Codex | `openai-web` | `web` | `web` |
| Codex | `codex-cli` | `cli` | `cli` |
| Claude | `claude`, `claude-cli` | `cli` | `cli` |
| Claude | `admin-api` | `api` | `api` |
| Windsurf | `windsurf-web` | `web` | `web` |

Antigravity and Windsurf's `cli` selector includes local probes. Preserve their
`local` provenance for both live results and cache eligibility; do not relabel it
as a CLI execution. Otherwise explicit selectors must equal the canonical source.
Auto accepts any recognized source. An alias cannot be borrowed from another
provider, and unlisted labels (including upstream `offline`) remain refused.
Personal normalization and native credential-source filtering are unchanged.
This corrects the earlier global alias list and strict selector equality using the
pinned upstream Antigravity, Codex, Claude, and Windsurf provider descriptors and
CLI result serialization.

Company cache eligibility combines the original producer `Snapshot.Stale` flag
with the outer recording-time TTL. Missing, invalid, or future provider timestamps
marked stale by the adapter remain stale through persistence and offline or
`--source cache` reads, even after increasing `--max-age` or the wall clock passing
a formerly future timestamp. Online reads refetch producer-stale observations;
they do not label them current using the cache recording time. A successful new
observation may replace them. Both native and delegated consumers use this shared
company-cache rule. Retention continues to use the original application recording
time, and personal cache semantics are unchanged.

Pinned regressions: `TestCompanyCodexBarProviderSourceLabels` exercises auto and
forced sources against actual labels, `TestCompanyCodexBarSourceAliasesStayProviderBound`
rejects borrowed/unrecognized labels, `TestCompanyCodexBarLocalCacheMatchesCLIMode`
checks cached local provenance, and `TestCompanyCacheProducerStaleness` checks
current/missing/future/producer-stale observations under multiple TTLs.
`TestNativeCompanyCodexBarApprovalCache` runs the approved child through FetchAll
and the company cache on the existing three-platform native CI fixture: local
app provenance, invalid timestamp, offline/cache-only no-child reads, and online
refetch. The fixture carries only synthetic quota data.

## Limits and evidence

The administrator reviews the pinned executable, dependent libraries/helpers,
configuration, endpoints and update procedure. Approval is not a process sandbox;
CodexBar's own hooks, keychain/browser access, files and network remain subject to
its implementation and endpoint controls. Pinning its configuration prevents user
config replacement from changing that reviewed input; it does not analyze every
upstream setting or provider implementation.

which-model verifies approval on macOS, Windows and Linux. The reviewed upstream
revision publishes macOS/Linux CLI distributions; native Windows approval tests
use a synthetic executable and do not assert an upstream Windows package exists.
No provider login, live allowance or real developer-machine enrollment is required
for the tests.

Pinned tests cover denied/default/no discovery or credential exposure, approved
argv/environment, changed image/config, redirects, PATH/CODEXBAR_BIN shadowing,
unrelated environment canaries, provider/source mismatch, bad exit, bounded output,
timeout and cache-only no-execution behavior. Native CI protects an actual test
image/config on all three operating systems and invokes that image through the
production boundary.

Upstream evidence reviewed: CodexBar revision
`9f4f544a5bf81da276fe94176aa1165c9e296b4b`, especially `docs/configuration.md`,
`docs/cli.md`, `docs/providers.md` and `docs/antigravity.md`. Reapproval must review
the installed revision rather than assuming future versions preserve this contract.
