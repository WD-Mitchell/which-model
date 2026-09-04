---
kind: feature-contracts
version: "1.0"
feature: U09-page-groups-benchmarks
project: which-model-desktop
---

# U09-page-groups-benchmarks — Contracts

DTOs (`GroupSummary`, `GroupDetail`, `GroupBenchmark`, `BenchmarkDetail`, `BenchRow`) and `EngineHost.catalog` are canonical in D00 CONTRACTS §2/§5 — not redefined here. `PageComponentProps` and the `Detail` union (including the `fromGroup` back-to-group rule) are owned by U07 CONTRACTS and cited by name.

## 1. Files

| File (`apps/desktop/src/settings/pages/groups/`) | Contents |
|---|---|
| `GroupsPage.tsx` | page entry registered with U07; routes list / GroupDetail / BenchmarkDetailView from the `Detail` state; groups list; New-group / duplicate / delete handlers |
| `GroupsPage.module.css` | list column geometry, footnote style |
| `GroupsPage.test.tsx` | §5 list tests |
| `GroupDetail.tsx` | group header row, rename input, filter, membership rows |
| `GroupDetail.module.css` | detail geometry (rename input 264px, filter 200px, bar 56px, cov 52px, scroll 268px) |
| `GroupDetail.test.tsx` | §5 detail tests |
| `BenchmarkDetailView.tsx` | chips, tested-models header, sortable table |
| `BenchmarkDetailView.module.css` | table geometry (value 124px, norm 32px, bar 144px, scroll 300px) |
| `BenchmarkDetailView.test.tsx` | §5 benchmark tests |

## 2. Props (exported TS)

```ts
// GroupsPage receives U07's PageComponentProps and is the only component
// touching EngineHost (via the app QueryClient helpers).
export type GroupsPageProps = PageComponentProps // U07 CONTRACTS

export interface GroupDetailProps {
  slug: string
  detail: GroupDetail            // ['group', slug] data
  inProfiles: number             // from ['groups'] cache; 0 when absent
  catalogueTotal: number         // detail.benchmarks.length
  onToggleBenchmark(name: string, on: boolean): void  // no-op never called for builtin
  onRename(sanitisedSlug: string): void  // fires only when a call is due (SPEC §2.7)
  onDuplicate(): void
  onDelete(): void               // never called for builtin
  onOpenBenchmark(name: string): void    // opens {kind:'benchmark', name, fromGroup: slug}
}

export interface BenchmarkDetailViewProps {
  detail: BenchmarkDetail        // ['benchmark', name] data
  coverageTotal: number          // denominator for "n of total"
  onOpenGroup(slug: string): void
}

export type BenchSortKey = 'model' | 'value' | 'score'
export interface BenchSort { k: BenchSortKey; dir: 'asc' | 'desc' } // default {k:'score',dir:'desc'}
```

Sanitiser (pure, exported from `GroupDetail.tsx` for tests):
`export function sanitiseGroupSlug(raw: string): string` = `raw.trim().toLowerCase().replace(/[^a-z0-9]+/g,'_').replace(/^_+/,'')`.

## 3. Host calls and toasts

| Action | Call | Toast (verbatim) |
|---|---|---|
| New group | `catalog.saveGroup(freeSlug, [])`, freeSlug ∈ `new_group`, `new_group_2`, … | `created {slug}` |
| Toggle membership | `catalog.saveGroup(slug, updatedOnList)` | — |
| Rename | `catalog.saveGroup(slug, currentOnList, renameTo)` | — |
| Duplicate | `catalog.duplicateGroup(slug)` → open returned slug | `editing {slug}` |
| Delete | `catalog.deleteGroup(slug)` | `deleted {slug}` |

Queries: `['groups']`, `['group', slug]`, `['benchmark', name]` (U00 CONTRACTS §6); refetch solely via `catalog:changed` (U00 CONTRACTS §5).

## 4. Copy (verbatim)

| Where | String |
|---|---|
| List section label | `benchmark groups` |
| List columns | `group` · `benchmark count` · `profiles` |
| Custom tag | `custom` |
| Builtin badge (detail) | `built-in · read-only` |
| Duplicate buttons | `Duplicate`; detail on builtin: `Duplicate & edit` |
| Delete titles | list builtin `Built-in group — cannot be deleted`; list custom `Delete {slug}`; detail builtin `Built-in group — cannot be deleted`; detail custom `Delete this group` |
| List footnote | `Every group here is weightable in a profile. A model’s score for a group is the mean of its results on that group’s benchmarks, so changing the list changes the ranking.` |
| Group summary | `{N} of {M} benchmarks · weighted by {X} profiles` |
| Rename label | `name` |
| Benchmarks label / filter / coverage | `benchmarks` · placeholder `filter the catalogue` · `models covered` |
| Coverage cell | `{covered} / {coverage_total}` |
| Readonly footnote | `A model’s score for this group is the mean of its results on the benchmarks switched on here — counted over every model and reasoning level that reports the benchmark. Duplicate the group to change what it measures.` |
| Detail blurb (builtin) | `A built-in group — its benchmark list is read-only. Duplicate it to make a version you can change.` |
| Detail blurb (custom) | `Add or remove benchmarks. A model’s score for this group is the mean of its results on the benchmarks listed here.` |
| Bench chips label / empty | `in groups` · `not in any group — it does not affect any profile` |
| Bench coverage | `tested models` · `{rows.length} of {coverageTotal}` |
| Bench columns | `model (reasoning)` · `benchmark result` · `normalised score`; active suffix `␣␣↓` / `␣␣↑` (two spaces) |
| Bench row label | `{model}␣␣({reasoning})` (two spaces) |
| Bench note fallback | `Carried in the model data export. No description recorded for this benchmark yet.` |
| Toasts | `created {slug}` · `editing {slug}` · `deleted {slug}` |

Page title/blurb/action for the list (`Benchmark groups` / blurb / `New group`) live in U07 PAGE_META; U09 only wires the action handler.

## 5. Tests (vitest + testing-library + `createMockEngineHost`)

**GroupsPage.test.tsx**
- renders one row per mock group with count and in-profiles numbers; builtin rows have no `custom` tag, custom rows do.
- builtin delete button has class `off`, title `Built-in group — cannot be deleted`, and clicking it calls nothing.
- New group action with existing `new_group` fixture calls `saveGroup('new_group_2', [])`, opens its detail, toasts `created new_group_2`.
- duplicate on a row awaits `duplicateGroup`, opens the returned slug, toasts `editing {slug}`.
- delete on a custom row calls `deleteGroup(slug)` and toasts `deleted {slug}`.

**GroupDetail.test.tsx**
- builtin: every row toggle has classes `sw off`, clicking fires no `saveGroup`; rename input absent; readonly footnote present.
- membership toggle on a custom group calls `saveGroup(slug, list)` where `list` = previous on-list plus/minus exactly the clicked name.
- filter `"MMLU"`/`"mmlu"` shows only substring matches; rows order = members first, then `localeCompare` alpha within each partition.
- sanitiser table: `"My Group! 2"`→`my_group_2`; `"__lead"`→`lead`; `"A--B"`→`a_b`; `"  "`→`""` (keep old slug, no call); rename to an existing slug → no call, input resets to old slug; valid rename calls `saveGroup(slug, onList, 'new_slug')`.
- benchmark name click fires `onOpenBenchmark`; resulting detail state carries `fromGroup: slug` and back returns to the group detail (U07 Detail-union rule).

**BenchmarkDetailView.test.tsx**
- group chips render per `detail.groups`; clicking one fires `onOpenGroup(slug)`; empty groups renders the no-groups line.
- default order = norm desc with header `normalised score  ↓`; click same header → asc `↑` and reversed order; click `model (reasoning)` → desc on `(model+reasoning).localeCompare`; click `benchmark result` → numeric desc on `value`.
- row shows value `toFixed(1)`, integer norm, bar width `{norm}%`; header shows `{rows.length} of {coverageTotal}`.

## Review correction — #172: durable membership edits

Membership changes enqueue a full-list snapshot immediately, with no debounce. One writer runs at a time; while it runs the latest pending snapshot replaces earlier pending snapshots. Navigation flushes retained work. Duplicate and rename drain the queue first; rename disables editing until completion and cancels work for the old identity after success. Delete drains before removal and cancels pending work after success. Detail components are keyed by slug. Only the latest acknowledged generation clears the local draft; errors toast and refetch the group without clearing newer edits.

Pinned regressions: toggle then Back/open benchmark persists; controlled delayed writes finish in order with newest state retained; rename/duplicate receive final membership; deletion does not recreate a group; failures refetch persisted truth.

The persistence barrier is shared by entity identity across editor mounts. Reopening an entity waits for the prior mount's final write and refetches before accepting edits; duplicate/delete/rename also wait for outstanding persistence. Pinned regression in the shared queue covers two mounts using the same key; the profile editor integration verifies disabled editing during the prior write.
