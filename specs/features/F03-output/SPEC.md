---
kind: feature-spec
feature: F03-output
version: "1.0"
project: which-model
---

# F03 — output: SPEC

## Purpose

`internal/output` renders every command's primary output: the JSON document (`--json`), the human text form (`--text`, the default), and the JSON Schema printer (`--schema` / `which-model schema`). It owns the stdout/stderr stream discipline — stdout carries only the payload, stderr carries only warnings and the failure line — so `which-model usage --json | jq .` is always safe (annex-d §1.3). It is Layer 0: it imports only the Go standard library.

Source: `docs/plan/annex-d-cli-reference.md` §1.2, §1.3, §2.9, §3; `specs/global/CONTRACTS.md` §6, §7; `docs/plan/research/usage-allowance-checks-spec.md` §1 (`runSafely` stream split).

## Behaviour

1. **JSON carries the envelope.** Every `--json` output is a single JSON document whose top-level object is the command payload merged with the canonical `output.OutputEnvelope` fields (`specs/global/CONTRACTS.md` §6): `schema_version` ("2.0"), `usage_enabled`, and `usage_disabled_reason` (present only when non-empty). `RenderJSON` performs the merge; no command may emit JSON without going through it. (annex-d §1.2 `--json`; `specs/global/CONTRACTS.md` §6.)
2. **Deterministic JSON.** The merged document is marshalled with `encoding/json` map marshalling (sorted keys) after a `json.Decoder` + `UseNumber` round-trip, so byte output is stable across runs and integer/decimal precision is preserved through the merge. The document is newline-terminated. (annex-d §3 determinism contract; `runSafely` writes `<output>\n`, `usage-allowance-checks-spec.md:249-263`.)
3. **Envelope keys are reserved.** If the payload's own JSON already contains `schema_version`, `usage_enabled`, or `usage_disabled_reason`, `RenderJSON` returns `ErrReservedField` instead of silently overwriting a canonical field. A payload that does not marshal to a JSON object returns `ErrPayloadNotObject`. (Decision D3, D4.)
4. **Text renderers.** `RenderLines` writes each line followed by `\n` (empty input writes nothing). `RenderTable` writes an aligned, header-first table: column width = widest cell in that column across header and rows, right-padded cells joined with a single space; rows shorter than the header are padded with empty cells, rows longer than the header return an error. (annex-d §1.2 text mode; `formatUsageReport` line style, `usage-allowance-checks-spec.md` §1.)
5. **Stream discipline.** `WriteFailure` emits the single fixed failure line `which-model <command>: [<code>] <message>` + `\n`; `WriteWarning` emits `warning: <message>` + `\n`. Callers route both to stderr; stdout never receives them. (annex-d §1.3 failure-line format; the prototype's `Usage check failed [...]` prefix is NOT carried forward — annex-d §1.3.)
6. **Identity redaction hook.** `RedactIdentity(value, show)` returns `nil` when `show` is false and `&value` when true. The `--show-identity` contract is "omitted entirely, not merely masked" (annex-d §1.2), so callers put the result in JSON with `omitempty` or skip it in text output when nil.
7. **Schema printer.** `PrintSchema` emits one JSON Schema document (a caller-supplied `map[string]any`, marshalled deterministically, newline-terminated). `PrintSchemaIndex` emits `{"commands": [...]}` for `which-model schema` with no argument (annex-d §2.9). `--schema` equivalence and unknown-command handling (exit 2) are F22-cli-skeleton concerns, not F03.

## Error behaviour

- `RenderJSON`: `ErrPayloadNotObject` when the payload marshals to anything other than a JSON object (array, string, number, null); `ErrReservedField` when the payload already contains an envelope key; underlying `io`/marshal errors are returned as-is. These are programmer/plumbing errors — commands surface them as generic runtime errors (exit 1 at the CLI layer, `specs/global/SPEC.md` §5).
- `RenderTable`: error when any row has more cells than the header (programmer error).
- `WriteFailure`/`WriteWarning`/`RenderLines`/`PrintSchema`/`PrintSchemaIndex`: only propagate `io.Writer` errors.
- F03 introduces **no new `Failure.Code` values** and owns **no flags and no config keys** (the global flags `--json`, `--schema`, `--show-identity`, `--quiet`, `--verbose` are declared and wired by F22-cli-skeleton; F03 supplies the renderers and the identity hook).

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | JSON output = envelope fields merged into the payload's top-level object | merge via `UseNumber` decode → inject → deterministic re-marshal, `\n`-terminated | `jq .schema_version` must work on every `--json` document (annex-d §1.2, §1.3); single document per run (annex-d §3) |
| D2 | Deterministic key order | `encoding/json` map marshalling sorts keys | byte-identical output across runs and machines; no map iteration order anywhere |
| D3 | Envelope keys reserved | payload already containing `schema_version`/`usage_enabled`/`usage_disabled_reason` → `ErrReservedField` | never silently overwrite canonical fields; fail loud |
| D4 | Payload must be an object | non-object payload → `ErrPayloadNotObject` | envelope fields are top-level; only objects can carry them |
| D5 | `schema_version` default | empty `env.SchemaVersion` → `"2.0"` (global constant, `specs/global/CONTRACTS.md` §7) | one canonical schema version; commands don't each pick one |
| D6 | Text table style | aligned columns, header first, single-space separator, no box drawing; short rows padded, long rows error | boring deterministic output; long rows are programmer bugs, not renderer quirks |
| D7 | Failure line owned by F03 | `which-model <command>: [<code>] <message>` via `WriteFailure` | annex-d §1.3 fixes the format once; every command shares one renderer |
| D8 | Identity redaction | omitted (nil), never masked | annex-d §1.2 verbatim; masking still leaks presence |
| D9 | Schema documents are `map[string]any` | `PrintSchema` marshals caller-supplied maps deterministically | command schemas are open-ended; sorted-key marshal keeps byte determinism |

## Milestone / dependencies

Milestone M1. `depends_on`: — (none, Wave W1, `specs/DEPENDENCY-GRAPH.md` §2–§3). `blocks`: F22 (cli-skeleton).

## Out of scope

- Cobra command tree, flag declarations, `--schema`/`--json` wiring, exit-code mapping → F22-cli-skeleton.
- Usage-disabled reasoning values (`flag|config|compiled_out|no_providers_enabled`) → F21-usage-toggle; F03 only renders the envelope.
- Per-command payload schemas (the actual JSON Schema documents) → each owning feature (F23, F24, F26, …).
- Color/ANSI styling (`--no-color`) → F22.
- Version string (`which-model version`) → F22; `internal/httpkit` owns its own build `Version` var (F04).
