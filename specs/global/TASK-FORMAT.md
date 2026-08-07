---
kind: task-format
version: "1.0"
project: which-model
audience: low-reasoning implementation model
---

# Task Format — how every TASKS.md is written

Every feature directory contains `SPEC.md`, `CONTRACTS.md`, and `TASKS.md`. Tasks are written for a **small model with very low reasoning**. That means:

## 1. Rules for task authors

1. **Every decision is made in the task, never left to the implementer.** If there are two ways to do something, the task picks one and says why in one sentence.
2. **No signposting without a destination.** Every reference like "see spec §3.2" MUST include the file path: `specs/features/F09-scoring/SPEC.md §3.2`.
3. **Verbatim code where the shape matters.** Type signatures, function signatures, test-case tables are given in full. The implementer types them, not designs them.
4. **One task = one file (or one tightly-related group), one test file, one `go test` run.** A task MUST NOT touch files outside its declared file list.
5. **TDD order inside every task:** write the test first, watch it fail to compile, write the implementation, run `go test ./path/...`, confirm pass.
6. **No open questions.** If a question exists, it was answered during planning and the answer is written into the task.

## 2. TASKS.md structure

```markdown
---
kind: feature-tasks
feature: F<NN>-<slug>
version: "1.0"
task_count: <N>
---

# F<NN> — <title>: Tasks

## Task graph

<mermaid graph of task deps within the feature>

## Task F<NN>-T1: <imperative title>

**Depends on:** F<NN>-T0 or none
**Files:**
- create `internal/<pkg>/<file>.go`
- create `internal/<pkg>/<file>_test.go`

**Spec references:** `specs/features/F<NN>-<slug>/SPEC.md §N.M`, `specs/global/CONTRACTS.md §N`

**Instructions:**
1. <numbered, imperative, no ambiguity>
2. ...

**Test cases (write these first):**

| # | input | want |
|---|---|---|
| 1 | ... | ... |

**Acceptance criteria:**
- [ ] `go build ./internal/<pkg>/...` succeeds
- [ ] `go test ./internal/<pkg>/...` passes with the test cases above
- [ ] no file outside the Files list modified
- [ ] <feature-specific criterion>
```

## 3. What "done" means for a task

A task is done when every checkbox in its acceptance criteria is checked by running the named commands and observing the named output. No narrative. Evidence = command + output.

## 4. Cross-feature references

When a task uses a type or function from another feature, it MUST cite the exact symbol and file:

> Use `usage.Window` exactly as defined in `specs/global/CONTRACTS.md §1.4` (Go file `internal/usage/types.go`, produced by task F11-T1).

The implementer does not look up the other feature's spec — the citing task carries everything needed.

## 5. Size limit

A task is too big if any of:
- it creates more than 2 source files,
- it has more than 12 test cases,
- its instructions exceed 25 numbered steps,
- it needs a design decision.

Split it.
