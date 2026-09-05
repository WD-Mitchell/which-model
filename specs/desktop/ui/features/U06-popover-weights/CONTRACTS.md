---
kind: feature-contracts
version: "1.0"
feature: U06-popover-weights
project: which-model-desktop
---

# U06-popover-weights — Contracts

## 1. Files

| File | Contents |
|---|---|
| `apps/desktop/src/popover/WeightsView.tsx` + `.css` + `.test.tsx` | weights header, editor body, add-metric popup, balance band, footer buttons, save/copy handlers |
| `apps/desktop/src/lib/overrides.ts` + `overrides.test.ts` | Zustand overrides store (SPEC §2.2), `overridesToProfileDetail`, `isDirty` |

DTO/host types from `@which-model/core`; `WeightEditor`/`BalanceSlider` from `@which-model/ui` (U03); footer container and rank query from U05. `overrides.ts` imports nothing from `EngineHost` — it is pure state; host calls live in `WeightsView` handlers.

## 2. Store / props

```ts
// lib/overrides.ts
export interface OverridesState {
  baseSlug: string                    // '' = not seeded (clean)
  coreShare: number                   // 10..90 step 5
  tier1: Record<string, number>       // keys ⊆ {intelligence, cost, speed}, values 1..5
  tier2: Record<string, number>       // category/custom group slugs, values 1..5
  init(profile: ProfileDetail): void          // seed from profile (SPEC §2.2)
  setWeight(key: string, v: number): void     // clamp int 0..5; 0 deletes key
  addMetric(key: string): void                // key := 3
  removeMetric(key: string): void
  setCoreShare(v: number): void               // clamp 10..90, snap to 5
  revert(profile: ProfileDetail): void        // back to base; keeps baseSlug
  clear(): void                               // baseSlug ''; store clean
}
export const useOverrides: UseBoundStore<StoreApi<OverridesState>>

export function isDirty(s: OverridesState, base: ProfileDetail): boolean
export function overridesToProfileDetail(s: OverridesState, base: ProfileDetail): ProfileDetail
// hash for the rank key is U05's overridesHashOf(dto | null) — 'none' when clean

// popover/WeightsView.tsx
export interface WeightsViewProps {
  profile: ProfileDetail              // active/base profile
  groups: GroupSummary[]              // for addable list
  weightControl: 'step' | 'bar' | 'slider'   // settings.weight_control
  pick?: RankedModel                  // current carousel pick (for Copy model id)
  onBack(): void
  onSaved(slug: string): void         // U05: set active profile + view 'landing'
}
```

Save retry rule (SPEC §2.8): attempt 1 `slug = base + '_custom'`, `name = profile.name + ' (custom)'`; attempt n≥2 `slug = base + '_custom_' + n`, `name = profile.name + ' (custom ' + n + ')'`; retry ONLY on `ErrorDTO.code === 'conflict'`, incrementing n by 1 each time.

## 3. Copy strings (exact)

| Where | String |
|---|---|
| Header | `Weights for {slug}` (slug mono, accent, underlined) |
| Section headers | `core benchmarks (higher = better, cheaper, faster)` · `task benchmarks` |
| Share texts | `{coreShare}%` · `{100-coreShare}%` |
| Buttons | `+ Add metric` · `Revert` |
| Balance labels | `core` · `task` |
| Footer buttons | `Copy model id` (primary) · `Save as profile` (secondary) |
| Revert toast | `weights reverted to {slug}` |
| Copy toast | `copied  {model_id}` (two spaces) |
| Copy toast, no pick | `nothing to copy` |
| Save toast | `saved as {finalSlug}` |

## 4. Geometry not in D00 §6

Add-metric popup 180px wide, max-height 150px, anchored left 0 / bottom 26px; header divider fades over 40px at each edge; back chevron 20×20 at margin-left −4px; balance knob 14×14 (weight-row knob stays 12×12 per D00 §6).

## 5. Test fixtures (vitest + `createMockEngineHost()`)

| Test | Assertion |
|---|---|
| store seeding | `init(profile)` copies core_share/tier1/tier2; `isDirty` false until an action |
| setWeight zero removes | `setWeight('mathematics', 0)` deletes the key from `tier2`; key appears in addable list |
| overrides change rank key | each edit → new `overridesHashOf` value → `useRank` refetches with `overrides` in the `RankRequest`; `profiles.save` NEVER called by edits |
| addable list | equals `[intelligence, cost, speed, …groups()]` minus present keys, in order |
| revert | `revert(profile)` restores base values, rank key returns to `'none'` hash after `clear`; toast `weights reverted to {slug}` |
| copy model id | with pick → `window.copyToClipboard(model_id)` once + toast `copied  {model_id}`; without → toast `nothing to copy`, no clipboard call |
| save-as-profile happy path | `profiles.create` called once with slug `{base}_custom`, name `{name} (custom)`, `builtin:false`; toast `saved as {base}_custom`; store cleared; `onSaved` called with final slug |
| save conflict retry | mock create rejects `conflict` twice → third call uses `{base}_custom_3` / `(custom 3)`; exactly 3 create calls; non-conflict rejection → no retry, toast message |

## Review correction — #171: create without replacing

Save as profile snapshots the current draft and uses the create-only API, retrying only `conflict` with `_custom`, `_custom_2`, etc. The action is disabled while pending; other failures retain the draft. Existing saved weights cannot be replaced by this flow. The collision regression verifies that the previous custom profile retains its values.

## Review correction — #173: reconcile saved profile changes

The overrides store retains a cloned saved baseline. `isDirty` compares weights to that baseline, so the render before a refetch reconciliation cannot send old clean weights as new overrides. `reconcile(profile)` seeds new saved data only when clean or switching identity; dirty edits retain their baseline and values. Revert seeds the newest fetched profile. A deleted active custom profile clears overrides and selects the first available complexity-scale profile. Config and pick events invalidate mounted profile details as well as summaries.

Pinned regressions: external Save refreshes clean controls without a stale override request; dirty values survive the same event; Revert uses the new persisted values; deleting the active custom profile selects a valid fallback; event tests refetch profile details and updated pick counts.

## Correction (2026-09-05)

The Profiles / Use Cases correction in `specs/desktop/backend/features/B03-profiles/SPEC.md` governs the new persisted profile selection and desktop terminology. The DTO extension is canonical in `specs/desktop/global/CONTRACTS.md`. Settings navigation now has both Profiles (curated defaults) and Use Cases (ranking presets).
