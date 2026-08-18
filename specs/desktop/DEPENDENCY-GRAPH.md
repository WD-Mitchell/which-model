---
kind: dependency-graph
version: "1.0"
project: which-model-desktop
---

# which-model Desktop — Work Units, Waves, Gates

Unit IDs: `SA-<node>` authors that node's SPEC.md + CONTRACTS.md; `IM-<node>` implements the node per its contracts (TDD: contract-named test files first). Every `IM-X` depends on `SA-X` plus the `IM-*` of the feature dependencies in the README index. Spec authoring depends only on parent-node specs.

## 1. Waves

```
WAVE 0  (1):    SA-D00                                  [done when global/ + README + this file exist]
WAVE 1  (3):    SA-B00, SA-U00, SA-S00
WAVE 2  (~13):  SA-B01..SA-B11, SA-U01, SA-S01
WAVE 3  (mix):  SA-U02..SA-U14, SA-S02..SA-S05,
                IM-B01, IM-B11, IM-U01, IM-S01
WAVE 4  (4):    IM-B02, IM-U02, IM-S02, IM-S03
WAVE 5  (~10):  IM-B03, IM-B05, IM-B06, IM-B07*, IM-B08, IM-B09, IM-B10,
                IM-U03, IM-U04, IM-U07          (*B07's Launch→RecordPick lands as a stub until IM-B04)
WAVE 6  (~10):  IM-B04, IM-U05, IM-U06, IM-U08..IM-U14
WAVE 7  (1):    IM-S04
WAVE 8  (1):    IM-S05 + final gate
```

Units within a wave touch disjoint files (per CONTRACTS file tables) and run in parallel.

## 2. Integration gates (all must pass before the next wave starts)

| Gate | After | Checks |
|---|---|---|
| G1 | wave 3 | `go test ./internal/config/...` · `pnpm --filter @which-model/core test` · `go build ./...` · `task desktop:dev` shows stub window |
| G2 | wave 5 | `go test ./internal/service/...` · `pnpm -r test` (ui green against MockEngineHost) · `go test ./...` (CLI regression) |
| G3 | wave 6 | popover + all 8 settings pages fully interactive against MockEngineHost in plain `vite dev` (browser) |
| G4 | wave 7 | real data: slider pick → weight edit changes rank → launch (copy mode) appends history.jsonl → restart shows pick count · every settings toggle round-trips config.toml (diff before/after) · `go test ./...` green |
| G5 | wave 8 | `task desktop:package` yields .app · cold-start manual pass of every mockup behaviour clause · `GOOS=windows go build ./cmd/which-model-desktop` compiles |

## 3. Status board

Maintained as units complete. Legend: ☐ pending · ◐ in progress · ☑ done.

| Unit | Status | | Unit | Status | | Unit | Status |
|---|---|---|---|---|---|---|---|
| SA-D00 | ☑ | | SA-U00 | ☑ | | IM-B11 | ☑ |
| SA-B00 | ☑ | | SA-U01..U14 | ☑ | | IM-U01 | ◐ (partial: package.json/types started; stopped mid-mock) |
| SA-S00 | ☑ | | SA-S01..S05 | ☑ | | IM-S01 | ◐ (partial: workspace files + wails v3.0.0-beta.9 pinned; stopped before verification) |
| SA-B01..B11 | ☑ | | IM-B01 | ☑ | | IM-B02..B10, IM-U02..U14, IM-S02..S05 | ☐ |
| gates G1..G5 | ☐ | | | | | | |

All spec-authoring (SA-*) units are complete: the tree is implementable as-is. Implementation (IM-*) was intentionally halted after IM-B01/IM-B11; resume from wave 3's remainder (finish IM-U01, IM-S01, then gate G1).
