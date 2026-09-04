---
kind: feature-contracts
version: "1.0"
feature: F22-cli-skeleton
project: which-model
---

# F22 — Contracts

## Packages

| Package | Path | Status |
|---|---|---|
| `whichmodel` | `pkg/whichmodel/` | F22-owned |
| `main` | `cmd/which-model/main.go` | F22-owned |

F22 consumes (verbatim): `config` (`internal/config`, F01), `output` (`internal/output`, F03).
F22 does not add `internal/` packages.

## Exported API — package `whichmodel` (`pkg/whichmodel/`)

### Entrypoints (`root.go`)

```go
func Execute() int                 // ExecuteArgs(os.Args[1:]); main() calls os.Exit(Execute())
func ExecuteArgs(args []string) int
func NewRootCmd() *cobra.Command   // Use "which-model"; SilenceErrors+SilenceUsage; SetFlagErrorFunc -> UsageError; completion subcommand disabled; no Run
```

`ExecuteArgs` contract: raw-arg pre-scan for `--schema`/`--version` (before the `--`
terminator; `--version=false` ignored) short-circuits before cobra executes; any other
execution error is wrapped: cobra's unknown-command error (`unknown command "x" for
"which-model …"`) → `*UsageError`, anything else passes through unchanged. The failure line
is rendered via `output.WriteFailure` to stderr; the `--json` error document goes to stdout
unless the error is a `*ReportedError` (whose stdout payload the command already wrote).

### Global flags (`flags.go`)

```go
type GlobalFlags struct {
    JSON              bool          // --json
    Text              bool          // --text (F22 addition; inverse of --json)
    MaxAge            time.Duration // --max-age
    Timeout           time.Duration // --timeout; default 10s (annex-d DefaultTimeoutSec)
    Quiet             bool          // --quiet
    Verbose           int           // --verbose (count)
    NoColor           bool          // --no-color
    Offline           bool          // --offline
    ConfigPath        string        // --config; feeds config.LoadOptions.Path
    RefreshUsage      bool          // --refresh-usage
    RefreshBenchmarks bool          // --refresh-benchmarks
    RefreshScores     bool          // --refresh-scores
    Refresh           bool          // --refresh (expanded by Normalize)
    NoUsage           bool          // --no-usage
    ShowIdentity      bool          // --show-identity
    Schema            bool          // --schema (pre-scan; see root.go)
    Version           bool          // --version (pre-scan)
    Normalizer        string        // --normalizer; default "minmax-linear"
    Aggregator        string        // --aggregator; default "weighted-arithmetic-mean"
}

var Global GlobalFlags

func (g *GlobalFlags) Bind(cmd *cobra.Command) error // registers every persistent flag above on cmd
func (g *GlobalFlags) Normalize() error              // --refresh -> RefreshUsage|RefreshBenchmarks|RefreshScores = true
func (g *GlobalFlags) Validate() error               // UsageError on: JSON&&Text, Offline&&Refresh, Offline&&RefreshBenchmarks
```

### Errors and exit mapping (`exitcode.go`)

```go
type UsageError struct{ Message string } // exit 2; code "arguments"
type CodedError struct{ Code, Message string } // code per global CONTRACTS §1.6; unknown code -> exit 1
// ReportedError marks a failure whose deliverable already went to stdout (F25 auth status,
// F27 verify): ExecuteArgs renders the stderr failure line only, NEVER the JSON error document.
type ReportedError struct{ Err error }

func (e *UsageError) Error() string
func (e *CodedError) Error() string
func (e *ReportedError) Error() string // returns e.Err.Error()
func (e *ReportedError) Unwrap() error // returns e.Err

func ExitCodeFor(err error) int // *UsageError->2; *CodedError->§1.6 table; *httpkit.Error->§1.6 table by Code; ExitCode() int interface (F01 ConfigError -> 2); else 1
func CodeFor(err error) string  // "arguments" | "config" | "error" | *CodedError.Code | *httpkit.Error.Code
func RegisterExitCode(code string, exit int) // extension point; F26 registers 3/4 (no_viable_candidate, band_gated)
```

`*httpkit.Error` (F04) is mapped by its `Code` field against the same global CONTRACTS §1.6
table (401 AA failures arrive as `unauthorized` → exit 5); message text is never matched
(F04 sanitizes `Error()`).

Failure line rendering is F03-owned: `output.WriteFailure(w, "which-model", code, message)`
→ `which-model <command>: [<code>] <message>`. JSON error document (stdout, when `--json`):

```json
{"schema_version":"2.0","error":{"code":"arguments","message":"..."}}
```

### Registry (`registry.go`) — Main DECISION A

```go
func register(factory func() *cobra.Command)           // unexported; called from init() in feature-owned files
func registeredCommands() []*cobra.Command             // built once (sync.Once); order per commandOrder

var commandOrder = []string{
    "usage", "catalog", "pick", "routes", "auth",
    "schema", "skills", "hooks", "explain", "serve",
    "config", "version",
}
```

Every command feature creates exactly one file `pkg/whichmodel/<name>_cmd.go`:

```go
func init() { register(New<X>Cmd) }   // NewUsageCmd, NewCatalogCmd, NewPickCmd, NewRoutesCmd,
                                      // NewAuthCmd, NewSchemaCmd, NewSkillsCmd, NewHooksCmd,
                                      // NewExplainCmd (fixed names); subcommands attach inside the file
func New<X>Cmd() *cobra.Command
```

Commands not in the fixed constructor list (e.g. `serve`, `completion`) are registered
via `register(new<X>Cmd)` from an F22-owned file.

### Schema (`schema_cmd.go`)

```go
func RegisterSchema(cmdPath string, doc map[string]any) // cmdPath e.g. "version", "config show"
func SchemaIndex() []string
func NewSchemaCmd() *cobra.Command // registered, order position 5
```

### Version (`version_cmd.go`)

```go
var Version   = "dev"     // ldflags: -X github.com/WD-Mitchell/which-model/pkg/whichmodel.Version
var Commit    = "unknown" // ldflags: -X .../pkg/whichmodel.Commit
var BuildDate = "unknown" // ldflags: -X .../pkg/whichmodel.BuildDate

func VersionLine() string            // "which-model <version> (commit <commit>, built <built_at>)"
func VersionJSON() map[string]string // {"version":..., "commit":..., "built_at":...}
func NewVersionCmd() *cobra.Command  // registered, order position 12
```

### Config commands (`config_cmd.go`, `output_config.go`)

```go
func NewConfigCmd() *cobra.Command // registered, order position 11; subcommands show|set|path|validate

type OutputConfig struct { // F22-owned [output] section schema (F01 DECISION B)
    Color           string `toml:"color"`           // "auto" | "always" | "never"; default "auto"
    Timestamps      string `toml:"timestamps"`      // "rfc3339" | "none"; default "rfc3339"
    IdentityDefault bool   `toml:"identity_default"` // default false
}
func DefaultOutputConfig() OutputConfig
func loadOutputConfig(cfg *config.Config) (OutputConfig, error) // cfg.UnmarshalKey("output", &x) into defaults
```

`config show` (text) = `cfg.MarshalTOML()`; `config show --json` = envelope + TOML-decoded
sections + `_sources`. `config set <key> <value>` = dotted TOML key write into the user
config file (atomic temp+rename). `config path` = resolved user config path.
`config validate` = `config.Load` + `[output]` validation; exit 1 on error (annex-d §2.7).

### Serve placeholder (`serve_cmd.go`)

```go
func newServeCmd() *cobra.Command // registered by F22; flags --warm (bool), --interval (5m), --listen (:8099)
```

Body: `CodedError{Code: "serve_unavailable", Message: "serve is not available in this build; it requires the usage cache subsystem (F13) which lands in a later milestone"}` → exit 1.

### Commands registered by F22 (tree at F22 completion)

```
which-model
├── version    (order 12)
├── schema     (order 5)
├── serve      (order 10; placeholder)
└── config     (order 11)
    ├── show
    ├── set
    ├── path
    └── validate
```

Root help lists commands in `commandOrder` order: `schema, serve, config, version`.
Later features append via `register()` (no edits to F22 files).

## Flags owned by F22

All root persistent flags (Behaviour 4 of `SPEC.md`): `--json --text --max-age --timeout
--quiet --verbose --no-color --offline --config --refresh-usage --refresh-benchmarks
--refresh-scores --refresh --no-usage --show-identity --schema --normalizer --aggregator
--version`. Subcommand flags: `--warm --interval --listen` (serve).

## Config keys owned by F22

`[output] color | timestamps | identity_default` (section schema per F01 DECISION B; defaults
in `DefaultOutputConfig`). Read-only: `--config` → `config.LoadOptions.Path`; the resolved
paths from `config.ResolvePaths` for `_sources` and `config path`.

## Error codes added by F22

| Code | Exit | Where |
|---|---|---|
| `arguments` | 2 | UsageError; flag/arg/contradiction errors; `--schema` on unknown/schema-less path; `config set` bad key |
| `config` | 2 | `config.Load` / `UnmarshalKey` failures (via `ConfigError.ExitCode()`) |
| `error` | 1 | generic runtime errors; `config` I/O failures; `config validate` (annex-d §2.7) |
| `serve_unavailable` | 1 | serve placeholder |

## JSON shapes emitted by F22

| Command | Shape |
|---|---|
| `version --json` | `{"version":"...","commit":"...","built_at":"..."}` + envelope |
| error doc | `{"error":{"code":"...","message":"..."}}` + envelope (stdout when `--json`) |
| `config show --json` | `{<toml sections as JSON>, "_sources": {user_config_file, config_dir, cache_dir, state_dir, explicit_config?}}` + envelope |
| `schema` docs | JSON Schema documents registered via `RegisterSchema` (F03 `PrintSchema`/`PrintSchemaIndex`) |

## Build contract

- Module `github.com/WD-Mitchell/which-model`; Go 1.23; binary `which-model`.
- Version injection: `-X github.com/WD-Mitchell/which-model/pkg/whichmodel.Version=<v>`,
  `-X .../Commit=<sha>`, `-X .../BuildDate=<date>`; unset → defaults above.
- Alias invariance: no code reads `os.Args[0]`; `Use` is fixed `"which-model"`; help/error
  output is byte-identical under symlinks `wm`, `wmodel`, `whichm`.
- Compiles under `-tags nousage` (usage-toggle stubs land in F21).

## Nested command execution correction (#162)

`ExecuteCommand` must build fresh command instances before dispatching a hook's underlying command and restore the caller's global flags/output streams on return. Cached Cobra commands cannot be reparented while the outer hook is still executing. The registry's inspection cache remains available; nested execution invalidates it before constructing a new tree.
