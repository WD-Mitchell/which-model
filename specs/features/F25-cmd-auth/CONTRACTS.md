---
kind: feature-contracts
version: "1.0"
feature: F25-cmd-auth
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F25 — cmd-auth: Contracts

## 1. Owned files

- `pkg/whichmodel/auth_cmd.go` — cobra command with the three subcommands, `//go:build !nousage` (F22 command-wiring contract; one file per command, self-registering `init()`).
- `pkg/whichmodel/auth.go` — pure logic: status resolution/redaction, login orchestration, logout orchestration. Compiles under both build tags (F21 stubs).
- Tests: `pkg/whichmodel/auth_cmd_test.go`, `pkg/whichmodel/auth_status_test.go`, `pkg/whichmodel/auth_redaction_test.go`, `pkg/whichmodel/auth_expiry_test.go`, `pkg/whichmodel/auth_login_test.go`, `pkg/whichmodel/auth_logout_test.go`, `pkg/whichmodel/auth_json_test.go`.

No config keys, no state files are owned by F25 (it reads `[usage] enabled` and `[providers.<id>].enabled`; it never writes).

## 2. Exported API (`pkg/whichmodel`)

```go
// StatusEntry is one provider's credential status.
type StatusEntry struct {
    Provider    string     `json:"provider"`
    Status      string     `json:"status"` // "ok" | "expired" | "missing"
    Source      *string    `json:"source"` // nil when missing
    ExpiresAt   *time.Time `json:"expires_at,omitempty"` // nil when missing; set when expired
    Fingerprint *string    `json:"fingerprint"`          // "a1b2c3…9f0e"; nil when missing
    Account     string     `json:"account,omitempty"`    // only with --show-identity
}

// AuthStatusReport is the status --json document root (F25 CONTRACTS §6).
type AuthStatusReport struct {
    SchemaVersion string        `json:"schema_version"` // "2.0"
    UsageEnabled  bool          `json:"usage_enabled"`  // always true when emitted
    Providers     []StatusEntry `json:"providers"`
}

// RunAuthStatus resolves and renders per-provider credential status.
func RunAuthStatus(args AuthStatusArgs, stdout, stderr io.Writer) error

// RunAuthLogin runs an interactive login for one provider.
func RunAuthLogin(provider string, stdout, stderr io.Writer, stdin io.Reader) error

// RunAuthLogout removes which-model-managed credential material for one provider.
func RunAuthLogout(provider string, yes bool, stdout, stderr io.Writer, stdin io.Reader) error

// Fingerprint returns the redacted credential fingerprint: sha256(secret) hex,
// rendered as first6…last4. Pure function.
func Fingerprint(secret string) string
```

`AuthStatusArgs` = `{Providers []string; All bool; ShowIdentity bool; JSON bool; ConfigPath string}` (filler type defined in `auth.go`; `All` mirrors `usage --all` expansion).

## 3. Flags owned

| Subcommand | Flag | Type | Default | Meaning |
|---|---|---|---|---|
| `logout` | `--yes` | bool | `false` | Skip the confirmation prompt |

Consumed global flags: `--json`, `--show-identity`, `--no-usage`. No other per-subcommand flags.

## 4. Config keys read (F01-owned)

| Key | Read via | Used for |
|---|---|---|
| `usage.enabled` | `cfg.UnmarshalKey("usage.enabled", &v)` | L1 disabled refusal (SPEC §2.13) |
| `providers.<id>.enabled` | `cfg.UnmarshalKey("providers.<id>.enabled", &v)` | `status` with no args (SPEC §2.1) |

## 5. Error codes and exit codes

Command-level code strings on the failure line (`which-model auth <sub>: [<code>] <message>`): `arguments` (exit-2 argument errors — `UsageError`), `usage_disabled` (L0/L1 refusal), `unsupported` (login for a non-device-flow provider), `runtime` (logout removal failure). F25 `status` exit 5 uses the canonical codes `expired_credential` (any expired) or `login_required` (missing only) — both map to exit 5 via F22's global table. No new `Failure.Code` values are added (global CONTRACTS §1.6 is closed).

| Subcommand | Exit | Condition |
|---|---|---|
| `status` | 0 | every queried provider `ok` |
| `status` | 5 | ≥1 queried provider `missing` or `expired` |
| `status` | 2 | unknown provider; bad flags; usage disabled |
| `login` | 0 | device flow started/handed off successfully |
| `login` | 2 | unknown provider; non-TTY or `WHICH_MODEL_NONINTERACTIVE=1`; unsupported provider; usage disabled |
| `logout` | 0 | removed; prompt declined (`aborted`); nothing to remove |
| `logout` | 1 | removal runtime error |
| `logout` | 2 | unknown provider; non-TTY without `--yes`; usage disabled |

## 6. JSON shape (`which-model auth status --json`)

```json
{
  "schema_version": "2.0",
  "usage_enabled": true,
  "providers": [
    {"provider": "claude", "status": "ok", "source": "oauth", "expires_at": "2026-09-01T00:00:00Z", "fingerprint": "a1b2c3…9f0e"},
    {"provider": "copilot", "status": "missing", "source": null, "fingerprint": null}
  ]
}
```

- `expires_at` is omitted entirely when nil (missing); `source` and `fingerprint` are explicit `null` when missing (schema-stable field positions).
- `account` appears only with `--show-identity`.
- `usage_disabled_reason` never appears (auth refuses with exit 2 when disabled).
- On nonzero exit in text mode, stdout is empty (SPEC §2.14) — EXCEPT `status` exit 5/1-with-report, where the report is the primary deliverable and `RunAuthStatus` returns `&ReportedError{Err: ...}` so F22 never replaces it with the JSON error document. `login` prompts likewise count as primary output.

## 7. Text layout spec

`status` text: one line per provider via `text/tabwriter` with `padding = 2`; columns in order `provider`, `status`, `source`, `fingerprint`, `expiry`; `-` for absent source/fingerprint/expiry; `(expires <RFC3339>)` / `(expired <RFC3339>)` for expiry; `run: which-model auth login <provider>` appended as the last column when `missing`; `(account <login>)` appended with `--show-identity`. Example:

```
claude   ok       oauth   a1b2c3…9f0e   (expires 2026-09-01T00:00:00Z)
codex    ok       oauth   f6a4e1…77ab
copilot  missing  -       -            -    run: which-model auth login copilot
```

`login` stdout: `Open <verification_uri> and enter code <code>.` (exactly one line); `waiting for confirmation...` on stderr.

`logout` prompt (stdout): `Remove which-model's cached credential for <provider>? [y/N] `; declined → stderr `aborted`.

## 8. Imported contracts (consumed upstream)

### 8.1 F22 `pkg/whichmodel` command wiring + exit signalling (pinned; canonical owner: `specs/features/F22-cli-skeleton/CONTRACTS.md`)

```go
package whichmodel

type GlobalFlags struct {
    JSON, Text, Quiet, NoColor, Offline, RefreshUsage, RefreshBenchmarks,
    RefreshScores, Refresh, NoUsage, ShowIdentity, Schema, Version bool
    MaxAge, Timeout time.Duration // Timeout default 10s
    Verbose int
    ConfigPath, Normalizer, Aggregator string
}
var Global GlobalFlags

func (g *GlobalFlags) Bind(cmd *cobra.Command) error
func (g *GlobalFlags) Normalize() error
func (g *GlobalFlags) Validate() error

type UsageError struct{ Message string } // exit 2, code "arguments"
type CodedError struct{ Code, Message string } // code→exit via global §1.6 table; unknown code → 1
type ReportedError struct{ Err error } // marker: deliverable already written to stdout;
// ExecuteArgs renders the stderr failure line + exit code, but NEVER the --json
// error document (that renders only for UsageError/CodedError/plain errors).
func ExitCodeFor(err error) int
func CodeFor(err error) string
func RegisterExitCode(code string, exit int)

func Execute() int
func ExecuteArgs(args []string) int
func NewRootCmd() *cobra.Command

func RegisterSchema(cmdPath string, doc map[string]any)
func SchemaIndex() []string

func register(factory func() *cobra.Command) // unexported; F25 calls from init() only
func registeredCommands() []*cobra.Command
```

- `pkg/whichmodel/auth_cmd.go` = `func init() { register(NewAuthCmd) }` + `func NewAuthCmd() *cobra.Command`. F25 never calls `AddCommand` or `os.Exit`; F22's `ExecuteArgs` maps the returned error via `ExitCodeFor` and renders the failure line via F03 `output.WriteFailure` (stderr; JSON error document on stdout in `--json` mode). F25 RunE returns errors only (SPEC Decision D-10).
- F25 registers no extra exit codes; all its codes are in F22's table (`arguments`→2, `usage_disabled`→2, `login_required`/`expired_credential`→5, `runtime`/unknown→1).

### 8.2 F12 `internal/usage/credential` (canonical owner: `specs/features/F12-credentials/CONTRACTS.md`)

```go
package credential

type Resolved struct {
    Source    usage.Source // which resolver produced the credential
    Secret    string       // the opaque token
    ExpiresAt *time.Time   // nil = no known expiry
    Account   string       // login/account identity if resolvable
    Path      string       // filesystem path when file-sourced ("" otherwise)
    FileMode  fs.FileMode  // mode of the source file (zero when not file-sourced)
}

// ResolveFirst tries the provider descriptor's ordered AuthSource chain and
// returns the first usable credential. Missing credential → ErrNoCredential.
func ResolveFirst(providerID string) (Resolved, error)

// Remove deletes ONLY credential material which-model itself wrote (its own
// cache entries / keychain items). Provider-native stores are never touched.
func Remove(providerID string) error

// DeviceFlow is an interactive device-code flow (Copilot).
type DeviceFlow struct {
    Code           string // e.g. "WXYZ-1234"
    VerificationURI string // e.g. "https://github.com/login/device"
}

// StartDeviceFlow begins the flow and returns immediately with the prompt
// fields; Wait blocks until confirmation or deadline.
func StartDeviceFlow(providerID string) (DeviceFlow, error)

var ErrNoCredential = errors.New("no usable credential")
```

Expected files: `internal/usage/credential/*.go`. F25 consumes `ResolveFirst`, `Remove`, `StartDeviceFlow`/`DeviceFlow` only.

### 8.3 F05 `internal/security` (pinned surface)

- `func HasBroadPermissions(mode fs.FileMode) bool` — used for the logout warning (SPEC §2.11).
- `func WithCanary(canary string, fn func() error) error` — canary harness for redaction tests.

### 8.4 F11 `internal/usage` registry

- `func All() []Descriptor`, `func Get(id string) (Descriptor, bool)` (`internal/usage/registry.go`) — provider validation (SPEC §2.1) and `AuthSources` display (SPEC §3).

### 8.5 F01 `internal/config`

- `func Load(path string) (*config.Config, error)` and `func (c *Config) UnmarshalKey(key string, out any) error` — `usage.enabled`, `providers.<id>.enabled` (SPEC §2.13).

## 9. Security invariants (this feature)

- The token never appears in any output under any flag; the only derived form ever shown is `Fingerprint` (SPEC §2.4; canary-tested per global SPEC §6.5).
- Account identity appears only with `--show-identity` (global SPEC §6.7).
- Logout never remediates permissions — warns only (global SPEC §6.6).
- Logout never touches provider-native credential stores (SPEC §2.10).
- Login refuses unattended contexts outright (SPEC §2.7); the device code appears only in the primary prompt line.
