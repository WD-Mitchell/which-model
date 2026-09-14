# Optional administrator-managed company profile

The company profile adds a machine-level policy for approved installations of
which-model. It controls provider access, credential sources, integration
installation and execution independently of user preferences. Personal users do
not enroll and keep their existing configuration, sign-in and launch behavior.
There is no company account, central credential broker or application RBAC.

This is the policy foundation for the managed-usage milestone. The complete
deployment includes [native secure stores and migration](secure-credentials.md)
(#283) and [retention and identity-free persistence](privacy-and-retention.md)
(#284) and [approved execution](approved-execution.md) (#285). Approved CodexBar
delegation (#287) remains a separate control.
The [restricted offline package](offline-score-only.md) remains available separately.

## Enrollment

An endpoint administrator provisions `required.json` and `policy.json` in the
fixed machine directory below. Neither normal configuration nor an environment
variable can supply an alternative policy origin.

| OS | Protected directory |
|---|---|
| macOS | `/Library/Application Support/which-model/managed` |
| Linux | `/etc/which-model/managed` |
| Windows | The OS ProgramData known folder, followed by `which-model/managed` |

`required.json` contains:

```json
{"schema_version":1,"required":true}
```

Start `policy.json` with the [documented schema and defaults](../../specs/features/F01-config/MANAGED-POLICY.md#policy-schema-and-defaults).
For example, this permits the three named providers when the user separately
enables them, while retaining all other managed defaults:

```json
{
  "schema_version": 1,
  "name": "Company profile",
  "allowed_providers": ["codex", "claude", "copilot"]
}
```

The defaults select keychain storage, prohibit plaintext fallback and delegated
commands, and disable skill/hook installation and hook execution. They record
identity-free persistence and retention of 24 hours for usage snapshots, 7 days
for launch logs, and 30 days for pick history and audit records. The persistence
consumer in #284 implements those retention settings; storing the settings alone
does not delete existing data. Native provider files require a separate explicit
source allowance. Company login now saves required access-token metadata in the native OS store;
secure-store-only policy prevents native provider-file writes.

A protected policy alone activates managed mode. The separate required marker
ensures that an accidentally missing policy is an error rather than a return to
personal mode. Install both through endpoint management. With neither present,
the application remains personal. The application never changes enrollment.

## OS protection requirements

On macOS and Linux, make the policy files and their ancestors root-owned; remove
group/other write permissions. Typical modes are `0755` for directories and
`0644` for these non-secret JSON documents. Use a local filesystem with native
POSIX permissions. On macOS, the reader conservatively refuses extended ACLs on
the policy path even when an administrator believes an ACL is harmless. It checks
ACL metadata through the native file-descriptor API, without launching a helper.
Linux access ACL write grants are constrained by the checked POSIX group mask.

On Windows, use a local NTFS directory with SYSTEM or the Administrators group as
owner. Grant SYSTEM and Administrators control and ordinary users read/traverse
access. Remove inherited write/delete/ACL-change rights for other principals.
The reader also accepts OS TrustedInstaller ownership for protected system
ancestors. It rejects null DACLs, unfamiliar grant types, writable policy files,
and untrusted delete-child or security-change rights on parents. It does not try
to compensate for unsafe allow entries using deny entries. Mere permission to
create an unrelated sibling or change directory metadata in a system ancestor is
not permission to replace an existing protected child. Such ancestors remain
nonempty because their existing children are protected against deletion; Windows
[refuses setting a reparse point on a nonempty directory](https://learn.microsoft.com/en-us/windows-hardware/drivers/ifs/fsctl-set-reparse-point). The Windows origin comes from the known-folder API,
not `%PROGRAMDATA%` supplied by the application environment.

Symlinks and Windows reparse points are refused, including redirects above a
missing marker. Files must be regular and at most 64 KiB. Readers compare the
opened file with its path and reject replacement or changes observed during the
read. JSON rejects duplicate keys, unknown fields and unsupported versions.

These checks use [Windows security descriptors](https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-getsecurityinfo),
[file access rights](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights),
[known folders](https://learn.microsoft.com/en-us/windows/win32/api/shlobj_core/nf-shlobj_core-shgetknownfolderpath)
and [Apple's file-attribute interface](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlist.2).

Policy JSON field names are case-sensitive throughout both documents. Mixed-case
and Unicode aliases are rejected, including duplicate spellings that an ordinary
Go JSON decoder might otherwise merge.

## Inspection and changes

Run `which-model config policy --json` to inspect enrollment, the fixed policy
origin, SHA-256 digest and effective restrictions. Errors report the origin and a
known diagnostic; they do not print the rejected document or credential input.
User/project files, `--config`, environment settings and desktop changes cannot
weaken these restrictions. Ordinary ranking preferences still work.

Use endpoint management to replace policy atomically while retaining protection.
Policy is reloaded at operation boundaries, so an administrator change affects
subsequent operations without making mutable desktop state authoritative. Stop
the application when revoking access immediately: an already issued request or
child process is not retroactively cancelled by changing a file.

Device login also reloads policy before every token poll. Revoking the provider
or losing a required policy stops the next request, including after a pending or
slow-down response. CLI reports policy errors with exit code 2 and desktop reports
`validation_failed`.

Default managed policy refuses legacy shell launch, credential CLI helpers,
CodexBar discovery/delegation and model-discovery subprocesses. Approval metadata
does not enable them until their verified consumers are implemented. These are
capability permissions. Missing quota evidence and failed audit writes remain
advisory under the separate #286 decision and do not add a launch gate.

Native CI tests provision protected fixtures on all three operating systems,
exercise missing/invalid/writable/redirected policy, and call actual credential,
integration, sign-in and launch APIs against the fixed enrolled origin. Tests
also cover configuration overrides, desktop storage mutations and forbidden file
fallback after keychain failure.

The profile applies inside the approved application. Endpoint management must
control enrollment, approved binaries, local administrator privileges and provider
accounts. A user who can replace the application or act as an administrator can
bypass application-level checks. See the [local identity and control-owner decision](company-identity.md).

The [privacy inventory and retention guide](privacy-and-retention.md) documents
live provider collection, default identity-free persistence, actual expiry
processing, legacy-data transition and explicit cleanup/purge. These settings are
now consumed by cache/history/audit/launch storage; a stopped application still
requires endpoint-scheduled cleanup for wall-clock deletion requirements.


CodexBar delegation remains default-off. An administrator can now approve a protected native image and configuration together; see [approved CodexBar](approved-codexbar.md) for the schema, child inputs, reapproval process and separate upstream trust boundary.
