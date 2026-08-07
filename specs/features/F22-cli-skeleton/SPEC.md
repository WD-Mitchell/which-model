---
kind: feature-spec
version: "1.0"
feature: F22-cli-skeleton
project: which-model
---

# F22 — CLI Skeleton

Depends on: F01, F03

## Purpose

F22 builds the executable `which-model` binary skeleton: the `pkg/whichmodel` command
registry (Main DECISION A), the root command with the global persistent flags from
annex-d §1.2, the exit-code mapping from global SPEC §5, and the core commands owned by
the skeleton itself (`version`, `schema`, `serve` placeholder, `config`). Later
command features (F23–F27, F30) register their commands through the registry without
editing F22's files, so F22's surface is the contract every command feature consumes.

## Behaviour

1. **Binary entrypoint** (`cmd/which-model/main.go`): `func main() { os.Exit(whichmodel.Execute()) }`. All behaviour lives in `pkg/whichmodel`; `Execute()` calls `ExecuteArgs(os.Args[1:])` and `ExecuteArgs` is the testable entrypoint (annex-d §1.1).

2. **Command registration** (Main DECISION A): `pkg/whichmodel/registry.go` holds
   `func register(factory func() *cobra.Command)` and `func registeredCommands() []*cobra.Command`.
   Registrars accumulate via `init()` in feature-owned files; `registeredCommands` builds the
   command set once (`sync.Once`) and orders it by the explicit
   `commandOrder = ["usage","catalog","pick","routes","auth","schema","skills","hooks","explain","serve","config","version"]`
   (commands not in the list sort last, then by name). Every command feature creates exactly one
   file `pkg/whichmodel/<name>_cmd.go` containing `func init() { register(New<X>Cmd) }` plus the
   exported constructor `New<X>Cmd() *cobra.Command`; subcommands attach inside that file.
   F22 itself registers `version`, `schema`, `serve`, `config` (order positions 10, 5, 11, 12).

3. **Root command** (`pkg/whichmodel/root.go`): `NewRootCmd()` sets `Use: "which-model"` (fixed —
   the binary name is never read from `argv[0]`, annex-d §1.1a), `SilenceErrors: true`,
   `SilenceUsage: true` (the failure line is F03's `output.WriteFailure`), `SetFlagErrorFunc`
   mapping cobra flag errors to `UsageError` (exit 2), the default completion subcommand disabled
   (cobra's `CompletionOptions.DisableDefaultCmd = true`; flag completion remains active), and
   no `Run` (bare `which-model` prints help, exit 0). `ExecuteArgs` runs the
   `--version`/`--schema` short-circuit pre-scan (§8–§9 below), then `Global.Normalize()` and
   `Global.Validate()` before `root.Execute()`.

4. **Global flags** (`pkg/whichmodel/flags.go`): the full annex-d §1.2 set plus `--text`
   (F22 addition, the explicit inverse of `--json`). `GlobalFlags.Bind(cmd)` registers the
   persistent flags on the root; `Global` is the package-level singleton every feature reads.
   `Normalize()` maps `--refresh` to `RefreshUsage|RefreshBenchmarks|RefreshScores = true`
   (annex-d §1.6 rule 5). `Validate()` rejects the contradictory sets (annex-d §1.6 rule 4 plus
   `--text`): `--json`+`--text`, `--offline`+`--refresh`, `--offline`+`--refresh-benchmarks`
   (exit 2, code "arguments"); `--offline`+`--refresh-scores` is allowed (Derive is offline-safe).
   Defaults: `--timeout` 10s (annex-d §1.2 DefaultTimeoutSec), `--normalizer`
   "minmax-linear", `--aggregator` "weighted-arithmetic-mean" (annex-b §4.0).

5. **Exit codes** (`pkg/whichmodel/exitcode.go`): `ExitCodeFor(err error) int` maps per global
   SPEC §5 and global CONTRACTS §1.6: `*UsageError` → 2 (code "arguments"); `*CodedError` →
   the §1.6 table (unauthorized/expired_credential/login_required/credential_file/credential_json/
   unsafe_credential/access_denied/device_expired/cookie_unavailable/signing_failed → 5;
   usage_disabled/usage_compiled_out → 2; every other code → 1); `*httpkit.Error` (F04) →
   the same §1.6 table keyed by its `Code` field (401 AA failures arrive as
   `Error{Code: "unauthorized", StatusCode: 401}` → exit 5); any error exposing
   `interface{ ExitCode() int }` → that value (F01's `ConfigError.ExitCode() == 2`); any other
   error → 1 (code "error"). `RegisterExitCode(code string, exit int)` is the extension point
   (F26 registers 3/4 for no-viable-candidate/band-gated). `CodeFor(err)` returns the failure
   code string. `UsageError{Message}` and `CodedError{Code, Message}` are the constructible
   error types command features use. `ReportedError{Err}` marks a failure whose deliverable
   has already been written to stdout by the failing command (F25 `auth status`,
   F27 verify) — it is rendered as the text failure line only. Unknown commands and unknown
   subcommands (cobra's `unknown command "x" for "which-model …"` error) are wrapped as
   `UsageError` (exit 2, code "arguments") by `ExecuteArgs` before mapping. Error message
   text is never matched (F04 sanitizes `Error()` output; codes are the only contract).

6. **Failure output**: `ExecuteArgs` renders the failure line via F03's
   `output.WriteFailure(w, "which-model", code, message)` and the exit code via `ExitCodeFor`.
   With `--json` (or `--text` suppressed by `--json`), the error document
   `{"schema_version":"2.0","error":{"code":"<code>","message":"<message>"}}` is written to
   stdout (annex-d §1.3) for `UsageError`, `CodedError`, and plain errors; `ReportedError`
   NEVER emits the JSON error document (the command already delivered its stdout payload).
   The text failure line goes to stderr in all cases. Warnings use F03's
   `output.WriteWarning` (stderr, `warning: <message>`).

7. **Version command** (`pkg/whichmodel/version_cmd.go`): `NewVersionCmd()` registered by F22;
   `--version` and `version` are equivalent. Output text:
   `which-model <version> (commit <commit>, built <built_at>)` where the three values come from
   ldflags `-X github.com/WD-Mitchell/which-model/pkg/whichmodel.Version|Commit|BuildDate`
   (defaults `"dev"`, `"unknown"`, `"unknown"`). JSON (with `--json`):
   `{"version": ..., "commit": ..., "built_at": ...}` inside the F03 envelope. The
   usage-state suffix (`...; usage <enabled|compiled-out>`) is deferred to F21
   (usage-toggle) which extends the version command output; the provider LastVerified table
   is out of scope for the skeleton (annex-d §2.8 lists it under the provider registry, M3).

8. **`--schema` short-circuit**: because cobra validates `Args` before `PersistentPreRunE`,
   the `--schema` hook cannot live in a pre-run callback without breaking on commands with
   required arguments. `ExecuteArgs` therefore pre-scans the raw args (before any `--`
   terminator) for the literal `--schema` or `--schema=true` token; when present, the token is
   removed, the remaining args are resolved via `root.Find`, and the schema document for the
   found command path is printed (F03 `output.PrintSchema`) with exit 0 — the command itself
   is not executed (annex-d §2.9). `--schema` on a path with no registered schema → exit 2
   (code "arguments"). `--version`/`--version=true` short-circuits the same way (§7), with
   `--version=false` ignored.

9. **Schema command and registry** (`pkg/whichmodel/schema_cmd.go`): `NewSchemaCmd()` is an
   exported, registered command (order position 5). `RegisterSchema(cmdPath string, doc map[string]any)`
   records the JSON Schema document for a command's `--json` output; `SchemaIndex()` lists
   registered paths. `schema` with no argument prints the index (F03 `output.PrintSchemaIndex`);
   `schema <command path>` prints that command's document. F22 registers documents for
   `version` and `config show`; all other commands are registered by their owning features.
   Unknown path → exit 2.

10. **Serve placeholder** (`pkg/whichmodel/serve_cmd.go`): `newServeCmd()` (unexported — `serve`
    is not in the fixed constructor list; registered via `register` in F22's file) exposes
    `--warm` (bool), `--interval` (duration, default 5m), `--listen` (string, default `:8099`).
    The body returns `CodedError{Code: "serve_unavailable", Message: "serve is not available in
    this build; it requires the usage cache subsystem (F13) which lands in a later milestone"}`
    → exit 1. Per SddCliCommands' confirmation, no feature in M2 owns serve; the refusal body
    stays until the DAG assigns serve (F28/F29/F30 candidates).

11. **Config commands** (`pkg/whichmodel/config_cmd.go`, `NewConfigCmd()` registered, order
    position 12; handoff confirmed by the F01 author): `config show|set|path|validate`.
    - `config show`: prints the fully resolved config as TOML (text) via F01's
      `(*Config).MarshalTOML()` (the merged document incl. env overlay; requested from F01);
      with `--json`, the TOML render is decoded to a map and emitted as
      `{"schema_version":"2.0","usage_enabled":..., <sections>..., "_sources": {...}}` where
      `_sources` = the four `config.ResolvePaths` values plus `explicit_config` when `--config`
      is given. Per-key layer provenance is deferred (see Decisions).
    - `config set <key> <value>`: writes a dotted TOML key into the user config file
      (the `--config` path if given, else the resolved user file; created if absent, dirs
      created) by decoding the file into a `map[string]any`, setting the nested key, and
      atomically replacing (temp file + rename). The value is parsed as a TOML literal:
      integer → int64, float → float64, bool → bool, else string. Empty/blank key, empty
      segment, or setting an existing array key → exit 2. Other keys in the file are preserved.
    - `config path`: prints the resolved user config file path (or the `--config` path when
      set) on one line, exit 0.
    - `config validate`: `config.Load(LoadOptions{Path: Global.ConfigPath})` plus F22's own
      `[output]` section validation (see Behaviour 12); on error prints the message and exits 1
      (annex-d §2.7 — the one deliberate deviation from the generic `ConfigError` → exit 2
      mapping); success prints `config is valid`, exit 0. Sections owned by other features
      ([catalog], [scoring], [bands], [strategy], [catalog.publish]) are validated at command
      use time by their owners, not by `config validate` (see Decisions).

12. **`[output]` section schema** (F01 DECISION B correction assigns `[output]` to F22/F03):
    F22 owns `pkg/whichmodel/output_config.go`: `type OutputConfig struct { Color string;
    Timestamps string; IdentityDefault bool }` (toml tags `color|timestamps|identity_default`),
    defaults `color="auto"`, `timestamps="rfc3339"`, `identity_default=false`, loaded via
    `cfg.UnmarshalKey("output", &x)` (decode-into-defaults; unknown keys → `ConfigError`,
    exit 2). `config validate` exercises it; F03 renderers consume colour decisions.

## Error behaviour

| Condition | Code | Code string | Output |
|---|---|---|---|
| Flag parse error (cobra `SetFlagErrorFunc`) | 2 | arguments | failure line + error doc |
| Contradictory flags (`--json`+`--text`, `--offline`+`--refresh`, `--offline`+`--refresh-benchmarks`) | 2 | arguments | failure line + error doc |
| Unknown command / subcommand | 2 | arguments | cobra suggestion text, failure line |
| `--schema`/`schema <path>` for unknown or schema-less path | 2 | arguments | failure line + error doc |
| Invalid `--config` file (F01 `ConfigError`) | 2 | config | failure line + error doc |
| `config set` bad key / array-key write | 2 | arguments | failure line + error doc |
| `config validate` finds errors | 1 | error | message (annex-d §2.7) |
| `config` I/O failure (unreadable file, unwritable dir) | 1 | error | failure line |
| `serve` (placeholder) | 1 | serve_unavailable | failure line + error doc |
| `--version` / `--schema` short-circuit | 0 | — | version line / schema doc |
| Bare `which-model` | 0 | — | help text |
| Any other runtime error | 1 | error | failure line + error doc |

Exit 0 success, 2 argument/config, 1 runtime; 3/4/5 are reserved for provider-evaluating
features (registered via `RegisterExitCode`).

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Cobra pin | `github.com/spf13/cobra` latest stable 1.x (v1.8.1 verified baseline) | annex-d §1.1 mandates cobra; v1 API stable |
| D2 | Aliases | install-time symlinks `wm`/`wmodel`/`whichm`; `argv[0]` never inspected; `Use` fixed `"which-model"` | annex-d §1.1a: byte-identical output under any name |
| D3 | Registration | `register()`/`registeredCommands()` + `commandOrder` in `pkg/whichmodel/registry.go` | Main DECISION A (binding) |
| D4 | `--text` flag | new persistent bool; `--json`+`--text` → exit 2 | explicit inverse of `--json`; no annex-d precedent |
| D5 | `--refresh` expansion | `Normalize()` sets all three refresh flags; validation rejects `--offline`+`--refresh` | annex-d §1.6 rules 4–5 |
| D6 | `--schema`/`--version` | raw-arg pre-scan in `ExecuteArgs`, short-circuit before cobra executes | cobra validates Args before PersistentPreRunE; pre-scan keeps the hook correct for arg-requiring commands |
| D7 | serve placeholder | `serve_unavailable` refusal, exit 1; body unchanged until DAG assigns serve | SddCliCommands confirmed no M2 owner |
| D8 | version output | core line only; usage-state suffix deferred to F21 | F21 owns usage toggle output |
| D9 | config commands owned by F22 | `config show\|set\|path\|validate` | explicit handoff from F01 author |
| D10 | `config show --json` | TOML render decoded to map + `_sources` (four resolved paths + optional explicit_config); per-key provenance deferred | F01 keeps merged doc internally; per-key layer tracking needs F01 API not yet pinned |
| D11 | `config validate` exit 1 | 1 on any error (annex-d §2.7), not 2 | annex-d §2.7 explicit; deviation recorded |
| D12 | `[output]` schema owned by F22 | `OutputConfig` + defaults via `cfg.UnmarshalKey("output", &x)` | F01 DECISION B correction |
| D13 | Completion subcommand | disabled (flag completion stays) | golden help text stability; no annex-d requirement |
| D14 | `config set` value typing | TOML literal parse: int/float/bool/else string | deterministic, lossless for the common cases |
| D15 | `config show` rendering | F01 confirmed `MarshalTOML` (fully resolved document, canonical section order, env overlay applied); `config show --json` decodes that TOML into a map and wraps it in the envelope with `_sources` | F01 CONTRACTS §1.8 pin; no fallback path needed |

## Out of scope

- Command implementations for usage/catalog/pick/routes/auth/skills/hooks/explain (F23–F27; they register through the registry).
- The `serve` runtime body (needs F13 usage cache subsystem; M2+ owner per D7).
- The usage-state suffix in version output (F21) and the provider LastVerified table (M3).
- Per-key config-layer provenance in `_sources` (D10).
- Validation of foreign config sections in `config validate` (D12; owners validate at use time).
- Shell completion generation for the aliases (flag completion only, D13).
