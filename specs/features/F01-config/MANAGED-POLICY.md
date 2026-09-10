# Administrator-managed policy (#282)

This extends F01's configuration contract with an optional, independent machine
policy. The policy is authoritative inside the approved application. Personal
installations need no administrator, account or server and keep their existing
configuration behavior. Native permissions and endpoint controls remain authoritative.

## Enrollment and trusted origin

The fixed policy directory is `/Library/Application Support/which-model/managed`
on macOS, `/etc/which-model/managed` on Linux, and the Windows machine ProgramData
known folder followed by `which-model/managed`. Windows uses the OS known-folder
API, not environment variables. Runtime flags, alternate config files, environment
variables, user/project settings and desktop mutations cannot change this origin.

`required.json` contains `{"schema_version":1,"required":true}`. Installing this
protected marker requires a valid `policy.json`; an absent, invalid, unreadable or
insufficiently protected required policy refuses restricted operations. A protected
policy without the marker also activates managed mode. When neither exists the
installation remains personal. Removing enrollment is an administrator operation;
the application never creates, edits or removes machine policy.

Each existing policy/marker file and parent directory is checked. Symlinks/reparse
points, non-regular files, oversized documents and ownership/permission failures
are refused. POSIX files must be root-owned and not group/other-writable, with
root-owned protected parents. macOS extended ACLs are refused conservatively;
Linux access ACLs cannot grant write beyond the checked mode's group mask.
Windows requires an administrator/SYSTEM (or OS TrustedInstaller) owner and a DACL with no untrusted write,
delete, ACL/owner-change grants on policy files; parent delete-child/replacement
rights are checked. Normal ProgramData directory metadata/sibling-creation grants
are permitted: every existing next component is separately protected against
removal and a nonempty directory cannot become a reparse point.
Unrecognized ACL grants are refused rather than guessed. Readers inspect the
opened file as well as its path and reject replacement during the read.

Both JSON documents are bounded to 64 KiB, schema-versioned, reject unknown or
duplicate fields and never contain credential material. Policy diagnostics name
the protected origin and violated restriction, not file contents or secrets.

## Policy schema and defaults

```json
{
  "schema_version": 1,
  "name": "Company profile",
  "allowed_providers": [],
  "credential_sources": ["keychain"],
  "secure_store_only": true,
  "allow_credential_migration": false,
  "identity_free": true,
  "retention": {
    "usage_snapshots_hours": 24,
    "launch_logs_days": 7,
    "pick_history_days": 30,
    "audit_records_days": 30
  },
  "integrations": {
    "skill_installation": false,
    "hook_installation": false,
    "hook_use": false
  },
  "executables": [],
  "codexbar_installations": [],
  "allow_custom_shell": false
}
```

Omitted settings retain these managed defaults. Credential source names are
`keychain`, `environment`, `provider_file`, `managed_file` and `cli`; the latter
requires an approved process implementation. `secure_store_only` forbids
`managed_file` and legacy native credential-file writes regardless of lower-trust storage preferences. Administrator
source selection may explicitly permit provider-owned files independently of
which-model's own storage. Providers are allowed explicitly; this never enables
them in user configuration. Ranking preferences may select allowed profiles,
weights and models without granting sensitive capabilities.

An executable approval has `id`, absolute `path`, SHA-256 `sha256` and `args`
(separate argument-template strings). A CodexBar approval has absolute `path`,
`sha256`, and required `config: {path, sha256}`; both are protected reviewed inputs
(global CONTRACTS §16, F14/APPROVED-CODEXBAR.md). Empty approvals prohibit execution/delegation. Approval metadata is
not authorization to run a legacy unverified command path: the consuming operation
must implement the approved path, digest, arguments and environment contract.
Those consumers are completed by #285/#287. Retention and identity-free persistence
are implemented by #284; #283 completes native secure stores and migration. The
policy origin/precedence and pre-effect guards are implemented here; this foundation
alone is not evidence that the entire managed-usage milestone is complete.

## Precedence and inspection

The shared policy reader is independent of normal configuration loading. It runs
before sensitive operations, including calls made directly through desktop services
or credential/integration packages. Configuration loading checks the active policy
before migration/discovery and validates merged user/project/env values against it.
Explicit `--config` changes ordinary configuration only. Cloning/marshalling a user
configuration cannot create, weaken or serialize administrator authority.

`which-model config policy [--json]` reports managed/personal state, required
enrollment, protected origin, policy digest and effective restrictions without
credential data. A required-policy error remains inspectable as an actionable
diagnostic. Desktop mutations use the same validation and operation checks; saving
`auth.use_keychain=false` cannot authorize a forbidden managed-file fallback.

Quota/authentication-evidence failures and audit-write failures remain advisory
under the user's #286 decision. Missing required administrator policy and refused
credentials/executable permissions are separate security failures; they do not
override the native harness permission model.

## Pinned evidence

- No enrollment retains personal behavior; trusted marker/policy activates managed mode.
- Missing required policy, invalid schema/JSON, duplicate/unknown fields, symlinks,
  oversized input, untrusted ownership/ACL and writable parents refuse authority.
- User/project/env/flag/alternate-config/desktop overrides cannot enable forbidden
  credential-file access, providers, integration installation or legacy launch.
- Allowed ranking preferences coexist with restrictions; policy inspection contains
  no policy-file contents on errors and no credential canary values.
- Native macOS, Windows and Linux protection checks supplement deterministic
  precedence and operation-boundary tests. The restricted offline distribution's
  import/credential/network exclusions remain intact.

## Secure-storage correction (#283)

`allow_credential_migration` defaults to false. An explicit administrator grant
permits only the `auth migrate` workflow's selected which-model-owned legacy file;
it grants neither ordinary plaintext fallback nor provider-file access. Native
secure login and migration now follow [F12 secure stores](../F12-credentials/SECURE-STORES.md).
`artificial-analysis` is the catalog API-key provider permission. Personal
`auth.native_keychain` opts into native stores and cannot weaken company policy.

Zero retention is supported: an administrator value of `0` disables persistence
for that category and removes its existing owned records when maintenance or a
write is attempted. It never means unlimited retention.


The #285 executable consumer verifies administrator ownership/ACLs and exact
SHA-256 at launch and passes only protected argument forms to os/exec. Each
entry may additionally declare `inputs:[{"path":"<canonical absolute input>",
"sha256":"<64 lowercase hex>"}]` (maximum 16, unique paths). Inputs support
approved native runtimes with pinned script/assets. The immutable command contract
and environment are specified in B07/MANAGED-EXECUTION.md. A harness approval does
not authorize legacy credential CLI discovery or CodexBar; their generic guards
remain denied until their specific consumers prove the operation.
