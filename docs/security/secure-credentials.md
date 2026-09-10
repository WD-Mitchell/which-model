# Company credential storage and migration

An enrolled company installation stores which-model credentials in the user's
macOS Keychain, Windows Credential Manager or Linux Secret Service. The protected
policy must permit the provider and `keychain` source. It never falls back to a
plaintext managed file when secure-store-only policy applies. Personal defaults
remain unchanged; a personal user can explicitly select the native adapter with
`auth.native_keychain = true` in configuration.

## Before sign-in

Provision the [company profile](managed-company-profile.md). Linux also needs an
unlocked default Secret Service collection on the user's protected local session
bus (`/run/user/<uid>/bus`). which-model does not start or unlock a service and
ignores alternate bus addresses in environment variables. Use the OS's credential
management tools to unlock the store or resolve access denial, then retry.
Missing credentials, locked/interaction-required stores, denied access, unavailable
services and oversize credentials have distinct internal outcomes and known
messages. None authorizes a plaintext fallback.

Company Codex/Claude sign-in retains the access token and required routing/expiry
metadata in the OS store. It does not retain ID/refresh tokens or write native
provider credential files. An expired token requires sign-in again. Codex account
claims supply routing metadata; which-model does not independently verify company
membership. The provider validates the access token. Normal usage/history
persistence and retention are governed separately by #284.

The Artificial Analysis catalog key has a separate OS entry under service
`which-model-catalog`, account `artificial-analysis`; its provider permission must
be approved too. Company catalog refresh does not export that key into the process
environment. Other managed credentials use service `which-model`, account
`<provider>`. OS user/session access controls remain authoritative.

## Migrate an existing which-model credential

An administrator must explicitly add `"allow_credential_migration": true` to
protected policy before company migration. This allows only the selected owned
legacy file to be read for migration. It does not permit ordinary plaintext
fallback or provider-owned credential discovery.

```sh
which-model auth migrate copilot --json
which-model auth migrate copilot --remove-source --json
```

The first command leaves the legacy copy intact. The second removes that selected
copy only after the OS store is written and read back successfully. A different
existing OS credential is preserved unless the user explicitly adds `--replace`.
Personal migration additionally persists the native-store preference before
considering source removal. A project/environment override or configuration-write
failure leaves the source and reports the incomplete transition.

For the separate owned catalog file `aa_api_key`, an enrolled company user can run:

```sh
which-model auth migrate artificial-analysis --remove-source --json
```

The report includes `secure_store` (`unchanged`, `unverified` or `verified`) and
`legacy_copy` (`retained`, `removed` or `recovery`). Failures exit unsuccessfully.
`recovery_file`, when present, is the basename of a retained copy beside the source.
Review it with authorized OS tools. A changed source is preserved, never silently
overwritten during recovery. No token or account metadata is printed.

Company `auth logout <provider> --yes` removes the owned OS entry and reports legacy
copies as uninspected. Neither normal logout nor migration deletes provider-owned
authentication files, backups, snapshots or synchronized copies. Removing the
selected file is not a claim of secure erasure or complete historical cleanup.

## Limits and verification

Windows entries are limited to 2560 credential bytes. macOS and Linux records
are bounded at 64 KiB. Oversize credentials fail explicitly; they are not
split or persisted elsewhere. Windows has no independently lockable per-item
vault. Headless or nonstandard Linux sessions may need endpoint provisioning.

Native CI uses disposable stores and synthetic credentials on all three systems.
Failure and migration tests cover denied/locked/unavailable/missing outcomes,
legacy-file canaries, exact read-back, explicit replacement and changed sources.
The [F12 contract](../../specs/features/F12-credentials/SECURE-STORES.md) records the
storage protocol and test obligations. The offline score-only package remains
independent of all credential adapters.

The macOS adapter accesses Security.framework directly without showing an unlock
prompt. Use OS tools to unlock the default keychain or approve access for the
current which-model binary; upgrades/path changes may require a fresh OS grant.
It never broadens an item's ACL. Framework calls use pinned purego v0.11.0 so the
CGO-disabled release has the same native support; the release SBOM and vulnerability
scan include that dependency. Apple deprecates the compatibility Keychain APIs;
native CI remains a required gate for supported macOS versions.
