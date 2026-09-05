---
kind: feature-spec
version: "1.0"
feature: U08-page-profiles
project: which-model-desktop
---

# U08-page-profiles — Profiles Settings Page

## 1. Purpose

The Profiles page of the settings window: a list of every profile (built-in and custom) with weight sparkbars, pick counts, and row actions, plus a detail view where a custom profile's weights and core/task balance are edited in place and a built-in profile is presented read-only. Lives in `apps/desktop/src/settings/pages/profiles/`; registered in U07's page registry under nav entry "Profiles". Data flows exclusively through `getHost().profiles` (D00 CONTRACTS §5) via TanStack Query.

Depends on: U04 (`ProfileWeightSparkbar`), U07 (settings shell, `DetailHeader`, page registry, `PageComponentProps`), U03 (`WeightRow`, `BalanceSlider`), U01/U00 (host, query keys). Mockup `specs/desktop/mockup/demo.dc.html` is normative for geometry and copy (list: lines 278–323; detail: 325–398; logic: 1122–1143, 1356–1379, 1476–1503).

## 2. Behaviour

1. **List view.** Query `['profiles']` → `profiles.list()`. Column header row (mono, 9px, uppercase, letter-spacing .13em): `name` (flex 1) / `weights` (120px) / `picks` (48px, right) / `used` (64px, right) / blank actions column (132px). Above it, the accent kicker `profiles`. Each row (11px 22px padding, 1px top hairline, pointer cursor): slug (mono 12.5px, ellipsized), `ProfileWeightSparkbar`, picks (`toLocaleString()`), used (relative-time from `last_used`, `—` when empty), then right-aligned actions: `Duplicate` ghost button, trash icon-button, chevron. Row click opens the detail view for that slug. Below the rows, the footnote (11px, 42% text, max-width 56ch): "Picks count every launch made with the profile — from the popover and from the `wm` CLI alike." (`wm` in mono).

2. **Sparkbars.** `ProfileWeightSparkbar` (U04) receives **only weighted keys** — core = tier1 keys with weight > 0 in `intelligence, cost, speed` order; task = tier2 keys with weight > 0 in catalogue group order. Zero/absent keys render no bar (mockup `weighted` filter, line 1488). Hover tooltip per bar: `{key}  {v} / 5`.

3. **Row actions.** *Duplicate*: `profiles.duplicate(slug)`, toast `duplicated {slug}`, stay on the list (refresh via `config:changed` invalidation). *Delete*: enabled (`ib`) with title `Delete {slug}` for custom profiles → `profiles.delete(slug)`, toast `deleted {slug}`; for builtins the trash is disabled (`ib off`), title `Built-in profile — cannot be deleted`, click inert. Both action clicks stop propagation (must not open the row).

4. **New profile (page action).** The header's primary action (label `New profile`, U07 `pageAction`) creates name `profile {N}` where N = current profile count + 1, slug = name with spaces replaced by underscores (`profile_4`). On `conflict` from `profiles.create`, retry with the next integer until it succeeds. Payload: `builtin:false`, `core_share:60`, `tier1_weights:{intelligence:3, cost:3, speed:3}`, `tier2_weights:{}`, picks 0. Then open its detail view and toast `new profile created`.

5. **Detail view.** Query `['profile', slug]` → `profiles.get(slug)`. Header via U07 `DetailHeader`: back link labelled `Profiles`, title = slug, blurb = builtin ? "A built-in profile — its weights are read-only. Duplicate it to make a version you can change." : "Drag a weight to change how much this profile cares about each benchmark. Zero means the benchmark is ignored.", no page action. Summary strip: builtin badge `built-in · read-only` (tag-neutral, 8.5px) when builtin; `pfSummary` text `{weighted} of {total} benchmarks weighted · {picks} picks` (weighted = count of weight>0 keys across both tiers; total = 3 + group count; picks localized); right-aligned `Duplicate & edit` (builtin) / `Duplicate` (custom) ghost button and trash icon-button (same enable/title rules as §3).

6. **Weight sections.** Section `core benchmarks` with side note `{core_share}% of the score`; rows for intelligence/cost/speed. Section `task benchmarks` with `{100−core_share}% of the score`; one row per tier-2 group. Each row: 150px mono label (dim when weight 0), U03 `WeightRow` in the variant named by `GUISettings.weight_control` (max-width 300px), 56px value cell: `{v} / 5` (accent-300) when v>0, `ignored` (dim) when 0. Read-only (builtin): cursor `default`, no pointerdown handler, values still rendered.

7. **Balance block.** Max-width 434px: mono caption row `core` / centred accent `pfRatio` = `{core_share} / {100−core_share}` / `task`, then U03 `BalanceSlider` (ratio variant: accent-500 core bar, 14px centre knob, accent-800 task bar, flex = share). Builtin ⇒ cursor `default`, no handler.

8. **Editing & persistence.** Weight drags and balance drags on a custom profile update local detail state immediately (bars/ratio track the pointer, as the mockup writes live). Setting a weight to 0 deletes the key. Persistence is DEBOUNCED: `profiles.save(detail)` fires 300ms after the last change in a batch — one save per edit burst, not per pointermove (deviation from the mockup, which has no backend; Decisions). Pending debounce is flushed on unmount/back-navigation. `config:changed` then invalidates `['profiles']`/`['profile', slug]` per U00 CONTRACTS §5.

9. **Detail actions.** *Duplicate*: `profiles.duplicate(slug)` → navigate to the returned copy's detail, toast `editing {newSlug}`. *Delete* (custom only): `profiles.delete(slug)`, return to list, toast `deleted {slug}`.

## 3. Error behaviour

- Query errors render the shared inline error state with retry (U00 SPEC §3); mutation rejections toast `ErrorDTO.message`.
- `builtin_readonly` can only arise from a bug — the UI never wires mutation handlers on builtins (defence in depth: handlers are absent, not merely guarded).
- Empty list is impossible in practice (builtins always exist) but renders headers + footnote without rows, no crash.
- Debounced save failure toasts the message and refetches `['profile', slug]` so the UI re-syncs with persisted truth.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Save cadence | Optimistic local state + one `profiles.save` 300ms after the last change; flush on unmount | Mockup writes state live with no backend; per-move saves would hammer the TOML writer |
| Sparkbar input | Weighted (v>0) keys only, tier order preserved | Mockup filters `weighted` before mapping bars (line 1491) |
| New-profile slug conflicts | Retry `profile {N+1}` on `conflict` until save succeeds | Mockup counts length+1; real store may have gaps/deletions |
| Weight-row variant | Follows `GUISettings.weight_control` from `['settings']` | Mockup's `st.sliderStyle` drives isStep/isBar/isSlider (line 1132) |
| Builtin read-only | No handlers attached; cursor default; trash `ib off` + title | Mockup passes `null` onDown/onPfBalance for builtins |
| List duplicate stays on list; detail duplicate navigates | Toasts `duplicated {slug}` vs `editing {slug}` | Mockup lines 1501 vs 1373–1378 |

## 5. Out of scope

- `ProfileWeightSparkbar`, `WeightRow`, `BalanceSlider` internals — U04/U03.
- Settings shell, nav, `DetailHeader`, page registry, `PageComponentProps` — U07.
- Popover profile picking and ephemeral overrides — U05/U06.
- Backend profile CRUD, slug validation, pick aggregation — B03/B11.

## Review correction — #171: create without replacing

New profile uses atomic `profiles.create`, starts at list count + 1, retries the next integer only on conflict, and disables submission while pending. The list-count collision regression verifies that an existing profile retains its weights while the new profile opens at the next free slug.

## Review correction — #172: durable editor saves

Each detail editor is keyed by slug and owns one serialized autosave queue. It retains the latest snapshot, debounces profile edits by 300ms, and flushes exactly once on navigation/unmount. Duplicate waits for the queue; Delete disables editing, drains the queue, deletes the identity, and cancels retained work before navigation. An older completion or error cannot clear a newer draft. Successful saves refetch the detail; failures toast and refetch persisted truth. Clean editors render fresh server data directly.

The persistence barrier is shared by entity identity across editor mounts. Reopening an entity waits for the prior mount's final write and refetches before accepting edits; duplicate/delete/rename also wait for outstanding persistence.

Pinned regressions: duplicate contains pending weights; delete never recreates the profile; delayed completion/error retains a newer draft; delay a flushed write, navigate away and reopen, then edit again—the final saved snapshot includes both edits in order.

## Review correction — #174: stable task controls

Task controls use the sorted union of available catalogue group slugs and persisted keys seen during this editor session, independent of the sparse saved weights. Zero removes a saved task key while its row remains as ignored and can be raised again. Newly created empty profiles still expose task rows. Existing persisted controls remain available while group data is absent. The summary denominator is three core axes plus displayed task rows. Core weights remain 1–5. Configured custom groups use the category-aware backend vocabulary.

Pinned regression: reducing a task weight to zero retains the same control, raising it again updates the draft, and immediate unmount persists the final value exactly once.

## Review correction — #173: fresh saved details

Configuration and recorded-pick events invalidate the mounted `profile` query prefix along with the profile list. Editors without a local draft render the refreshed saved weights and pick counts. Draft ownership follows the durable-save contract above, so a refresh cannot replace an in-flight local edit. The profile-prefix event regression verifies both saved edits and recorded-pick statistics are refetched.


## Complete task vocabulary correction — #174 review

Editable task rows are the union of F10's twelve canonical categories, catalog
group slugs, and keys already stored on the profile. Include
`planning_capability` even when no group or existing weight exposes it. An
empty new profile must allow every canonical task weight to be set.


## List-action persistence correction — #172 review

After leaving an editor, list-row duplicate and delete wait for that entity's
outstanding autosave to finish. Thus duplication reads committed edits and
deletion cannot be followed by a queued save recreating the entity. Profile
list actions suppress duplicate submissions while waiting.


## Correction — Profiles and Use Cases (2026-09-05)

The user's requested distinction between Profiles and Use Cases supersedes the
conflicting terminology and Quick complexity-scale behavior above. The governing
behavior and pinned validation cases are in
`specs/desktop/backend/features/B03-profiles/SPEC.md` §Correction. Canonical DTOs
are extended in `specs/desktop/global/CONTRACTS.md` §Profiles / Use Cases extension.
