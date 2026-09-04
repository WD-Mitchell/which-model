---
kind: feature-spec
version: "1.0"
feature: U06-popover-weights
project: which-model-desktop
---

# U06-popover-weights — Weights View and Overrides Store

## 1. Purpose

The weights view is the popover's "tune it here, now" surface: the active profile's weights, editable in place, re-ranking live — without ever touching the saved profile. U06 owns `WeightsView` (`apps/desktop/src/popover/`) and the ephemeral overrides store (`apps/desktop/src/lib/overrides.ts`, Zustand) that U05's `useRank` reads. Nothing the user does here persists unless they explicitly `Save as profile`.

Depends on: U01, U03 (WeightEditor/WeightRow/BalanceSlider), U04 (RankCarousel), U05 (shell/footer/queries/view state). Mockup `specs/desktop/mockup/demo.dc.html` weights markup (lines 102–182, 218–221) and logic (`rowFor`, `onRevert`, `onSaveProfile`, `onCopyId`) are normative.

## 2. Behaviour

1. **Header + entry.** `PopoverApp` (U05) switches to `view: 'weights'` via the app menu. `WeightsView` supplies the shell header: back chevron (`.ib` 20×20, left −4px) → back to landing (overrides are KEPT — leaving the view is not a revert), title `Weights for {slug}` (13px, 62% text) with the slug rendered mono in `--color-accent-300` underlined by `box-shadow: 0 1px 0 color-mix(in srgb, var(--color-accent) 55%, transparent)`, then U05's hamburger. Below: the mockup's edge-fading 1px divider. `{slug}` is the overrides store's `baseSlug`.

2. **Overrides store (`lib/overrides.ts`).** Zustand store, state `{ baseSlug, coreShare, tier1, tier2 }` — a decomposed `ProfileDetail`. Initialised from the active profile's `ProfileDetail` when the weights view mounts with a different/empty `baseSlug`, and re-initialised whenever the active profile changes (U05 §2.7 calls `clear()`; next mount re-seeds). Actions:
   - `setWeight(key, v)` — v ∈ 0..5; 0 REMOVES the key from its map (D00 §2: 0 means absent); key routes to `tier1` iff ∈ {intelligence, cost, speed}, else `tier2`.
   - `addMetric(key)` — sets the key to 3 (mockup default).
   - `removeMetric(key)` — deletes the key (task-row × button).
   - `setCoreShare(v)` — clamped 10..90 step 5 (D00 §6).
   - `revert(profile)` — resets `coreShare`/`tier1`/`tier2` to the given base `ProfileDetail`; keeps `baseSlug`.
   - `clear()` — empties the store (`baseSlug: ''`), making rank queries clean again.
   Derived `isDirty(profile)`: true iff seeded and `coreShare`/`tier1`/`tier2` deep-differ from the cloned baseline stored at seed time. Every mutating action also resets U05's `selectedIndex` to 0 (mockup `resultIndex: 0`).

3. **Rank with overrides.** When dirty, U05's `useRank` sends `RankRequest.overrides` = the store re-assembled as a `ProfileDetail` (base profile's slug/name/builtin/picks/last_used, store's `core_share`/`tier1_weights`/`tier2_weights`); `overridesHash` = stable JSON stringify of that DTO (U00 §6), so every edit changes the query key `['rank', slug, overridesHash, holds]`. Clean store → `overrides` omitted, hash `'none'`. Overrides ranking is ephemeral engine-side too (D00 §2 RankRequest): no history, no writes — the frontend must NEVER call `profiles.save` from an edit.

4. **Editor body.** Tinted section (`color-mix(in srgb, var(--color-text) 3%, transparent)`) hosting U03's `WeightEditor` in the control style from `useSettings().weight_control` (step|bar|slider): header `core benchmarks (higher = better, cheaper, faster)` with `{coreShare}%` right; core rows (tier1, no remove affordance, `cost` label in `--color-accent-300`); header `task benchmarks` with `{100-coreShare}%`; task rows (tier2, × remove button). Row drag maps fraction → `round(f*5)` → `setWeight`. Value column shows the weight integer.

5. **Add metric / Revert row.** Ghost buttons `+ Add metric` (opens 180px popup, max-height 150px, scrollable) and `Revert` (right-aligned). Addable keys = ordered list `[intelligence, cost, speed] ∪ group slugs from useGroups()` minus keys present in `tier1 ∪ tier2`; picking one calls `addMetric(key)` and closes the popup. Revert calls `revert(baseProfile)` + toast `weights reverted to {slug}`.

6. **Balance band.** Bottom band (same tint, top border `--color-divider`): mono uppercase `core` / `task` labels, then U03's `BalanceSlider` — accent-500 core segment, 14px knob, accent-800 task segment, flex-proportioned `coreShare` / `100-coreShare`. Drag maps fraction → `max(10, min(90, round(f*20)*5))` → `setCoreShare`.

7. **Results.** The carousel always renders in the weights view regardless of `settings.layout` (U05 §2.8), directly below the balance band, re-ranking as the store changes.

8. **Footer (weights variant).** Via U05's `PopoverFooter` children: primary button `Copy model id`, secondary `Save as profile`.
   - **Copy model id:** with a current pick → `window.copyToClipboard(pick.model_id)` then toast `copied  {model_id}` (two spaces, mockup verbatim); no pick → toast `nothing to copy`, no clipboard call.
   - **Save as profile:** build a non-builtin `ProfileDetail` from the store with `slug = "{baseSlug}_custom"`, `name = "{profile.name} (custom)"`, `picks: 0`, `last_used: ""`, and call `profiles.create`. On rejection with code `conflict`, retry with a numeric suffix starting at 2: `{baseSlug}_custom_2` / name `{profile.name} (custom 2)`, then `_custom_3` / `(custom 3)`, … incrementing until save resolves (only `conflict` retries; any other code stops and toasts its message). On success: toast `saved as {finalSlug}`, `clear()` the store, set the saved slug as U05's active profile, and switch back to landing. (`config:changed` refreshes `['profiles']` via U05's invalidation.)

## 3. Error behaviour

- `profiles.create` non-conflict rejection → toast `ErrorDTO.message`; store untouched; view stays on weights.
- Rank query error while dirty → carousel shows the U05 §3 placeholder state; edits remain in the store.
- Store actions are total: out-of-range inputs are clamped (`setWeight` to 0..5 integers, `setCoreShare` per D00 §6), never thrown.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Persistence of edits | Zustand memory only; NEVER written to config | Plan §U06 "NEVER persisted"; D00 RankRequest.Overrides semantics |
| Back navigation | Keeps overrides; only profile change or explicit revert/save/clear discards | Round-tripping landing↔weights to compare picks must not lose work |
| Weight 0 semantics | Removes the key from the map | D00 §2: TOML stores 1–5 only; 0 = absent/ignored |
| Add-metric default | 3 | Mockup `w[k] = 3` |
| Conflict retry rule | `_custom`, then `_custom_2`, `_custom_3`, … first non-conflicting wins; names `(custom)`, `(custom 2)`, … | Deterministic, mirrors mockup naming; engine `conflict` is the probe |
| Save-then-select | Saved profile becomes active; view → landing | Mockup `onSaveProfile` sets profile/slug + `view: 'landing'` |
| Addable ordering | tier1 triple first, then `catalog.groups()` order | Mockup `ALLM = CORE.concat(TASK)` |

## 5. Out of scope

- Shell, header menu instance, footer container, queries/invalidation — U05.
- WeightEditor/WeightRow/BalanceSlider internals and drag hook — U03/U02.
- Profile management pages (duplicate/delete/rename) — U08.
- Engine-side ephemeral ranking semantics — backend features (D00 §2 is the contract).

## Review correction — #171: create without replacing

Save as profile snapshots the current draft and uses the create-only API, retrying only `conflict` with `_custom`, `_custom_2`, etc. The action is disabled while pending; other failures retain the draft. Existing saved weights cannot be replaced by this flow. The collision regression verifies that the previous custom profile retains its values.

## Review correction — #173: reconcile saved profile changes

The overrides store retains a cloned saved baseline. `isDirty` compares weights to that baseline, so the render before a refetch reconciliation cannot send old clean weights as new overrides. `reconcile(profile)` seeds new saved data only when clean or switching identity; dirty edits retain their baseline and values. Revert seeds the newest fetched profile. A deleted active custom profile clears overrides and selects the first available complexity-scale profile. Config and pick events invalidate mounted profile details as well as summaries.

Pinned regressions: external Save refreshes clean controls without a stale override request; dirty values survive the same event; Revert uses the new persisted values; deleting the active custom profile selects a valid fallback; event tests refetch profile details and updated pick counts.
