# Approved company execution (#285)

This corrects B07's unconditional shell-string launch for the optional company
profile. Personal launch/discovery/configuration behavior remains unchanged.
The protected policy, not a mutable `builtin` flag or command string, supplies
executable authority. Recommendation-only use remains available.

## Executable and argument contract

Each protected `executables` entry names a harness slug, canonical absolute native
executable path, SHA-256 and an argument template array. The matching entry is
required even for copy-command mode. No PATH lookup chooses the launched image.
The executable and all ancestors must satisfy the existing administrator-owned
POSIX/Windows protection checks, with no symlink/reparse components. Executable
content is hashed with a bounded stream and identity/size/mtime are checked again
before use. POSIX requires an execute bit; Windows requires an .exe image. Shell
scripts, .cmd/.bat wrappers and shebang launchers are not executable approvals.
Deploy native CLI builds to protected directories. Administrators may approve a
native runtime with fixed arguments and optional `inputs` path/digest entries for
its script/assets; those explicit inputs receive the same protection/hash checks.
The approved application and its dependencies still require normal endpoint
software controls; which-model does not sandbox the application's later loads.

Known built-in slugs come from the compiled seed registry. Their mutable saved
command/name/builtin values cannot change company execution. Custom slugs require
both a matching protected executable entry and `allow_custom_shell=true`. The
administrator chooses the shell/runtime path and literal command form; `$SHELL`,
project commands and inherited environment cannot choose them. Custom execution
is a separate broad capability and preserves native harness permissions.

Argument placeholders `{model_id}`, `{reasoning}`, `{provider}`, `{profile}` are
whole array elements only. No replacement is performed inside command scripts,
options or quotes. Values are bounded operational identifiers, never whitespace,
control characters, leading option prefixes or shell metacharacters. Unknown or
embedded placeholders are refused. The approved args are passed directly to
os/exec, preserving argument boundaries. Administrators express model/effort flags
in this array; there are no hidden flags appended outside the approved command
form. Existing OpenCode/Kilo catalog provider prefixes and Cline provider mapping
are available through the existing route mapping, with mapped values validated.

## Child environment and launch

Review correction for #308: pure-Go Linux (and `osusergo` builds) reads `/etc/passwd` directly, bounded to 4 MiB and scanner-bounded records. It never accepts `os/user`'s cached HOME/USER fallback. A missing, malformed or unavailable account record refuses launch with a fixed diagnostic; NSS-only accounts require a build using native lookup. Native macOS and CGO Linux continue to use the OS account database.

Company children receive a fresh environment containing only OS-derived home,
platform application/temp locations and a fixed system PATH. On Unix, UID/home
come from the OS user database, not HOME; Linux also supplies the standard
UID runtime/Secret Service bus location. Windows uses its current user token with CreateEnvironmentBlock(inherit=false),
GetUserProfileDirectory and the OS Windows directory, not USERPROFILE/APPDATA/
SystemRoot from the parent. Only the documented location fields are selected
from the token environment; its arbitrary user/system variables are not forwarded. UTF-8
locale is fixed. Parent provider tokens, proxy settings, runtime preload/options,
Git configuration injection and arbitrary environment variables are omitted.
Native harnesses can use their normal OS/home credential stores; environment-only
credentials must be configured through the native harness's own approved flow.

Working directory remains the user's selected current project: agents need its
files and may read project instructions/configuration under native harness rules.
The environment/path approval does not make project content trusted or restrict
an authorized agent's later tools/network. Endpoint controls and native harness
permissions govern those effects. Company children discard captured stdout/stderr
and use #284's structured, bounded launch record. Audit/history/recording failures
are advisory and do not block an otherwise authorized launch.

## Integration controls and removal

Skill installation, hook installation and hook use retain independent protected
booleans. A prohibition fails before file/process effects. Installation permission
allows the selected shipped integration content; review that content and native
harness permissions before enabling it. Removal remains available with installation
disabled and preserves foreign settings/entries. Company skill removal cannot use
`--force` to override a modified-file ownership mismatch.

Legacy CLI credential discovery is still denied: approving a harness does not
approve `gh auth token`, arbitrary credential helpers or their environment.
CodexBar has its separate approval consumer in #287.

## Pinned evidence

Native macOS/Windows/Linux fixtures verify administrator-owned images, exact argv,
fixed child environment, changed digest/path refusal and absence of substitute
PATH launches. Unit/integration cases pin disabled capability side-effect absence,
custom-shell denial despite mutable settings, typed provider mapping, metadata
substitution boundaries and removal of owned integrations with foreign data kept.


Company discovery uses protected allowed provider IDs, a matching native provider
default and explicit user provider switches; it does not inspect credential-bearing
harness files. Cline's optional configured-provider mapping requires provider_file
permission; otherwise the existing compiled catalog/provider alias is used.

Copy-command mode verifies the same approval then returns quoted POSIX-shell
syntax. Windows PowerShell 5.1 and PowerShell 7 copy text uses ProcessStartInfo
with UseShellExecute=false and the same Windows argv serialization as os/exec,
bypassing PowerShell's version-dependent native argument binder. Empty arguments,
embedded quotes and backslashes must survive unchanged. It inherits the terminal's
current filesystem location and standard I/O, waits for completion and publishes
the native exit status through LASTEXITCODE. Copying does not start a process; manually running
the text uses the terminal's environment and is outside the app's child-environment
boundary. The approved application's later subprocesses, dynamic dependencies,
project hooks and network actions remain under endpoint/native harness controls.

Recognized shell image names (sh/bash/dash/zsh/ksh/fish/cmd/powershell/pwsh) also
require allow_custom_shell even if assigned a built-in slug. The administrator
must approve the actual intended program; filename checks do not classify renamed
or arbitrary programs, and an approved native executable can itself invoke tools.
