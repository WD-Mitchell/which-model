---
name: meta-orchestration-model-selection
description: >-
  Use when choosing a verified model and reasoning-effort row for a dispatched
  task. Trigger when a task needs a deterministic model ranking, a task profile,
  explicit score weights, or live harness availability filtering.
---

# Model selection: generated scores plus live availability

Use the generated score artifact and ranking script to choose an exact model and
reasoning level. The score artifact is a data-driven prior; it cannot prove that
the target harness exposes a row.

## Rank a task

From the repository root, run:

```sh
python3 .agents/skills/meta-orchestration-model-selection/scripts/rank_models.py \
  --profile balanced_implementation --top 5 --pretty
```

The script reads `.centree-agentic-framework/available_model_scores.csv` by
default. Use `--scores PATH` for another generated artifact. It emits JSON with
`recommendation`, ranked `alternatives`, per-tier contributions, category scores,
warnings, and excluded rows.

The score artifact contains only rows with all three Tier 1 source values. Rows
that are incomplete remain in `available_model_raw_values.csv` for later source
refreshes but are not eligible for ranking. The mandatory source metrics are
the Artificial Analysis intelligence index, cost per Intelligence Index task,
and the AA V2 `median_end_to_end_response_time_seconds` response-time metric.

Choose a profile from the task intent. The definitions and exact 0–5 weights
are executable data in `scripts/rank_models.py` (`PROFILES`):

- `simple_implementation` — cheap and fast, with instruction following.
- `simple_action_execution` — cheap and fast first, then instruction following
  and reliable evidence capture with tool and software-engineering support.
- `balanced_implementation` — balanced implementation value.
- `complex_implementation` — intelligence and software engineering with planning
  support.
- `ui_ux` — UI/UX and visual evidence plus implementation quality.
- `complex_action_execution` — tool execution, instruction following, and
  evidence capture.
- `financial_work` — finance, knowledge, reasoning, research, and instructions.
- `research` — research, knowledge, reasoning, and tool-assisted investigation.
- `planning` — the highest-capability planning signal.
- `orchestration` — planning and instruction following first for delegated
  agent workflows.
- `review` — instruction following, software engineering, reasoning, security,
  and evidence capture.

Complexity is a task-classification input used to select or adjust a profile; it
is not an additional score column. If custom weights are needed, provide either
`--weights-json` or repeatable `--tier1-weight NAME=VALUE` and
`--tier2-weight NAME=VALUE` arguments. Do not use both forms.

## Weighting rules

Tier 1 is mandatory in every profile. `intelligence`, `cost`, and `speed` must
all be present with positive weights. A row missing any of those scores is
excluded; values are never imputed or changed to zero. Tier 1 maps to the
Artificial Analysis intelligence index, cost per Intelligence Index task, and
the AA V2 median end-to-end response-time score. The page-derived
`time_per_intelligence_index_task_seconds` value is retained as optional legacy
raw data, but it is not fetched by the default or scheduled refresh workflow.

Tier 2 categories are optional. Missing category values are omitted from that
row's optional mean and produce a warning; they are never treated as zero. If a
row has no usable Tier 2 values, the script uses its Tier 1 score and reports
that fallback. The CSV contains score columns only, not coverage columns.

The generated category columns are:

`reasoning_score`, `knowledge_score`, `research_score`,
`planning_capability_score`, `instruction_following_score`,
`software_engineering_score`, `ui_visual_score`, `agentic_tools_score`,
`finance_score`, `evidence_capture_score`, `security_score`, and
`data_ml_score`.

`planning_capability_score` is fixed at 40% reasoning, 30% knowledge, 20%
agentic execution (`agentic_tools_score`), and 10% research. It deliberately
does not include instruction following or long-context evidence. Do not weight a
planning profile with the component categories again, because that would count
the same evidence twice. The `orchestration` profile therefore uses
`planning_capability_score` plus the independent instruction-following score;
it does not separately re-weight reasoning, knowledge, agentic tools, or
research.

Category generation is implemented in
`.github/workflows/update_available_model_data/generate_scores.py`. It uses the
exact benchmark groups in `benchmarks.toml`, requires the configured minimum
number of populated independent benchmarks, leaves an under-evidenced composite
blank, and deduplicates known aliases and variants before averaging. The direct
Artificial Analysis coding and coding-agent indexes are preferred over duplicate
models.dev benchmark columns.

## Live harness availability

After ranking, filter with the target harness's live picker or dispatcher. Pass
an exact model/reasoning list to the script when one is available:

```sh
python3 .agents/skills/meta-orchestration-model-selection/scripts/rank_models.py \
  --profile simple_action_execution \
  --available .tmp/live-model-efforts.txt
```

Each availability entry must identify the exact accepted row as
`model|reasoning` (JSON lists of strings or `{ "model": ..., "reasoning": ... }`
objects are also accepted). The script filters unavailable rows after score
calculation and never silently substitutes another model, effort, provider, or
harness. Record the exact ID and resolved effort accepted by the target
harness; model names or Artificial Analysis slugs alone are not availability
proof.

## Dispatch checklist

- [ ] Classify complexity and task intent; choose the closest profile.
- [ ] Keep intelligence, cost, and speed weights present and positive.
- [ ] Run the ranking script and inspect warnings/exclusions.
- [ ] Apply target-harness live availability with exact model and effort IDs.
- [ ] Dispatch the exact recommended row or a listed alternative; never invent a
      nearby effort as a fallback.
- [ ] Record the model, reasoning level, profile, and availability evidence.
