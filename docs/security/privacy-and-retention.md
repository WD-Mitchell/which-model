# Company privacy and retention

Inventory version: 2. Decision records: #284, #286 and #287.
The implementation baseline is `ba3aed68cf32a40fd84a3ebd0517b886466503d9` plus
#287's approved delegation changes. This document describes the optional managed profile, with
personal differences stated explicitly. It supplies review evidence; the
company's privacy acceptance is a separate decision.

## Data paths

```mermaid
flowchart LR
  S[Approved credential source] --> U[Provider allowance request]
  U --> M[Typed snapshot and minimization]
  M --> C[Retained local records]
```

Live allowance collection requires authentication and may collect account identity
in memory even when persistence is identity-free. Output redaction is separate:
`--show-identity` can display live identity, but cannot recover fields already
removed from a cached snapshot. There is no new telemetry recipient or storage
service in this change. A launched agent has its own provider/tool data flows;
which-model's retention does not control that agent or its provider.

The standalone `which-model-score-only` package uses its embedded repository
catalog and built-in profiles. It does not load credentials, usage caches,
company policy, hooks or harnesses, or make provider requests. See
[restricted distribution](offline-score-only.md) for exact input hashes and
the independent artifact boundary.

## Native usage inventory

The following table describes values on the wire and in transient memory.
Response bodies are parsed into typed snapshots, then the persistence rules below
apply. Access tokens never belong in snapshots or selection evidence.

| Path / code source | Sensitive source and transmitted fields | Recipient and purpose | Collected fields and local treatment |
|---|---|---|---|
| [Codex usage](../../internal/usage/provider/codex) | Approved OS credential access token; account routing ID from the protected record. `Authorization: Bearer` and `ChatGPT-Account-Id`; normal client/JSON headers. | `https://chatgpt.com/backend-api/wham/usage`, allowance retrieval. Managed secure credentials bypass personal auth/config endpoint fallback. | Account/plan where supplied; numeric windows, limits, reset times, usage-known, model scope. Account/plan and free-form labels/hints are omitted from default company caches. Routing account ID remains necessary in the OS credential record. |
| [Claude usage](../../internal/usage/provider/claude) | Approved OAuth token in Bearer header; fixed JSON, client User-Agent and `anthropic-beta` headers. Sources can include the provider-owned `Claude Code-credentials` keychain item or the owned managed item, subject to policy. | `https://api.anthropic.com/api/oauth/usage`, allowance retrieval. | Five-hour/weekly/model-specific windows and extra-usage values; normalized snapshot fields only. Free-form identity/plan/labels/hints are removed before default persistence. No raw response is retained. |
| [Copilot usage](../../internal/usage/provider/copilot) | Approved GitHub token in Bearer header; fixed editor/plugin/User-Agent/API-version headers. | Mandatory identity gate at `https://api.github.com/user`, followed by `https://api.github.com/copilot_internal/user`. | GitHub login becomes transient snapshot Account; plan and quota windows are normalized. Default persistence drops account/plan. User profile fields unrelated to allowance are not copied into snapshot/history/audit. |

[Credential resolution](../../internal/usage/credential) applies the protected
provider/source policy independently of ordinary configuration. The personal
source inventory includes provider environment tokens, provider-owned native
files/keychain items, Git configuration, `gh auth token`, and which-model's owned
fallback credential file. The default company profile permits keychain sources
only and has no enabled providers until the administrator selects them. Legacy
CLI credential discovery remains denied; a harness approval is not a credential-
helper approval.
The actual allowed sources are inspectable with `config policy --json`.

Personal Codex may use its documented, explicitly trusted configured origin
fallback after selected unsupported endpoint statuses. That does not authorize a
company secure-store credential to be sent to a project-selected endpoint.

## Authentication and delegated helpers

| Flow / source | Transmitted values and recipient | Scope / authority | Persistence and deletion owner |
|---|---|---|---|
| [Codex device login](../../internal/usage/provider/codex/codex_login.go) | `https://auth.openai.com/api/accounts/deviceauth/usercode`: client ID; `/api/accounts/deviceauth/token`: device-auth ID/user code; `/oauth/token`: authorization code, client ID, redirect URI and code verifier. Browser/device state and returned tokens exist in memory. | Existing Codex provider client/device flow; this implementation does not add a separate scope parameter or an independent application identity. | Company stores access token plus required account routing ID/expiry in the OS store. ID/refresh tokens are not written by the company flow. Personal native provider-file behavior is unchanged. |
| [Claude browser login](../../internal/usage/provider/claude/claude_login.go) | `https://claude.ai/oauth/authorize`: client ID, redirect URI, state, PKCE challenge, scopes; `https://platform.claude.com/v1/oauth/token`: code, verifier, state, client ID, redirect URI and grant type. | Existing requested scope string: `org:create_api_key user:profile user:inference user:sessions:claude_code`. This is inherited provider-client authority, not a claim of allowance-only privilege. | Company stores access token/expiry in the OS store; no refresh/ID token or provider file write. Provider grants remain governed/revoked at the provider. |
| [Copilot device login](../../internal/usage/provider/copilot/copilot_device.go) | `https://github.com/login/device/code` and `/login/oauth/access_token`: client ID, device code and grant type; user visits `https://github.com/login/device`. | `read:user`; token also used by the existing Copilot internal allowance endpoint. | Owned OS credential entry in company mode. Logout removes the owned entry; provider grant revocation is separate. |
| [CodexBar usage](../../internal/usage/provider/codexbar) | External process receives provider/source selection and may use its own credentials. It returns provider/source, account or identity.accountEmail, identity.loginMethod, window values/reset descriptions, updatedAt and error code/message. | Company delegation requires protected image and config paths/digests, fixed argv, an OS-derived environment and CODEXBAR_CONFIG bound to the approved input. Native and delegated credentials/storage are separate trust boundaries. | Shared which-model cache minimizes the typed result; company diagnostics replace arbitrary error text. CodexBar's own storage/keychain/network behavior is outside which-model cleanup and requires review for the approved installation. |
| [Antigravity login/delegation](../../internal/usage/provider/antigravity) | Google OAuth at `https://accounts.google.com/o/oauth2/v2/auth`, `https://oauth2.googleapis.com/token`, `https://www.googleapis.com/oauth2/v2/userinfo`; authorization state/PKCE, tokens, email/user info. Existing delegation passes owned credential JSON via `ANTIGRAVITY_OAUTH_CREDENTIALS_JSON` to CodexBar. | `https://www.googleapis.com/auth/cloud-platform` and `https://www.googleapis.com/auth/userinfo.email`. No native usage adapter is registered; this is a login/delegation helper. | Owned credentials use the secure-store policy. Only an approved CodexBar image/config may receive the canonical bounded OAuth credential JSON after verification. Authentication/account-routing fields may include access/refresh/ID tokens, expiry, email and client credentials; unknown fields are dropped. The child may use its own credential stores and refresh behavior. Approval is never inferred from a provider toggle. |
| [Cursor login helper](../../internal/usage/provider/cursor) | Existing external CLI login or browser dashboard at `https://cursor.com/dashboard`. | External/native provider login authority. No native usage adapter is registered. | Provider-owned sessions/files are outside product-data cleanup. Company external login helpers retain their legacy execution denial; a harness launch approval does not grant helper authority. |
| [Artificial Analysis catalog](../../internal/catalog/fetch/aa) | API key in `x-api-key` to `https://artificialanalysis.ai/api/v2/language/models`, optional `/free` fallback, pagination. | Catalog retrieval; separate from provider allowance authentication. | Company key uses dedicated OS service `which-model-catalog`; old `aa_api_key` is touched only by explicit credential migration. Public model/benchmark data persists in catalog files and is not identity-bearing usage history. |

Public catalog downloads/model discovery have their own [catalog](../../specs/features/F08-collectors/SPEC.md)
and [route](../../specs/features/F18-routing/SPEC.md) contracts. They do not receive
provider allowance tokens merely because catalog refresh is requested. Explicit
provider model discovery, where supported, remains an authenticated provider
operation governed by the same provider/source policy.

Company OS storage uses macOS Keychain, Windows Credential Manager or Linux
Secret Service. Necessary credential routing/expiry metadata has the credential's
lifetime, not the 24-hour snapshot lifetime. See [secure credentials](secure-credentials.md)
for owned entry names, migration, native errors, logout and provider-file limits.
Provider grants/scopes and company acceptance belong to the identity/security
owners described in [company identity](company-identity.md).

## Local storage, minimization and deletion

`state` below means the existing resolved application state directory; `cache`
means the application usage-cache roots. The native `os.UserCacheDir()` root and
the configured platform layout's `CacheDir/usage-cache` are both included when
they differ. Desktop overrides are included by its maintenance worker.

| Category | Persisted company fields | Owned location | Default lifetime and deletion |
|---|---|---|---|
| Usage snapshots | Provider/source/confidence, usage-known/stale flags, application recording time, provider timestamp, numeric usage/limit/remaining, reset times, window/model IDs. No account/plan or free-form label/reset hint under identity-free policy. | `cache/usage-cache/<provider>.json` | 24 hours from original application recording time. Read/offline read/maintenance deletes expired data; fresh legacy data is minimized without changing its original clock. Failures are not cached. |
| Pick history | Timestamp, generated ULID, operational profile/strategy/candidate IDs, scores, excluded counts, typed score/band/route evidence and stable exclusion reason codes. No opaque evidence, prompt, account or provider error prose. | `state/pick/history.jsonl` | 30 days. Append/read/maintenance rewrites retained rows and deletes expired ones; removes empty file. |
| Hook audit | Timestamp, fixed schema/privacy version, operational candidate/dispatched model/route ID and the same typed evidence. | `state/audit/evidence.jsonl`, `state/audit/mismatches.jsonl` | 30 days. Append/maintenance prunes; selected/current project's two legacy audit files are deleted rather than copying their undated raw payloads. |
| Launch records | Timestamp, operational harness/provider/model/profile IDs, fixed started/copied/failed outcome. | `state/launch.jsonl` | 7 days. Structured append/maintenance prunes. Company launches do not capture child stdout/stderr into a raw log. Legacy `state/launch.log` is deleted during transition. |
| Native credentials | Provider access token and only required account routing/expiry metadata; separate catalog key. | Owned native OS secure-store entries | Explicit logout/remove/migration controls, not product-record retention. No automatic provider grant revocation. |
| User-authored config, catalogs and installed integrations | User-selected aliases/preferences, public catalog data and integration configuration. | Existing config/catalog/project locations | Outside record-retention cleanup. User/admin manages them explicitly. Do not put personal identity in operational aliases if those aliases are to be retained. |

Operational IDs can be user-chosen. Removing account fields is data minimization;
it is not a guarantee of anonymization for an alias such as a person's name.
The protected `identity_free=false` opt-out permits cached account/plan and
free-form labels/reset hints; typed history/audit/launch minimization remains.
Personal automatic cache/history/audit/log behavior is unchanged.

## Rollout and explicit cleanup

1. Stop older running which-model processes, deploy the reviewed binary and
   protected company policy, then restart the desktop. An old process can retain
   an open log handle or recreate old-format data; this change cannot retroactively
   control it.
2. Inspect `which-model privacy status`. Run `which-model privacy cleanup` to
   apply retention and scrub existing owned stores. It returns JSON counts for
   retained, removed, scrubbed, deleted files and failed files by category.
3. Run `which-model privacy cleanup --project-root <previous-project>` for each
   prior workspace containing owned legacy hook audit files. No repository/home
   crawl or persistent workspace inventory is performed.
4. To delete regardless of age, run `which-model privacy purge`, optionally
   `--category usage,history,audit,launch`. An explicit purge is also available to
   personal users without enrolling them or changing their defaults.
5. If a category fails, treat cleanup as incomplete. The explicit command exits
   nonzero and reports failed_files without printing record contents or local
   paths. Resolve the OS permission/redirect/lock problem and rerun it; do not
   infer deletion from a successfully processed different category.

Automatic maintenance runs at normal full-CLI startup, except config/privacy/
version and --no-usage. The desktop runs it at startup and every minute while
running. Affected reads/writes also maintain their own store. Company offline
usage may therefore write/delete local records while making no network calls.
A stopped application cannot delete files: endpoint management must schedule
`which-model privacy cleanup` if expiry must be processed while the application
is otherwise closed. Schedule it as the data-owning user with the intended data
roots. The standalone score-only package does not run this command or maintenance.

Malformed, missing/future timestamp and oversized owned records are discarded.
Redirected/nonregular files or data directories are refused. Processing uses
bounded sizes and lock waits; overfull directory inventories report incomplete
work. Adjacent empty `.privacy.lock` files contain no records. Cleanup never
recursively removes directories or follows links into provider data.

Deletion removes the owned filesystem entry; it is not secure media erasure.
Backups, synchronized copies, shell redirection/terminal scrollback, external
agent/CodexBar logs, provider-side records and OS credential stores have separate
owners and deletion policies. Audit/history failures remain visible and advisory;
they do not grant or deny permission to launch an agent.

### Review corrections for #307

Company usage warnings now use fixed remediation messages too. Credential-file
permission notices omit user/workspace paths, cache-write failures point to
`privacy cleanup`, and unknown diagnostic payloads are replaced. Personal warning
text is unchanged.

Cleanup also removes abandoned write files in the reserved numeric namespaces
`.<final-basename>.<decimal-uint32>` for all four record categories and
`.tmp-<provider>-<decimal-uint32>` for older personal caches (1–10 decimal digits).
They are uncommitted product data and are removed regardless of age, including
when the final file is missing. Each store's lock protects active company writers;
stop older personal processes as described in rollout. Unrelated backups and lock
files remain outside deletion. State-directory scans share the 1,024-entry bound.
Successful temporary-file removals appear in `deleted_files`, while unsafe entries
or incomplete scans still make cleanup/purge fail with accurate partial counts.

## Review evidence

The governing contract is [managed retention](../../specs/features/F13-usage-cache/MANAGED-RETENTION.md).
Synthetic tests cover original-clock preservation, configured category expiry,
physical deletion, concurrent append/prune, explicit partial failure, redirected
credential protection, identity/payload canaries, central hook audit, fixed usage
diagnostics and structured launch output. The `company-privacy` CI matrix runs
native macOS, Windows and Linux cases. The [approved CodexBar contract](approved-codexbar.md) records the reviewed child environment, upstream stores, endpoints, update procedure and limits; #291 maps evidence to the listed controls.

Zero retention is supported: an administrator value of `0` disables persistence
for that category and removes its existing owned records when maintenance or a
write is attempted. It never means unlimited retention.


Inventory update for #285: approved company launch receives only OS-derived
home/platform directories and fixed system PATH/locale. Parent provider tokens,
proxy/runtime/preload and arbitrary variables are omitted. Approved harnesses may
subsequently read their own OS/home credentials and the selected project under
native permissions. Company provider discovery uses allowed provider IDs and
explicit preferences instead of scanning credential-bearing harness files. Cline's
optional provider-ID mapping reads its bounded configuration only with an explicit
provider_file allowance. New launch records remain the typed seven-day store;
raw child output is not captured. See [approved execution](approved-execution.md).


## Advisory launch audit fields (#286)

The central audit store additionally permits generated `launch_id`, operational `profile`, `quota_state` (global §15 enum), and phase `launch_intent`, `launch_started`, `launch_failed` or `copy_prepared`. They retain the approved 30-day audit default; structured launch summaries retain seven days. A zero-day audit policy persists no evidence and cannot report audit success. Intent is not a process-start claim; neither a missing audit record nor a recording failure proves launch was blocked.
