---
kind: feature-spec
version: "1.0"
feature: U09-page-groups-benchmarks
project: which-model-desktop
---

# U09-page-groups-benchmarks — Groups & Benchmarks Pages

## 1. Purpose

The "Benchmark groups" settings page: a groups list, a group-detail editor (membership toggles, rename, coverage), and a benchmark-detail view (group chips + sortable tested-models table). Three app-level components in `apps/desktop/src/settings/pages/groups/`, registered with the U07 settings shell. Mockup `specs/desktop/mockup/demo.dc.html` (groups list ~431–455, group detail ~457–490, benchmark detail ~400–429, logic ~1327–1466) is normative for geometry, copy, and interaction.

Depends on: U02 (primitives), U07 (shell, `PageComponentProps`, `Detail` union, `DetailHeader`, PAGE_META).

## 2. Behaviour

1. **Views and navigation.** `GroupsPage` renders one of three views from the U07 detail state: no detail → groups list; `{kind:'group', slug}` → `GroupDetail`; `{kind:'benchmark', name, fromGroup?}` → `BenchmarkDetailView`. Opening a benchmark from inside a group detail sets `fromGroup: slug`; the shell's back action on a benchmark detail with `fromGroup` returns to that group's detail, not the list (U07 CONTRACTS `Detail` union rule). Group chips on the benchmark view navigate to `{kind:'group', slug}` (replacing the benchmark detail, mockup 1329).

2. **Data.** TanStack Query only — no extra state libraries; local view state (filter text, sort) is component `useState`. Queries (keys per U00 CONTRACTS §6): `['groups']` → `host.catalog.groups()` (`GroupSummary[]`); `['group', slug]` → `host.catalog.groupDetail(slug)` (`GroupDetail`); `['benchmark', name]` → `host.catalog.benchmarkDetail(name)` (`BenchmarkDetail`). All refetch via the `catalog:changed` invalidation map (U00 CONTRACTS §5); no manual invalidation.

3. **Groups list.** Header row (mono 9px uppercase): `group` (flex 1) / `benchmark count` (112px, right) / `profiles` (74px, right) / 132px blank actions column. One `.row` per `GroupSummary`: slug (mono 12.5px) with `custom` tag when `!builtin`; `benchmark_count`; `in_profiles`; actions = ghost `Duplicate` button, delete icon-button (`.ib`, disabled `.ib off` + title "Built-in group — cannot be deleted" for builtins; enabled title `Delete {slug}`), chevron. Row click opens group detail. Below the rows the footnote (verbatim, CONTRACTS §4). List page header comes from U07 PAGE_META (`New group` action); U09 wires the action handler.

4. **New group.** Compute the first free slug from the `['groups']` cache: `new_group`, then `new_group_2`, `new_group_3`, … (mockup 1198–1201). Call `catalog.saveGroup(slug, [])` (unknown slug ⇒ create), open `{kind:'group', slug}`, toast `created {slug}`.

5. **Duplicate / delete.** Duplicate (list row or detail button) → `catalog.duplicateGroup(slug)`; the HOST generates the copy slug; open detail on the returned `GroupDetail.slug` and toast `editing {slug}`. Detail button label is `Duplicate & edit` for builtins, `Duplicate` otherwise. Delete (custom only) → `catalog.deleteGroup(slug)`, toast `deleted {slug}`; from detail, also navigate back to the list. Builtin delete buttons render `.ib off`, no-op, detail title "Built-in group — cannot be deleted" / "Delete this group".

6. **Group detail header.** `DetailHeader` back link to the list; title = slug; blurb = builtin/custom variant (CONTRACTS §4). First content row: `built-in · read-only` tag (builtins only), then `grSummary` = `{N} of {M} benchmarks · weighted by {X} profiles` where N = count of `on` benchmarks in `GroupDetail`, M = `benchmarks.length` (full catalogue), X = `in_profiles` from the matching `['groups']` cache entry (0 while absent); right-aligned Duplicate + delete controls (§5).

7. **Rename (custom groups only).** A `name` label + text input (initial value = slug), hidden entirely for builtins. On commit (change event): sanitise `trim().toLowerCase().replace(/[^a-z0-9]+/g,'_').replace(/^_+/,'')`; empty result → keep old slug (reset input, no call); result colliding with another existing group slug → keep old slug (no call); unchanged → no call. Otherwise call `catalog.saveGroup(slug, currentOnList, sanitised)` and re-point the open detail to the new slug. No toast on rename (mockup 1440–1453).

8. **Benchmarks section.** Header row: `benchmarks` label, filter input placeholder `filter the catalogue`, right-aligned `models covered` column label. Rows = full catalogue from `GroupDetail.benchmarks`, filtered client-side by case-insensitive substring of the trimmed query, then sorted **on-first-then-alpha**: membership descending, ties by `name.localeCompare` ascending (mockup 1417–1420). Filter text resets to `''` whenever a group detail is opened. Each row: `.sw` toggle (`on` when member; builtins additionally `off` class, `cursor:default`, no handler), benchmark name (mono, click → benchmark detail with `fromGroup`), 56px coverage bar (fill `round(covered/coverage_total*100)%`), 52px right-aligned `{covered} / {coverage_total}` text. Scroll region max-height 268px.

9. **Membership toggle.** Immediate mutation, no debounce: build the updated membership list (current `on` names minus/plus the toggled benchmark) and call `catalog.saveGroup(slug, updatedList)`. Builtin rows never fire. Read-only groups show the footnote (CONTRACTS §4) below the list; custom groups show none.

10. **Benchmark detail.** `DetailHeader` title = name; blurb = `BenchmarkDetail.note`, or the fallback line (CONTRACTS §4) when `""`. First row: `in groups` label + one accent tag chip per `groups[]` slug (click → that group's detail); when empty, the no-groups line instead. Then `tested models` label + `benchCovText` = `{rows.length} of {coverage_total}` where the total comes from any catalogue row (the group query's `coverage_total`; host guarantees `BenchmarkDetail.rows` are tested-only).

11. **Benchmark table.** Three sortable header columns: `model (reasoning)` (flex 1, left), `benchmark result` (124px, centre), `normalised score` (188px header, centre). Active column tinted accent with suffix `  ↓` (desc) / `  ↑` (asc) — two spaces. Default sort: `score` desc. Clicking the active column flips desc↔asc; clicking another column selects it at desc (mockup 1341). Comparators: `model` → `(a.model+a.reasoning).localeCompare(b.model+b.reasoning)`; `value` and `score` → numeric on `value` (norm is monotone in value); desc = negated. Row cells: label `{model}  ({reasoning})` (two spaces), value `toFixed(1)` (124px centre), `norm` integer (32px right), 144px bar of width `{norm}%`. Scroll region max-height 300px.

## 3. Error behaviour

- Mutation rejections (`ErrorDTO`) toast `message` (U00 SPEC §3); the UI performs no optimistic update, so a failed `saveGroup`/`deleteGroup`/`duplicateGroup` leaves the rendered state untouched.
- Query error states render inline with a retry button per U07's shared page-error affordance.
- A group/benchmark detail whose query rejects `not_found` (e.g. deleted elsewhere) navigates back to the list.
- Empty states: zero groups → list renders only header + footnote; empty filter result → empty scroll region; `rows` empty → empty table under the headers.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Membership persistence | Immediate `saveGroup` per toggle, full updated list | Mockup writes state per click; `catalog:changed` refetch keeps UI truthful |
| Rename transport | `saveGroup(slug, onList, renameTo)` | Host API has no dedicated rename; D00 §5 signature |
| Rename collision/empty | Silently keep old slug, no call, no toast | Mockup 1441–1443 returns early; avoids `conflict` round-trip |
| New-group creation | Client picks `new_group[_N]` from `['groups']` cache, `saveGroup(slug, [])` | No `createGroup` on EngineHost; mirrors mockup slug loop |
| Duplicate slug | Host-generated (`duplicateGroup` return) | Backend owns slug uniqueness; UI just opens + toasts |
| Sort/filter locality | Component state + `localeCompare`, no library | Deterministic, testable, matches mockup exactly |
| Bench "of total" denominator | `coverage_total` (all scored (model,reasoning) rows) | Same denominator as group coverage bars |
| Back from benchmark | `fromGroup` on the `Detail` arm → returns to group detail | U07 CONTRACTS Detail-union rule; matches mockup nav feel |

## 5. Out of scope

- `catalog.*` host implementation and fixtures — U01/IM tiers; group weighting in profiles — U08; shell, PAGE_META copy, back-stack mechanics — U07; shared primitives (`Toggle`, `Tag`, `CoverageBar`, `Input`, toasts) — U02.
