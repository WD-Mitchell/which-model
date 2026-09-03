---
kind: feature-spec
feature: F28-agent-skills
version: "1.0"
project: which-model
---

# F28 — Agent Skills

## Purpose

Ship the three agent skills — `model-selection`, `provider-usage`, `usage-aware-dispatch` — as authored SKILL.md files plus their `agents/openai.yaml` interface descriptors, and the `which-model skills install/remove/list` command that places them into harness-discovered locations. Provide `which-model schema <command>` as the machine contract for every `--json` output agents consume (usage, pick, explain, routes), with JSON Schemas matching annex-c §4 field names verbatim. Delete the two prototype skills (clean cutover, master plan M6 / annex-c §2.4).

## Behaviour

1. **Skill source of truth is the repo `skills/` tree** (`skills/<name>/SKILL.md` + `skills/<name>/agents/openai.yaml`, name and description frontmatter per annex-c §2). `which-model skills install` copies these files into the harness-visible target; it does not embed copies in the binary. Decision: the repo files are the single source of truth (no embed/sync drift); the binary does not need to be self-contained for skills because skills are always installed from a checkout (`docs/plan/annex-c-agent-integration.md §2`).

2. **Install location** (recorded decision): default target is the repo-local generic convention `.agents/skills/<name>/` (prototype convention and annex-c §2 path `agents/skills/`, discovered by Codex-style harnesses); `--target claude` writes `.claude/skills/<name>/` (Claude Code repo-level skills); `--user` (valid only with `--target claude`, else exit 2) writes `~/.claude/skills/<name>/` (user-global Claude Code skills). Repo-local is the default because it is versioned with the project and works for every harness; the user dir is an explicit opt-in.

3. **Repo-root resolution**: install/remove/list resolve the repository root by walking upward from the current directory to the nearest ancestor containing `.git/` (directory or git worktree file); `--repo <path>` overrides. Running outside a repo without `--repo` is exit 1 with a message naming the resolution rule (same upward-walk convention as project-local config/evidence, `docs/plan/annex-d-cli-reference.md §4.6`).

4. **Overwrite protection**: `skills install` writes a destination file only when it is absent or byte-identical to the source; a differing existing file is exit 1 with the file path named unless `--force` is given. `skills remove` deletes a file only when it is byte-identical to the shipped content unless `--force`; a user-modified installed skill is never destroyed silently. `--force` overwrites/deletes regardless. Idempotence: installing an already-current skill is success (no rewrite); removing a not-installed skill is success with a "nothing to remove" message (global SPEC §6 security invariants, "permission warnings, never auto-remediation").

5. **Skill content** (one file per skill, all content requirements below present in every SKILL.md):
   - **Trigger conditions** in the `description` frontmatter, verbatim from `docs/plan/annex-c-agent-integration.md §2.1/§2.2/§2.3`.
   - **Commands**: exact CLI invocations. Decision (recorded): annex-c §2's illustrative `catalog rank` and `usage get/list` command forms are superseded by the normative CLI of `docs/plan/annex-d-cli-reference.md §2.1/§2.4/§2.5` — the skill commands are `which-model pick --profile <P> --top N [--strategy S] [--seed N] [--available <path>] [--identity <M|R>] --json`, `which-model usage [provider...] --json` (positional), `which-model explain <candidate-id> --json`, and `which-model config show --json` (migration mapping `annex-d §5` row for `rank_models.py`).
   - **Output-parsing rules**: which JSON fields to read (`candidates[0].candidate_id` / `route.{provider,model_id,model,reasoning}` / `final_score` / `warnings` / `excluded_candidates[].reason_code`, `snapshots[].windows[]`, `evidence.*` per `annex-c §4.1/§4.2/§4.3`), and **`usage_enabled` is checked FIRST** before any band/pressure/quota field is read or cited (`annex-c §1` capability-declared principle, §4.6).
   - **Failure handling by exit code**: the `annex-c §4.7` table verbatim (0 proceed; 1 surface Failure.message, hard stop; 2 fix invocation, never retry unchanged; 3 widen filters/ask user; 4 all band-gated — quota-specific, surface band_gated candidates, do not dispatch; 5 explicit user-present login only).
   - **Evidence-recording rule**: record the exact model ID and reasoning effort accepted by the target harness plus the full `Evidence` object from `which-model explain` (annex-c §5) — never a free-text availability claim, never a model name/slug alone; when `usage_enabled` is false record `usage_disabled_reason` and never cite band fields (annex-c §5.1).
   - Usage-disabled applicability per `annex-c §2.5`: `model-selection` fully applicable; `provider-usage` inapplicable (report the lever and stop, exit 2 not retryable); `usage-aware-dispatch` defers to `model-selection` and records the pick as score-only.

6. **`which-model schema <command>`** (annex-d §2.9): `which-model schema usage|pick|explain|routes` prints the JSON Schema document for that command's `--json` output, byte-identical to the shipped registry document; `which-model schema` with no argument prints the index `{"commands":["usage","pick","explain","routes"]}`. Unknown command name → exit 2. Field names are verbatim from `annex-c §4.1/§4.2/§4.3` and `specs/global/CONTRACTS.md §3.1`.

7. **Schema documents are `schema_version` "2.0"** (recorded decision): annex-c §4.1–§4.3 show `const: "1.0"` for the pre-toggle shape; per annex-c §4.6 the `usage_enabled` field lands in the `required` array of every root object (with conditionally required `usage_disabled_reason`), which is a MAJOR bump to `"2.0"` matching `specs/global/CONTRACTS.md §6` (`SchemaVersion = "2.0"`). The emitted documents therefore carry `const: "2.0"`, `usage_enabled` in root `required`, the `if/then` for `usage_disabled_reason`, and everything else verbatim from annex-c §4.

8. **Score fields are decimal strings in the pick schema** (recorded decision): `model_score`, `band_weight`, `provider_weight`, `final_score` are `"type": "string"` (decimal-preserving JSON per `docs/plan/annex-b-catalog-port.md §1.2`, as shown in the `annex-d §2.4` `--json` example `"model_score": "88.4"`), not the annex-c §4.2 illustrative `number`.

9. **Degraded-mode optionality** (recorded decision): in pick-result, `Candidate.band`/`band_weight` are properties but NOT required (they are omitted when `usage_enabled` is false per the annex-c §4.6 degraded example); in explain-result, `Evidence.band`, `snapshot_age_seconds`, `confidence`, `last_verified` are properties but NOT required (omitted in degraded mode per annex-c §5.1). The route document carries an optional `provenance` (global CONTRACTS §3.1; present in the annex-c §4.6 degraded pick example).

10. **Routes schema** (recorded decision): F28 ships a `routes-list.json` schema for `which-model routes list --json` (F27 output) because agents consume route coverage; shape = `{"schema_version": "2.0", "routes": [Route]}`, `Route` fields verbatim from `specs/global/CONTRACTS.md §3.1` (`provider`, `model_id`, `model`, `reasoning` enum, `window_ids`, optional `provenance`). It does NOT carry `usage_enabled` — annex-c §4.6 applies `usage_enabled` to the three outputs usage/pick/explain only.

11. **Interface descriptors** (annex-c §6): each skill ships `agents/openai.yaml` with the verbatim `interface:` block from `docs/plan/annex-c-agent-integration.md §6`, installed alongside the SKILL.md.

12. **Legacy deletion** (annex-c §2.4, master plan M6): `usage-allowance-checks/SKILL.md`, `usage-allowance-checks/agents/openai.yaml`, and the whole `available-model-data-export/.agents/skills/` directory (containing `meta-orchestration-model-selection/SKILL.md`) are DELETED, not aliased, redirected, or stubbed. Any live (non-`docs/plan`) reference to the old skill names is updated in the same change.

## Error behaviour

| Condition | Exit |
|---|---|
| `which-model schema <known>` / `which-model schema` (index) success | 0 |
| `which-model schema <unknown>` | 2 |
| `skills install/remove/list` success (including idempotent no-ops) | 0 |
| Unknown skill name, unknown `--target`, `--user` with `--target generic` | 2 |
| Destination write failure, no repo root found, protected (non-identical) file without `--force` | 1 |
| Under `-tags nousage`: skills and schema commands are unaffected (no usage dependency); all skill content requirements still hold | — |

No new `Failure.Code` values are added by this feature.

## Decisions

| Decision | Value | Rationale |
|---|---|---|
| Skill install location | Default repo-local `.agents/skills/<name>/`; `--target claude` → `.claude/skills/<name>/`; `--user` (claude only) → `~/.claude/skills/<name>/` | Repo-local is versioned with the project and discovered by every harness (prototype + annex-c §2 convention); user dir is a Claude Code-specific opt-in |
| Install source | Repo `skills/` tree resolved via upward `.git/` walk (`--repo` override); no embedded copies | Single source of truth, zero embed/sync drift; skills are installed from a checkout by definition |
| Overwrite/remove protection | Refuse when destination differs from shipped bytes unless `--force` (exit 1) | Never destroy user-modified content (global SPEC §6 posture) |
| `catalog rank` / `usage get` command forms | Superseded by `pick --top N` / positional `usage [provider...]` | annex-d is the normative CLI contract (annex-d §1, §5 migration table); annex-c §2 commands were illustrative |
| Schema version | `"2.0"` with `usage_enabled` required + conditional `usage_disabled_reason` | annex-c §4.6 MAJOR bump; global CONTRACTS §6 `SchemaVersion = "2.0"` |
| Score field types in pick schema | `"type": "string"` (decimal strings) | Decimal-preserving JSON (annex-b §1.2); annex-d §2.4 example |
| Degraded optionality | band/band_weight and band/snapshot_age/confidence/last_verified not in `required` | Omitted-not-falsified rule (annex-c §4.6, §5.1) |
| Schema index scope | Exactly `usage, pick, explain, routes` at this feature's version | Assignment scope + annex-c §4.4; additive-only growth later |
| Routes schema | `{"schema_version","routes"}` with global Route fields; no `usage_enabled` | annex-c §4.6 covers usage/pick/explain only |
| Legacy skills | Deleted, not aliased | annex-c §2.4, master plan M6 |

## Out of scope

- Agent hooks and their installation (`F29-agent-hooks`).
- Workflow generation / publishing (`F30-publishing`).
- Shell completions, man pages, and alias symlinks listed under M6 — not part of this feature's assignment.
- Harness-specific packaging beyond `SKILL.md` + `agents/openai.yaml` (no Claude Code plugin manifest, no Codex `AGENTS.md`-style wiring; Claude Code and generic discovery is the harness's own feature).
- Editing F24/F26/F27 command implementations; this feature only documents and consumes their public `--json` surfaces (`docs/plan/annex-d-cli-reference.md §2.1/§2.4/§2.5`) and emits schemas for them.
