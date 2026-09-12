# Managed secure stores and explicit migration (#283)

This extends F12 and the F01 managed-policy contract. It intentionally replaces
managed-mode fallback with explicit OS-store outcomes while preserving personal
storage defaults. Native adapters remain excluded from `nousage` builds.

## Storage selection and outcomes

Managed installations use macOS Keychain, Windows Credential Manager or a Linux
Secret Service on the local user's protected session bus when policy permits
`keychain`. No company account, endpoint or credential broker is introduced.
Missing items, locked/interaction-required stores, access denial and unavailable
stores are distinct internal outcomes with known, credential-free messages.
Only a missing item is absence; other secure-store failures must not be disguised
as missing credentials. Secure-store-only resolution performs no managed-file
stat/read/write after any of those outcomes. Save never falls back to plaintext.

Personal installations retain macOS keychain preference and the existing fallback
behavior on other systems. Explicit `[auth] native_keychain = true` opts a personal
installation into the new native adapter. Company authority selects native secure
storage independently of that preference. Desktop settings preserve this setting.

The resolution-only `managed_store` marker records the actual permitted source:
`keychain` or `managed_file`. Native personal mode retains file fallback when
the keychain is disabled or unavailable; Codex consumes that resolved token and
routing metadata without reopening provider files. The marker is never stored
in a credential record, and a stored marker cannot override the actual source.
Default personal mode retains its existing provider-file precedence. A failed
personal sign-in rolls back a newly saved fallback file as well as OS storage.

The pinned go-keyring Windows adapter is reused. The macOS adapter calls the
fixed Security/CoreFoundation frameworks through pinned purego v0.11.0, including
CGO-disabled release builds. It selects the OS default keychain, checks lock state,
serializes noninteractive calls and restores the process interaction preference.
It verifies writes by reading the exact value back.
The Linux adapter uses the already-pinned D-Bus dependency and the Secret Service
protocol without implicit unlock prompts; it connects to the protected local
session bus independently of user-supplied bus-address environment variables.
Lookup, update and removal select the same unique matching item across collections.
An existing item is updated in its original collection even if the default changes;
only a missing item is created in the current default collection. Locked or
ambiguous matches refuse the write. Writes require unique lookup and exact secret
read-back before reporting success; a missing or mismatched result is unavailable.
Other native failures retain their typed outcomes. Failed verification may leave
a committed OS entry; it never authorizes company plaintext fallback.
Unsupported sizes or unavailable OS services are explicit failures, never a reason
to persist plaintext. Native integration tests use disposable synthetic stores.

## Authentication and provider sources

Provider-owned credential files remain a separate administrator source allowance.
Company login saves the required access token and provider routing/expiry metadata
to secure storage; it does not write native provider files. The Codex adapter can
consume the securely resolved access token/account identifier without reopening
native auth/config files. No new identity claim, refresh-token broker or provider
authorization scope is implied. Personal provider-file behavior remains unchanged.

The desktop catalog API key is authentication material too: company-mode reads,
writes and removal use the approved OS store instead of the legacy catalog key
file. Catalog credential source and provider permission checks apply before use.
Existing provider-owned files are not silently changed or deleted.

## Migration and removal workflow

The workflow is explicit `auth migrate <provider>`
with optional `--remove-source` and `--replace`. A different existing OS entry
is retained unless `--replace` explicitly authorizes replacement. In company mode it additionally requires protected
administrator `allow_credential_migration: true` (default false). That permission
is narrow authority to read the existing which-model-owned legacy credential file
for migration; it does not enable ordinary file fallback or provider-file access.

Migration validates a bounded legacy record whose leaf is a regular, non-symlink file, writes only to the
native store and reads back to verify the exact credential before considering
source removal. No source is removed unless the user supplied `--remove-source`.
A removal failure or changed source reports a residual copy; successful secure
storage is not misreported as complete cleanup. Failure before secure verification
leaves the source intact. The command never prints the credential or its metadata.
A concurrent source replacement is preserved; if restoration cannot safely
recreate the original name, the report identifies an adjacent recovery basename.
The command never follows provider-native files. `artificial-analysis` selects
the separate owned `aa_api_key` file and requires company enrollment; personal
catalog storage remains unchanged.

Personal migration also enables the explicit native-store preference, reporting
configuration persistence failures without claiming a completed transition.

Removal targets only which-model-owned entries and explicitly selected legacy
copies. It reports incomplete cleanup and never silently removes provider-owned
credentials. Rollback after a settings failure must restore all saved metadata and
must not use broad removal that deletes unrelated legacy files.

Windows configuration writes used by sign-in/migration use native replacement
semantics; unsupported directory fsync must not produce a false failure after a
successful rename. Pre-commit failures and actual committed-write uncertainty
remain distinguishable.

## Required evidence

- Unavailable/locked/denied/missing cases, including a pre-existing plaintext
  canary file and counters proving no fallback probes, reads or writes.
- Native save/read/delete on all three OSes and native lock/unavailable behavior
  where the OS exposes it; Windows has no independently lockable per-item vault.
- Source permission, metadata preservation, migration residual-copy and rollback
  tests; no credential canaries in errors, output, logs or status rendering.
- Personal compatibility, native Windows configuration persistence and default/
  `nousage` build audits.

## Platform limits and trust boundary

macOS and Linux bound records at 64 KiB. Windows stores at most 2560
credential bytes per entry. Oversize is explicit
`too_large`; this implementation does not split tokens across entries or persist
an oversized token elsewhere. Provider tokens exceeding the platform limit need
an independently approved source or a future storage-format change.

Linux requires an unlocked default Secret Service collection on the fixed
`/run/user/<uid>/bus` socket in an owner-private runtime directory. It never
opens an arbitrary D-Bus address from environment variables, auto-unlocks or
starts a daemon. D-Bus session encryption is `plain` over that authenticated,
local Unix socket; lookup attributes contain application/provider labels only.
The OS user, session service and endpoint protections remain the trust boundary.
Windows Credential Manager has no separately lockable per-item vault; denied
logon access is distinguished from missing items and unavailable services.

macOS uses native generic-password APIs, with no shell/helper and no secret argv
or stdin. The earlier utility approach was superseded after native CI demonstrated
that a locked keychain waited for UI until timeout. `SecKeychainGetStatus` and
scoped `SecKeychainSetUserInteractionAllowed(false)` preserve locked versus missing
outcomes and prohibit authentication prompts during each serialized operation.
These compatibility APIs are deprecated by Apple but remain available; the native
CI gate covers their supported OS behavior. Framework paths are fixed. The new
purego dependency is required to call them from CGO-disabled release binaries;
its version is pinned and included in the existing SBOM/vulnerability workflow.
Native Keychain access control applies to which-model itself. After a binary
identity/path change, an endpoint owner may need to approve its access with OS
tools; the adapter never broadens the item's ACL. Existing go-keyring encodings
remain readable when OS access permits them.

Windows replacement uses `MoveFileEx` with replacement/write-through flags after
syncing the staged file; this corrects the unsupported parent-directory flush,
without promising immunity to every hardware/power-loss failure.

The native stores necessarily retain access-token routing and expiry metadata
inside OS-protected records. That narrowly required credential metadata is
separate from #284's identity-free usage/history persistence. ID tokens and
refresh tokens are not retained by company sign-in; expired access tokens require
sign-in again. JWT claim parsing supplies routing metadata, never verification of
identity, signature, organization membership or additional privileges.
