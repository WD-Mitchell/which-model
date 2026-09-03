---
kind: feature-contracts
feature: F28-agent-skills
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F28 — Agent Skills: Contracts

Source: `docs/plan/annex-c-agent-integration.md` §2 (skills), §4.4–§4.7 (schema command, exit codes), §5 (evidence), §6 (interface descriptors); `docs/plan/annex-d-cli-reference.md` §2.9 (schema command), §2.1/§2.4/§2.5 (usage/pick/explain surfaces), §5 (migration table); `docs/plan/README.md` M6.

## 1. Skill artifact paths (authored, committed)

| File | Content |
|---|---|
| `skills/model-selection/SKILL.md` | Skill content per SPEC behaviour §5 (see `docs/plan/annex-c-agent-integration.md §2.1`) |
| `skills/model-selection/agents/openai.yaml` | Interface descriptor per `annex-c §6` |
| `skills/provider-usage/SKILL.md` | Per `annex-c §2.2` |
| `skills/provider-usage/agents/openai.yaml` | Per `annex-c §6` |
| `skills/usage-aware-dispatch/SKILL.md` | Per `annex-c §2.3` |
| `skills/usage-aware-dispatch/agents/openai.yaml` | Per `annex-c §6` |

### 1.1 SKILL.md frontmatter schema

```yaml
---
name: <kebab-case skill name, required>
description: <string, required; the trigger conditions phrased as "Use when ... Trigger when ...">
---
```

No other frontmatter keys are used.

### 1.2 Required SKILL.md content sections (every skill)

1. `description` frontmatter with trigger conditions (verbatim from annex-c §2).
2. `## Commands` — exact `which-model` invocations (per SPEC behaviour §5).
3. Output-parsing rules naming the exact JSON fields to read, with `usage_enabled` checked FIRST (annex-c §1, §4.6).
4. `## Failure handling` — the annex-c §4.7 exit-code table.
5. Evidence-recording rule (exact model ID + full `Evidence` object; never narrate availability; annex-c §5).
6. A closing `## Checklist` (patterns from `usage-allowance-checks/SKILL.md` and `available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md`).

### 1.3 Install destinations

| Target | Default dir | `--user` dir |
|---|---|---|
| `generic` (default) | `<repo>/.agents/skills/<name>/` | not allowed (exit 2) |
| `claude` | `<repo>/.claude/skills/<name>/` | `~/.claude/skills/<name>/` |

Each install writes `SKILL.md` and `agents/openai.yaml` under that directory. Repo root = nearest ancestor of cwd containing `.git/` (directory or worktree file), or `--repo <path>`.

## 2. `internal/schema` (new package)

`internal/schema/schema.go`:

```go
package schema

// Commands returns the schema-bearing commands in index order.
func Commands() []string // ["usage", "pick", "explain", "routes"]

// Emit returns the JSON Schema document for command, or an error naming
// the valid commands if name is unknown.
func Emit(name string) ([]byte, error)

// Index returns the no-argument index document:
// {"commands":["usage","pick","explain","routes"]}\n  (compact JSON + "\n")
func Index() []byte
```

`internal/schema/schemas.go` — the four documents as package-level `const` raw strings, byte-exact:

| const | `$id` | Root `required` |
|---|---|---|
| `usageSchemaJSON` | `https://github.com/WD-Mitchell/which-model/schema/usage-snapshot.json` | `schema_version`, `usage_enabled`, `snapshots` |
| `pickSchemaJSON` | `https://github.com/WD-Mitchell/which-model/schema/pick-result.json` | `schema_version`, `usage_enabled`, `profile`, `strategy`, `candidates`, `excluded_candidates` |
| `explainSchemaJSON` | `https://github.com/WD-Mitchell/which-model/schema/explain-result.json` | `schema_version`, `usage_enabled`, `candidate`, `evidence` |
| `routesSchemaJSON` | `https://github.com/WD-Mitchell/which-model/schema/routes-list.json` | `schema_version`, `routes` |

### 2.1 Document deltas applied to `docs/plan/annex-c-agent-integration.md §4.1–§4.3`

Base text for usage/pick/explain is annex-c §4.1/§4.2/§4.3 verbatim, with EXACTLY these changes (field names otherwise verbatim):

1. Root `schema_version` property: `"const": "2.0"` (was `"1.0"`).
2. Root `required` gains `"usage_enabled"` (per the table above).
3. Root `properties` gains:
   ```json
   "usage_enabled": { "type": "boolean" },
   "usage_disabled_reason": {
     "type": "string",
     "enum": ["flag", "config", "compiled_out", "no_providers_enabled"]
   }
   ```
   plus the conditional:
   ```json
   "if": { "properties": { "usage_enabled": { "const": false } }, "required": ["usage_enabled"] },
   "then": { "required": ["usage_disabled_reason"] }
   ```
4. pick-result `Candidate`: `required` = `["candidate_id","route","model_score","provider_weight","final_score"]` (band/band_weight stay optional properties); `model_score`, `band_weight`, `provider_weight`, `final_score` are `"type": "string"` (decimal strings, annex-b §1.2). `Route` gains optional `"provenance": { "type": "string" }`.
5. explain-result `Evidence`: `required` = `["profile","score_inputs","route_provenance","excluded_candidates"]` (band, `snapshot_age_seconds`, `confidence`, `last_verified` stay optional properties per annex-c §5.1).

`routes-list.json` is new (full document in `specs/features/F28-agent-skills/TASKS.md` task F28-T2): root `{"schema_version": {"type":"string","const":"2.0"}, "routes": {array of $defs/Route}}`; `Route` required `["provider","model_id","model","reasoning","window_ids"]`, optional `provenance` enum `["provider_live","models_dev","user_declared"]`, `reasoning` enum `["minimal","low","medium","high","xhigh","max","default"]`, `additionalProperties: false`.

`usage_enabled`/`usage_disabled_reason` semantics: `docs/plan/annex-c-agent-integration.md §4.6`; consumer pinning policy: `annex-c §4.5`.

## 3. `internal/skills` (new package)

`internal/skills/skills.go`:

```go
package skills

type Target string

const (
    TargetGeneric Target = "generic"
    TargetClaude  Target = "claude"
)

// Names is the fixed skill set, in install order.
var Names = []string{"model-selection", "provider-usage", "usage-aware-dispatch"}

// RepoRoot walks upward from cwd to the nearest ancestor containing ".git"
// (directory or git worktree file, or returns repoDir from --repo). Error when neither exists.
func RepoRoot() (string, error)

// Install copies skills/<name>/SKILL.md and skills/<name>/agents/openai.yaml
// from the repo tree into the target dir (SPEC behaviour §2/§4).
// Returns a human message ("installed <name> to <dir>").
func Install(name string, target Target, user, force bool) (string, error)

// Remove deletes the two installed files (and now-empty dirs) for name.
// Not-installed is a no-op success. Modified files are refused without force.
// Returns a human message.
func Remove(name string, target Target, user, force bool) (string, error)

// List returns the installed skill names for the target.
func List(target Target, user bool) ([]string, error)
```

## 4. CLI commands owned (`pkg/whichmodel`)

Wiring per DECISION A: one file per command, `func init() { register(New<X>Cmd) }`, F22 owns `register`/`commandOrder` (`schema` and `skills` already in the order list).

### 4.1 `which-model schema [command...]`

`pkg/whichmodel/schema_cmd.go` — `func NewSchemaCmd() *cobra.Command`.

- No args → stdout = `schema.Index()` (JSON, `{"commands":[...]}` + `\n`).
- `schema usage|pick|explain|routes` → stdout = `schema.Emit(name)` bytes verbatim.
- Unknown name → stderr `which-model schema: unknown command "<name>" (known: usage, pick, explain, routes)`, exit 2.
- Exit codes: `0` known (incl. index); `2` unknown. (`docs/plan/annex-d-cli-reference.md §2.9`.)

### 4.2 `which-model skills {install|remove|list}`

`pkg/whichmodel/skills_cmd.go` — `func NewSkillsCmd() *cobra.Command`.

```
which-model skills install [skill...] [--target claude|generic] [--user] [--force] [--repo <path>]
which-model skills remove  [skill...] [--target claude|generic] [--user] [--force] [--repo <path>]
which-model skills list    [--target claude|generic] [--user] [--repo <path>] [--json]
```

- `skill...` ⊆ `{model-selection, provider-usage, usage-aware-dispatch}`; no args = all. Unknown name → exit 2.
- `--user` with `--target generic` → exit 2.
- install/remove stdout: one message line per skill; list stdout: one name per line (installed order = `skills.Names` order), `--json`:
  ```json
  {"target":"generic","user":false,"installed":["model-selection"]}
  ```
- Exit codes: `0` success (incl. idempotent no-ops); `1` I/O failure, no repo root, protected file without `--force`; `2` bad flags/names.

## 5. Exit codes used

All within the fixed six (`specs/global/SPEC.md §5`): `0` success, `1` runtime/I-O/environment, `2` argument/config errors. No new `Failure.Code` values (`specs/global/CONTRACTS.md §1.6`).

## 6. JSON shapes owned

- `schema.Index()`: `{"commands":["usage","pick","explain","routes"]}\n`.
- `skills list --json`: `{"target":"<target>","user":<bool>,"installed":[<names>]}`.

## 7. Config keys / flags owned

- Config keys: none.
- Flags: `--target` (claude|generic, default generic), `--user` (bool), `--force` (bool), `--repo <path>` on the `skills` command; positional `[command...]` on `schema`.

## 8. Cross-feature references

- Consumes only the public CLI surfaces of F24/F26/F27 (`which-model usage`, `which-model pick`, `which-model explain`, `which-model routes list --json`) as documented in `docs/plan/annex-d-cli-reference.md §2.1/§2.4/§2.5/§2.6`; no Go imports into their packages.
- The `explain --json` root carries `schema_version`, `candidate`, `evidence` per `docs/plan/annex-c-agent-integration.md §4.3` (schema emitted here locks that shape).
- Compiles under `-tags nousage`: `internal/schema` and `internal/skills` import nothing from `internal/usage`.
- Import boundaries (`specs/global/CONTRACTS.md §8`): `internal/schema`/`internal/skills` import stdlib only; `pkg/whichmodel` files import `internal/schema`/`internal/skills` via `pkg/whichmodel`'s own package (allowed: `pkg/whichmodel` MAY import any `internal/`).
