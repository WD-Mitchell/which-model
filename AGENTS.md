# AGENTS.md

This repo is implemented **spec-as-source**. The specs are the source of truth; `docs/plan/` is the design record they were distilled from.

## Start here

1. Read `specs/README.md` — roadmap and feature index.
2. Read `specs/global/TASK-FORMAT.md` — how tasks are written and what "done" means.
3. Read `specs/global/CONTRACTS.md` — canonical types. Use them verbatim; never redefine or extend them.
4. Read `specs/DEPENDENCY-GRAPH.md` — a task is runnable only when every task it lists under `Depends on:` is complete.

## Working on a task

- Open the feature's `specs/features/F<NN>-<slug>/TASKS.md`, find your task, and follow its **Instructions** step by step. Every decision has already been made there — if something looks ambiguous, the answer is in the task or its cited spec section; do not invent behaviour.
- TDD order is mandatory: write the listed test cases first, confirm the red state, then implement, then run the task's exact `go test` command.
- Touch only the files in the task's **Files** list. Done = every **Acceptance criteria** checkbox verified by running the named commands.

## Invariants

- Go module `github.com/WD-Mitchell/which-model`; import boundaries are enforced per `specs/global/CONTRACTS.md` §8.
- No credential material may ever appear in errors, logs, or output (canary-tested; `specs/global/SPEC.md` §6).
- `internal/catalog/**` must compile and test under `go build -tags nousage`; CI runs both variants.

## Known seams

`specs/DEFERRED.md` records resolved and accepted design seams — read it before working on F14 or the provider adapters (F15–F17).
