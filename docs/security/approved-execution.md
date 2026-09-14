# Approved company execution

The optional company profile can approve individual native harness installations
and command forms. Personal users keep their current discovery, command templates
and shell behavior. The profile remains local OS/provider identity, with native
harness permissions authoritative for agent tools and project access.

## Administrator setup

Install the native CLI in an administrator-owned directory. Its file and all
parents must pass the same POSIX ownership/ACL or Windows owner/DACL checks as
company policy. User-writable package-manager installations are not suitable for
this boundary. Resolve symlinks to an administrator-protected canonical path and
record its SHA-256. Native ELF, Mach-O and Windows .exe images are supported;
shebang/.cmd/.bat launchers are refused.

Add an entry to the protected policy, for example on a Unix deployment:

```json
{
  "schema_version": 1,
  "allowed_providers": ["codex"],
  "executables": [{
    "id": "codex",
    "path": "/opt/company/bin/codex",
    "sha256": "REPLACE_WITH_THE_REVIEWED_BINARY_SHA256",
    "args": ["-m", "{model_id}", "-c", "model_reasoning_effort=high"]
  }]
}
```

Replace the digest with 64 lowercase hexadecimal characters before deployment;
the placeholder is intentionally invalid. Windows uses its canonical absolute
.exe path and the corresponding digest. Verify the installation/update through the
company's release process; updating the file requires updating its approval.

The application never searches PATH for the approved image. The protected `args`
array is the entire command form: each element becomes one process argument.
`{model_id}`, `{reasoning}`, `{provider}` and `{profile}` may occupy a whole element.
Embedded placeholders such as `--model={model_id}` are refused; use
`["--model", "{model_id}"]`. Route/profile substitutions accept bounded operational
IDs and reject option prefixes, whitespace, control and shell metacharacters.
Choose explicit effort flags/values supported by the approved CLI; which-model
adds no flags outside the approved array. OpenCode/Kilo model prefixes retain
catalog provider mapping; Cline uses the compiled provider alias unless separately
allowed to inspect its provider configuration.

For a native runtime invoking a script, put the script path in the protected args
and declare `inputs` with its canonical path and digest. Each declared input is
verified with the same ownership/hash checks, for example
`"inputs":[{"path":"/opt/company/tool/entry.js","sha256":"<reviewed SHA-256>"}]`.
The administrator owns the complete dependency/runtime installation; a digest of
one entry point does not attest every module or later plugin the program loads.

Built-in identities come from the compiled harness registry. Editing a project
command or setting `builtin=true` cannot change the approved image or turn a custom
harness into a built-in. A custom slug additionally needs
`"allow_custom_shell":true` and its own approved executable/args entry. The
administrator selects the shell/runtime explicitly; `$SHELL` is ignored. Do not
put substitution placeholders inside shell scripts; pass them as separate positional
arguments to an administrator-reviewed script. Enabling this capability permits
the reviewed shell program's behavior and is a broader company decision.

## Child environment and project scope

The launched child receives a fresh environment:

| OS | Values supplied |
|---|---|
| macOS | HOME/USER/LOGNAME from the OS user lookup; PATH `/usr/bin:/bin`; LANG `en_US.UTF-8`; TMPDIR `/tmp`. |
| Linux | OS user home/name, fixed PATH `/usr/bin:/bin`, LANG `C.UTF-8`, TMPDIR `/tmp`; standard XDG home paths and `/run/user/<uid>` runtime/Secret Service bus path. |
| Windows | Current-token user profile and non-inherited token environment for roaming/local application data and ProgramData; OS Windows/system directory; fixed system PATH; profile home fields and local application-data Temp; fixed UTF-8 locale. |

Pure-Go Linux requires an account record in `/etc/passwd`; an NSS-only or missing
account is refused even when HOME/USER are set. Use a build with native account
lookup for NSS-only directory-service accounts. The local database read is bounded
to 4 MiB with bounded records. macOS native lookup and CGO Linux retain OS account
service support. No missing account details are filled from inherited variables.

Parent provider tokens, shell selectors, proxies, runtime/preload options and Git
configuration injection variables are omitted. Native harnesses can use their
approved OS/home credential stores. Environment-only login needs the native
harness's own approved configuration; which-model does not forward a token just
because it exists in the parent's environment.

The current project remains the working directory. An authorized agent can read
project instructions/configuration and invoke its own tools under native harness
permissions. The approved application, dynamic libraries, subprocesses and network
activity still require endpoint and native harness controls. This feature does not
sandbox them or create an independent application identity.

Company discovery shows only protected allowed providers, with the matching native
provider selected by default. Explicit user provider switches may select another
allowed provider. It does not scan native credential-bearing files to infer access.
The Installed flag is a discovery hint that an approved regular file exists; the
full hash/ownership proof runs immediately before launch.

Copy mode checks the same approval and returns quoted POSIX-shell text, or
PowerShell text on Windows. The Windows text supports Windows PowerShell 5.1 and
PowerShell 7, preserving empty/quoted arguments through an explicit native command
line. Running it retains the terminal filesystem location and standard I/O and
sets LASTEXITCODE when the child finishes. It starts no child until pasted and run;
manually running the copied text
uses the terminal's environment. A user already able to run native tools directly
remains governed by endpoint/native permissions.

## Integrations, records and cleanup

Skill installation, hook installation and hook use have independent protected
booleans. A disabled operation stops before file/process effects. Enabling an
integration permits the selected shipped integration content; review that content
and the native harness's permissions as part of deployment.

Removal remains available after installation is disabled. Only owned entries/files
are removed, with foreign settings preserved. Company `--force` cannot override a
modified skill file; review and remove that edited file manually if appropriate.
Personal force behavior is unchanged.

Company launches discard captured child stdout/stderr and use the typed launch
records described in [privacy and retention](privacy-and-retention.md). Recording
failure is visible and advisory. Missing quota/authentication evidence or failed
audit persistence does not gain launch-blocking authority;
[advisory evidence](advisory-evidence.md) defines the consistent reporting
implemented in #286. Legacy credential commands and external login/model helper
commands remain denied until they have their own verified consumers. A harness
approval does not approve CodexBar; [approved delegation](approved-codexbar.md)
defines the separate #287 boundary.

The governing spec is [managed execution](../../specs/desktop/backend/features/B07-harnesses/MANAGED-EXECUTION.md).
Native CI verifies protected images, exact arguments/environment, altered or
redirected image refusal, installation/use gates, and owned removal after the
administrator disables installation.

Recognized shell images require separate custom-shell permission even if placed
under a built-in approval ID. Review the actual approved program and its command
form: filename checks cannot classify renamed programs or prevent an authorized
native application from invoking its own tools.

Windows derives the source block using [CreateEnvironmentBlock](https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-createenvironmentblock)
with inheritance disabled, then selects only the documented location fields.
It does not pass the complete token environment or any arbitrary persisted
user/system environment variables to the child. Credentials belong in the native
secure store; keep them out of policy arguments, which are visible as process
arguments and copyable command text.

For Claude hooks, company removal also requires a recognized manifest version
and exact shipped hook ID/event/matcher/command tuples. A forged foreign-command
claim or an unrecognized older command is refused for manual review before
settings are changed.
