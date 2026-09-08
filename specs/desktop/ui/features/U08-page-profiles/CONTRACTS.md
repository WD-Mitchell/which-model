---
kind: feature-contracts
version: "1.0"
feature: U08-page-profiles
project: which-model-desktop
---

# U08-page-profiles — Contracts

DTOs (`ProfileSummary`/`ProfileDetail`), `EngineHost.profiles`, error codes: D00 CONTRACTS. Query keys and invalidation: U00 CONTRACTS §5–6. Visual tokens: D00 CONTRACTS §6. `PageComponentProps` and `DetailHeader` are defined in `specs/desktop/ui/features/U07-settings-shell/CONTRACTS.md` (referenced by name; not yet authored at time of writing — its shape is owned there, not here).

## 1. Files

| Path | Contents |
|---|---|
| `apps/desktop/src/settings/pages/profiles/ProfilesPage.tsx` | List view; page-registry entry component |
| `apps/desktop/src/settings/pages/profiles/ProfilesPage.module.css` | Header/row/footnote styles ported from mockup lines 278–323 |
| `apps/desktop/src/settings/pages/profiles/ProfilesPage.test.tsx` | List + create tests (§4) |
| `apps/desktop/src/settings/pages/profiles/ProfileDetail.tsx` | Detail view incl. debounced-save hook |
| `apps/desktop/src/settings/pages/profiles/ProfileDetail.module.css` | Summary strip/sections/balance styles from mockup lines 325–398 |
| `apps/desktop/src/settings/pages/profiles/ProfileDetail.test.tsx` | Detail tests (§4) |

## 2. Component signatures & state

```ts
// ProfilesPage: registered as the "Profiles" page. Receives U07's
// PageComponentProps (see U07 CONTRACTS), which at minimum provides
// detail-stack navigation (openDetail(slug)/closeDetail) and registers the
// header's pageAction callback ("New profile").
export function ProfilesPage(props: PageComponentProps): JSX.Element
// queries: ['profiles'] -> ProfileSummary[]; ['settings'] (weight_control)
// mutations: duplicate(slug), delete(slug), create (create-only persistence of the new-profile
//   payload, retrying on ErrorDTO.code === 'conflict' with N+1)

export interface ProfileDetailProps {   // rendered by the detail stack
  slug: string
  onBack(): void                        // return to list
  onOpenSlug(slug: string): void        // navigate to a duplicate's detail
}
export function ProfileDetail(props: ProfileDetailProps): JSX.Element
// queries: ['profile', slug]; ['settings']
// local state: draft: ProfileDetail  (initialised from query; edits land here
//   synchronously; source of truth for rendering while a save is pending)
// useDebouncedProfileSave(draft): schedules profiles.save(draft) 300ms after
//   the last mutation; timer resets per change; flushes pending save on
//   unmount and on onBack; at most one in-flight save per burst.
```

New-profile payload: `{ slug: "profile_{N}", name: "profile {N}", builtin: false, core_share: 60, tier1_weights: {intelligence: 3, cost: 3, speed: 3}, tier2_weights: {}, picks: 0, last_used: "" }` with N starting at `profiles.length + 1`.

Sparkbar mapping (list rows): `core = tier1[k] > 0` in `[intelligence, cost, speed]` order, `task = tier2[k] > 0` in group-catalogue order, each as `{k, v}`; zero-weight keys omitted entirely.

## 3. Copy (exact strings)

| Where | String |
|---|---|
| Page title / blurb / action (U07 PAGE_META) | `Profiles` / `Built-in profiles are read-only; duplicate one to edit its weights.` / `New profile` |
| List kicker | `profiles` |
| Column headers | `name` · `weights` · `picks` · `used` (widths flex/120px/48px/64px + 132px actions) |
| Footnote | `Picks count every launch made with the profile — from the popover and from the wm CLI alike.` (`wm` in mono span) |
| Trash title (custom, list) | `Delete {slug}` |
| Trash title (builtin, both views) | `Built-in profile — cannot be deleted` |
| Trash title (custom, detail) | `Delete this profile` |
| Duplicate button | `Duplicate` (custom & list rows) / `Duplicate & edit` (builtin detail) |
| Builtin badge | `built-in · read-only` |
| Detail blurb (builtin) | `A built-in profile — its weights are read-only. Duplicate it to make a version you can change.` |
| Detail blurb (custom) | `Drag a weight to change how much this profile cares about each benchmark. Zero means the benchmark is ignored.` |
| Summary | `{weighted} of {total} benchmarks weighted · {picks} picks` |
| Section headers | `core benchmarks` / `task benchmarks`, each with `{pct}% of the score` |
| Weight value cell | `{v} / 5` when v>0; `ignored` when 0 |
| Balance caption | `core` … `{core} / {task}` … `task` |
| Toast: duplicate (list) | `duplicated {slug}` |
| Toast: duplicate (detail) | `editing {newSlug}` |
| Toast: delete | `deleted {slug}` |
| Toast: create | `new profile created` |

Back link label in detail: `Profiles`. Empty `last_used` renders `—`.

## 4. Test fixtures & assertions

Both test files use `createMockEngineHost` (U00 CONTRACTS §4) wrapped in a QueryClient + ToastProvider harness; `vi.useFakeTimers()` for debounce.

| Test | Fixture / action | Assertion |
|---|---|---|
| builtin read-only | open a builtin slug's detail; dispatch pointer events on weight rows and balance | `profiles.save` NEVER called; rows have no pointerdown handler; trash disabled with builtin title; badge + `Duplicate & edit` rendered |
| duplicate opens detail | detail view, click Duplicate | `profiles.duplicate(slug)` called once; `onOpenSlug` called with returned slug; toast `editing {newSlug}` |
| list duplicate stays | list view, click row Duplicate | `profiles.duplicate` called; no navigation; toast `duplicated {slug}`; row click not triggered (propagation stopped) |
| delete returns to list | custom detail, click trash | `profiles.delete(slug)` called; `onBack` invoked; toast `deleted {slug}` |
| new-profile conflict retry | seed mock so `profile_{N}` exists (first create rejects `conflict`) | `profiles.create` called again with `profile_{N+1}`; detail opened for it; toast `new profile created` |
| debounced save single call | custom detail; 3 weight edits within 300ms; advance timers | UI reflects each edit immediately; `profiles.save` called exactly once, with the final draft |
| flush on back | edit then immediately `onBack` before 300ms | pending save flushed exactly once |
| sparkbar weighted-only | profile with a zero-weight tier2 key | that key absent from `ProfileWeightSparkbar` props |

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
