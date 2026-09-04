---
kind: feature-spec
version: "1.0"
feature: B03-profiles
project: which-model-desktop
---

# B03-profiles — ProfileService

## 1. Purpose

`internal/service/profiles.go` is the profile surface of the service layer: it merges the 11 built-in engine profiles (`pick.Profiles`) with the user's custom `[profiles.*]` config sections into one `ProfileSummary`/`ProfileDetail` catalogue, attaches per-profile pick statistics (B11), and owns the mutations the Settings→Profiles page and popover "Save as profile" need: Create, Save, Duplicate, Delete. It also publishes the fixed 5-slug complexity scale the popover slider maps onto.

Depends on: B02 (Services core, weight helpers), B01 (`[profiles.*]` schema), B11 (pick stats). Inherits D00 + B00.

## 2. Behaviour

1. **Built-in set.** The built-ins are exactly the 11 keys of `pick.Profiles` (`internal/pick/profiles.go`), verbatim: `balanced_implementation`, `complex_action_execution`, `complex_implementation`, `financial_work`, `orchestration`, `planning`, `research`, `review`, `simple_action_execution`, `simple_implementation`, `ui_ux`. They are never written to config and always carry `Builtin: true`. A built-in's DTO `Name` equals its slug (that is what `catalog.Profile.Name` holds); a custom's `Name` also equals its slug (the `[profiles.<slug>]` schema stores no display name).

2. **CoreShare ↔ shares conversion.** `catalog.Profile.Tier1Share`/`Tier2Share` are decimal *percentages that must sum to exactly 100* (`pick.ValidateProfile` rule 2; e.g. `simple_implementation` is 80/20). The DTO `CoreShare` **is** the tier-1 share as an int — no ×100 scaling. DTO→engine: `Tier1Share = decimal(CoreShare)`, `Tier2Share = decimal(100 − CoreShare)` (B02 `engineProfile`). Engine→DTO: `CoreShare = round(Tier1Share)` to the nearest integer, then to the nearest multiple of 5 (ties round up), then clamped to [10, 90]. All 11 built-ins are already multiples of 5 in 60..80, so the rounding path is exact for them.

3. **Weight conversion.** Engine decimal weights (1–5) ↔ DTO int weights via B02 `dtoWeights`/`engineWeights`. DTO weight 0 means "absent": `engineWeights` drops the key, so a tier-1 axis set to 0 reaches `pick.ValidateProfile` as missing and fails with the engine's own message (the required tier-1 set is exactly {intelligence, cost, speed}, each in (0, 5]).

4. **List.** Returns every built-in followed by every custom. Order is fixed and canonical: built-ins first, sorted **alphabetically by slug** (the `pick.Profiles` map has no order), then customs sorted alphabetically by slug. `Picks`/`LastUsed` come from B11's aggregation of `<StateDir>/pick/history.jsonl` keyed by profile slug; a slug with no history gets `Picks: 0, LastUsed: ""`. The merge is disjoint by construction (§2.6 forbids custom slugs colliding with built-ins); if a stale config nonetheless contains a built-in slug under `[profiles.*]`, the built-in wins and the config entry is ignored (not deleted).

5. **Get.** Looks up the merged set by slug; unknown slug → `not_found`.

6. **Save.** Persists a custom profile at `[profiles.<slug>]`, creating or replacing. Fixed validation order (messages in CONTRACTS §5): (1) slug grammar `[a-z0-9_]+`, non-empty; (2) slug equal to a built-in slug → `builtin_readonly`; (3) `Name` equal to a built-in's name while the slug differs → `conflict`; (4) `CoreShare` in 10..90, step 5; (5) `engineWeights` on both tiers (weight > 5 rejected); (6) `engineProfile` → `pick.ValidateProfile` — any failure surfaces as `validation_failed` carrying the engine message verbatim. On success: write TOML (weights 1–5 only; 0-valued keys omitted), atomic persist, swap in-memory, emit `config:changed {"section":"profiles"}`. `Builtin`, `Picks`, `LastUsed` on the incoming DTO are ignored.

7. **Duplicate.** `Duplicate(slug)` copies an existing profile (built-in or custom) to a new custom. New slug = source slug + `_copy`; if taken (in the merged built-in ∪ custom set), `_copy_2`, `_copy_3`, … first free wins. The copy takes the source's weights and `CoreShare`; `Builtin: false`, `Name` = new slug, `Picks: 0`, `LastUsed: ""`. Persisted via Create, retrying its atomic `conflict` result if another writer claims the candidate slug and emits `config:changed {"section":"profiles"}`; the new `ProfileDetail` is returned so the UI can open it for editing (mockup `onPfDuplicate`).

8. **Delete.** Customs only: unknown slug → `not_found`; built-in slug → `builtin_readonly`. Removes the `[profiles.<slug>]` section, persists, emits `config:changed {"section":"profiles"}` (mockup `onPfDelete`).

9. **Complexity scale.** `ComplexityScale()` returns exactly, in this order (stops 0..4 of the popover slider): `simple_action_execution`, `simple_implementation`, `balanced_implementation`, `research`, `planning`. Every slug MUST exist in `pick.Profiles`; this is asserted once at package init and **panics** on failure (a broken build-time invariant, mirroring `pick.mustProfile`'s import-time crash — never a runtime error). The method itself never errors and returns a fresh copy each call.

10. **Read-only reads.** `List`, `Get`, `ComplexityScale` take RLock, never mutate config, never emit, and never write history.

## 3. Error behaviour

- All boundary errors map through B02 `toErrorDTO`: `errValidation` → `validation_failed`, `errBuiltinReadonly` → `builtin_readonly`, `errNotFound` → `not_found`, `errConflict` → `conflict`; persist failures → `io_error` naming the config path.
- Validation stops at the first failing check of the fixed order (§2.6) so messages are golden-testable.
- A failed Save/Duplicate/Delete leaves in-memory state untouched and emits nothing (B00 §2.2).
- Corrupt/invalid `[profiles.<slug>]` sections encountered by `List` (e.g. weights outside 1..5 edited by hand) are surfaced per B01's load validation, not silently skipped.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Built-in order | Alphabetical by slug (then customs alphabetical) | `pick.Profiles` is a map with no declared order; alphabetical is stable and locale-free |
| CoreShare semantics | `CoreShare == Tier1Share` (int percent); shares sum to 100 | `pick.ValidateProfile` rule 2 requires Tier1Share + Tier2Share == 100 exactly |
| CoreShare rounding | nearest int → nearest 5 (ties up) → clamp [10, 90] | DTO contract (D00) fixes 10..90 step 5; built-ins land exactly |
| Builtin-slug Save | `builtin_readonly`, checked before conflict | The UI's read-only affordance ("Built-in profiles are read-only; duplicate one to edit") makes this an attempted edit, not a naming clash |
| Builtin-name collision | `conflict` when slug differs | B00 CONTRACTS §6.4 collision rule, applied to the name axis |
| Duplicate suffix | `_copy`, `_copy_2`, … first free in merged set | Mockup `onPfDuplicate` uses `_copy`; numbered fallback keeps it total |
| Scale validated at init | `init()` panic on missing slug | Same invariant style as `pick.mustProfile`; a shipped binary can never return a dangling scale slug |
| Custom Name = slug | No display-name key in `[profiles.<slug>]` | B01 schema has none; keeps TOML minimal and rename = re-slug |

## 5. Deviations

- **B00 CONTRACTS §6.4** says a custom slug colliding with a built-in slug is `conflict` at save time. B03 narrows this for profiles: a Save targeting a built-in **slug** returns `builtin_readonly` (§2.6 check 2); `conflict` is reserved for a built-in **name** collision under a different slug (check 3). Rationale: the mockup treats built-in rows as read-only edit targets, so the honest code is "read-only", not "name taken".

## 6. Out of scope

- Profile-driven ranking and Overrides handling — B04. History file format and aggregation — B11. `[profiles.*]` TOML decode/validation — B01. Weight-helper implementations — B02. UI editing flows — U08.

## Review correction — #171: atomic profile creation

`Create(ctx, detail)` adds create-only semantics while Save remains an upsert. After slug validation, Create checks both built-in and custom membership under the same writer lock used for validation and persistence. Any occupied slug returns `conflict`, without changing bytes or emitting an event. Successful creation emits exactly one `config:changed` event. Save/Create mutate an independent config clone, publishing it only after successful persistence. Duplicate retries Create conflicts; concurrent copies cannot overwrite one another.


## Request validation precedence correction — #171 review

Save validates slug, built-in protection, reserved name, shares, and weights
before decoding unrelated stored profiles. Create reports conflict for built-in
slugs, validates request fields, then checks custom occupancy under the same
write lock. Malformed stored profiles cannot mask an intrinsic request error.
Pin `TestProfileRequestErrorsPrecedeStoredProfileDecode`.
