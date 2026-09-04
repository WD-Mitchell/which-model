---
kind: feature-contracts
feature: F29-agent-hooks
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F29 — Agent Hooks: Contracts

Source: `docs/plan/annex-c-agent-integration.md` §3 (hook inventory), §3.5 (variants), §4.7 (exit codes), §5 (evidence); `docs/plan/annex-d-cli-reference.md` §2.10; `specs/features/F29-agent-hooks/SPEC.md`.

## 1. `internal/hooks` (new package)

`internal/hooks/hooks.go` — registry:

```go
package hooks

// ID identifies a hook; Event/Matcher/Timeout mirror the Claude Code hook
// table (annex-c §3.1–§3.4); Timeout is in seconds.
type Hook struct {
    ID      string
    Event   string
    Matcher string
    Timeout int
    // Underlying builds the annexed command argv: defaults first, then the
    // caller-supplied passthrough args (later args win in cobra).
    Underlying func(passthrough []string, env map[string]string) []string
}

// All is the four-hook registry, in annex-c §3 order.
var All = []Hook{ /* usage-refresh, quota-guard, spawn-gate, model-audit */ }

func Get(name string) (Hook, bool)
```

`internal/hooks/run.go` — the decision-protocol core:

```go
package hooks

// Envelope is the sole stdout shape (SPEC behaviour 3). MarshalEnvelope is
// compact JSON + "\n".
type Envelope struct {
    Decision           string         `json:"decision"`
    Reason             string         `json:"reason,omitempty"`
    HookSpecificOutput map[string]any `json:"hookSpecificOutput"`
}

func MarshalEnvelope(e Envelope) []byte

// Runner executes the underlying command in-process. The CLI layer supplies
// pkg/whichmodel.ExecuteCommand as the default.
type Runner func(args []string, stdout, stderr io.Writer) int

// Options carries execution inputs. Stdin is a host JSON object; Runner
// is always invoked and is the only output-fixture seam (SPEC behaviour 4).
// Env overrides os.Environ for
// WHICH_MODEL_TASK_PROFILE / WHICH_MODEL_CANDIDATE_ID /
// WHICH_MODEL_DISPATCHED_MODEL. RepoRoot is the evidence-file base dir.
type Options struct {
    Runner   Runner
    Stdin    []byte
    Env      map[string]string
    RepoRoot string
}

// Run executes hook (SPEC behaviours 4–8). It returns the bytes to write to
// stdout (possibly empty = fail-open silence), or an error for exit-2-class
// conditions: unknown hook name, non-empty Stdin that is not a valid JSON object.
// Run NEVER returns an error for underlying command failures (fail-open).
func Run(name string, passthrough []string, opts Options) ([]byte, error)
```

`internal/hooks/config.go` — installed-config shapes (also the install/remove data model):

```go
package hooks

// Entry is one owned hook as recorded in the claude manifest and mirrored in
// settings.json.
type Entry struct {
    ID      string `json:"id"`
    Event   string `json:"event"`
    Matcher string `json:"matcher"`
    Timeout int    `json:"timeout"`
    Command string `json:"command"`
}

// Manifest is .claude/which-model-hooks.json.
type Manifest struct {
    Version         int     `json:"version"`
    CreatedSettings bool    `json:"created_settings"`
    Hooks           []Entry `json:"hooks"`
}

func LoadManifest(path string) (*Manifest, error)        // missing file → nil, nil
func SaveManifest(path string, m *Manifest) error        // 0600; JSON + "\n"
func SaveManifestClaude(m *Manifest, e []Entry) (*Manifest, error) // variant assembly

// claudeSettings holds the repo .claude/settings.json (generic map to
// preserve foreign keys; semantic round-trip only).
type claudeSettings map[string]any
func loadClaudeSettings(path string) (claudeSettings, error) // missing → empty map
func (s claudeSettings) mergeOwned(entries []Entry)           // replace/append only owned (event, matcher, command)
func (s claudeSettings) removeOwned(entries []Entry)
func (s claudeSettings) empty() bool
func saveClaudeSettings(path string, s claudeSettings) error // MarshalIndent 2-space + "\n", 0644
```

`internal/hooks/install.go` — install/remove:

```go
package hooks

// Variant mirrors SPEC behaviour 9.
type Variant int

const (
    VariantAuto Variant = iota // resolved by the CLI layer via usage.Enabled
    VariantUsage               // all four hooks
    VariantNoUsage             // spawn-gate + model-audit only
)

// Installed returns the owned entries for a variant, with exact commands
// (SPEC behaviour 9). Commands reference `which-model hooks run <id>`, plus
// variant-B passthrough args (--no-usage --profile balanced_implementation
// --quiet / --last).
func Installed(v Variant) []Entry

// Install merges the variant's entries into the target config (SPEC behaviour
// 10). target "claude" | "generic". Returns a human summary line per hook.
func Install(target string, entries []Entry, repoRoot string) ([]string, error)

// Remove deletes owned entries only (SPEC behaviour 11). Returns a human
// summary. Nothing installed → no-op success.
func Remove(target string, repoRoot string) ([]string, error)
```

## 2. CLI command owned (`pkg/whichmodel`)

`pkg/whichmodel/hooks_cmd.go` — `func NewHooksCmd() *cobra.Command` (DECISION A: `func init() { register(NewHooksCmd) }`; `hooks` already in F22's `commandOrder`).

```
which-model hooks install [--target claude|generic] [--usage|--no-usage] [--repo PATH]
which-model hooks remove  [--target claude|generic] [--repo PATH]
which-model hooks run <hook> [args...]
```

- `--target` default `claude`; unknown target → exit 2.
- `--usage`+`--no-usage` together → exit 2. Neither → variant resolved via F21 `usage.Enabled(cfg)` (see §5) at install time only.
- `run`: builds `Options{Runner: ExecuteCommand, RepoRoot: <resolved>}`, `ExecuteCommand` being F22's `func ExecuteCommand(args []string, stdout, stderr io.Writer) int`; underlying argv = `hook.Underlying(passthrough, env)`; writes `Run`'s bytes to stdout; errors → stderr + exit 2.
- Exit codes per SPEC error table: install/remove `0`/`1`/`2`; run `0`/`2`.

## 3. Files and JSON shapes owned

| Path | Shape |
|---|---|
| `.claude/settings.json` (merged, repo-local) | Claude Code hooks config: `{"hooks":{"<Event>":[{"matcher":"<M>","hooks":[{"type":"command","command":"which-model hooks run <id>…","timeout":<N>}]}]}}` merged into any existing map; foreign keys preserved |
| `.claude/which-model-hooks.json` | `Manifest` (§1): `{"version":1,"created_settings":bool,"hooks":[Entry…]}` |
| `agents/hooks.toml` (repo-local) | TOML between `# === which-model managed hooks (do not edit) ===` and `# === end which-model managed hooks ===`; each hook: `[[hooks]]` with `event` = `session_start`\|`pre_dispatch`\|`post_dispatch`, `command` = `which-model hooks run <id>…`, `timeout_ms`, `on_failure = "ignore"`, plus `inject_as = "context.which_model_quota_guard"` (quota-guard) / `"context.which_model_pick"` (spawn-gate) |
| `<repoRoot>/.which-model/evidence.jsonl` | one line per dispatch: the sanitized, compact F26 explain object (annex-c §4.3: `schema_version`, `candidate`, `evidence`) |
| `<repoRoot>/.which-model/audit-mismatches.jsonl` | one line per mismatch: `{"ts":"<RFC3339 UTC>","dispatched_model":"…","route_model_id":"…","evidence":{…}}` |
| stdout of `hooks run` | `Envelope` JSON + `\n` (or empty) |

## 4. Envelope shapes per hook (golden)

- `usage-refresh` success: `{"decision":"approve","reason":"usage cache refreshed","hookSpecificOutput":{}}`
- `quota-guard` ≥1 critical: `{"decision":"block","reason":"quota guard: 2 provider(s) at or above critical band","hookSpecificOutput":{"critical_providers":["claude","codex"]}}`
- `quota-guard` none: `{"decision":"approve","reason":"no providers at or above critical band","hookSpecificOutput":{}}`
- `spawn-gate` pick 0: `{"decision":"approve","reason":"dispatch approved: <id>","hookSpecificOutput":{"candidate":{…annex-c §4.2 first candidate verbatim…}}}`
- `spawn-gate` pick 4: `{"decision":"block","reason":"all eligible providers band-gated: <comma-joined names>","hookSpecificOutput":{"excluded_candidates":[…verbatim…]}}`
- `spawn-gate` pick 1/2/3/5: `{"decision":"approve","reason":"fail-open: spawn-gate underlying command exited <N>","hookSpecificOutput":{}}`
- `model-audit` explain 0: `{"decision":"approve","reason":"dispatch evidence recorded","hookSpecificOutput":{"evidence_logged":"<repoRoot>/.which-model/evidence.jsonl","mismatch":false}}` (mismatch → `"mismatch":true` + `audit-mismatches.jsonl` line)
- `model-audit` explain 1/2/3/5: `{"decision":"approve","reason":"fail-open: model-audit underlying command exited <N>","hookSpecificOutput":{}}`

## 5. Cross-feature references (pinned)

- F22 `pkg/whichmodel/registry.go`: `register(func() *cobra.Command)`, `commandOrder` (already includes `hooks`); F22 `pkg/whichmodel.ExecuteCommand(args []string, stdout, stderr io.Writer) int` — the `Runner` default.
- F21 `internal/usage/toggle` package: `func Enabled(cfg *config.Config) (config.UsageEnabled, string)` — variant detection at install time (CLI layer only).
- F26 `pkg/whichmodel/pick_cmd.go` + `explain_cmd.go` — underlying `pick`/`explain`; `explain --json` root carries `schema_version`, `candidate` (string `provider:model_id`), `evidence` per `docs/plan/annex-c-agent-integration.md §4.3`; `pick` exit 4 = all band-gated.
- F24 `pkg/whichmodel/usage_cmd.go` — underlying `usage --all --json --quiet --refresh-usage --timeout 5s` and `usage --all --json --band-at-or-above critical --quiet` (annex-c §3.1/§3.4).
- F28 `internal/skills.RepoRoot()` — repo-root resolution (`--repo` override honored).
- Compiles under `-tags nousage`: `internal/hooks` imports stdlib + `internal/skills` only; `pkg/whichmodel/hooks_cmd.go` guards `usage.Enabled` behind the F21 stub contract.

## 6. Config keys / flags owned

- Config keys: none.
- Flags: `--target` (claude|generic, default claude), `--usage`, `--no-usage`, `--repo <path>` on install/remove; positional `<hook> [args...]` on run.

## 7. Error codes added

None (uses the fixed 0/1/2 set; no new `Failure.Code` values — `specs/global/CONTRACTS.md §1.6`).

## Review corrections (#162, #163)

`ExecuteCommand` builds fresh command instances and restores outer global flags/output streams. Runtime stdin never supplies command results. Model audit selects `--last` unless an explicit `--pick-id` is supplied; `WHICH_MODEL_CANDIDATE_ID` is a correlation check only. Evidence is decoded through the documented F26 fields, compacted to one JSONL line, and model ID is the suffix after the first colon. Invalid/mismatched evidence produces the established fail-open envelope and no write.

Explicit global flags before `hooks run` are parsed before the hook name and forwarded to the underlying command. Arguments after the hook name override them. JSON remains required for underlying machine output; outer text/JSON rendering flags do not alter hook protocol. A regression covers outer offline/config/timeout and later timeout override.


## Audit validation correction — #163 review

Before creating any audit file, validate the required F26 evidence profile,
score-input map, route provenance, and exclusions array, including enum values
and optional age/date/band bounds. Missing or null required fields fail open
without writing a plausible but incomplete audit record. Empty maps and arrays
remain valid. Pin `TestAuditRejectsIncompleteEvidence`.
