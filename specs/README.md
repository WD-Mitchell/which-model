---
kind: roadmap
version: "1.0"
project: which-model
---

# which-model — SDD Roadmap

Spec-as-source development. The specs in this tree are the source of truth for implementation; `docs/plan/` is the design record the specs were distilled from. When a spec and the plan disagree, the spec wins (and the plan gets a correction note).

## Reading order

1. [`global/SPEC.md`](./global/SPEC.md) — purpose, layers, packages, exit codes, security invariants
2. [`global/CONTRACTS.md`](./global/CONTRACTS.md) — canonical types (Window, Snapshot, Route, Candidate, Failure codes)
3. [`global/TASK-FORMAT.md`](./global/TASK-FORMAT.md) — how tasks are written and what "done" means
4. [`DEPENDENCY-GRAPH.md`](./DEPENDENCY-GRAPH.md) — feature DAG, parallel waves, milestone map

## Feature index

Every feature lives in `features/F<NN>-<slug>/` with three files:

- `SPEC.md` — what the feature is, its behaviour, and the decisions made
- `CONTRACTS.md` — exported API signatures, config keys, flags, error codes
- `TASKS.md` — ordered tasks with acceptance criteria for TDD implementation

| Feature | Title | Milestone | Depends on |
|---|---|---|---|
| [F01](./features/F01-config/SPEC.md) | config | M1 | — |
| [F02](./features/F02-decimal/SPEC.md) | decimal | M1 | — |
| [F03](./features/F03-output/SPEC.md) | output | M1 | — |
| [F04](./features/F04-http/SPEC.md) | http | M1 | — |
| [F05](./features/F05-security/SPEC.md) | security | M1 | — |
| [F06](./features/F06-csvstore/SPEC.md) | csvstore | M1 | F02, F05 |
| [F07](./features/F07-identity/SPEC.md) | identity | M1 | F02 |
| [F08](./features/F08-collectors/SPEC.md) | collectors | M1 | F02, F04, F07 |
| [F09](./features/F09-scoring/SPEC.md) | scoring | M1 | F02, F06, F07 |
| [F10](./features/F10-ranking/SPEC.md) | ranking | M1 | F02, F09 |
| [F11](./features/F11-usage-types/SPEC.md) | usage-types | M2 | F05 |
| [F12](./features/F12-credentials/SPEC.md) | credentials | M2 | F05, F11 |
| [F13](./features/F13-usage-cache/SPEC.md) | usage-cache | M2 | F11 |
| [F14](./features/F14-usage-fetch/SPEC.md) | usage-fetch | M2 | F04, F11, F12, F13 |
| [F15](./features/F15-provider-claude/SPEC.md) | provider-claude | M2 | F12, F14 |
| [F16](./features/F16-provider-codex/SPEC.md) | provider-codex | M2 | F12, F14 |
| [F17](./features/F17-provider-copilot/SPEC.md) | provider-copilot | M2 | F12, F14 |
| [F18](./features/F18-routing/SPEC.md) | routing | M3 | F07, F08, F11 |
| [F19](./features/F19-bands/SPEC.md) | bands | M4 | F01, F11 |
| [F20](./features/F20-strategies/SPEC.md) | strategies | M4 | F10, F18, F19 |
| [F21](./features/F21-usage-toggle/SPEC.md) | usage-toggle | M4 | F01, F11, F14 |
| [F22](./features/F22-cli-skeleton/SPEC.md) | cli-skeleton | M1 | F01, F03 |
| [F23](./features/F23-cmd-catalog/SPEC.md) | cmd-catalog | M1 | F06, F08, F09, F22 |
| [F24](./features/F24-cmd-usage/SPEC.md) | cmd-usage | M2 | F14, F22 |
| [F25](./features/F25-cmd-auth/SPEC.md) | cmd-auth | M2 | F12, F22 |
| [F26](./features/F26-cmd-pick/SPEC.md) | cmd-pick | M4 | F20, F21, F22 |
| [F27](./features/F27-cmd-routes/SPEC.md) | cmd-routes | M3 | F18, F22 |
| [F28](./features/F28-agent-skills/SPEC.md) | agent-skills | M6 | F24, F26 |
| [F29](./features/F29-agent-hooks/SPEC.md) | agent-hooks | M6 | F28 |
| [F30](./features/F30-publishing/SPEC.md) | publishing | M6 | F01, F23 |

## Parallelism

See [`DEPENDENCY-GRAPH.md` §3](./DEPENDENCY-GRAPH.md) for waves. Within a feature, each `TASKS.md` carries its own task DAG.
