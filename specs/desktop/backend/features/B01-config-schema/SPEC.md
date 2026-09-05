---
kind: feature-spec
version: "1.0"
feature: B01-config-schema
project: which-model-desktop
---

# B01-config-schema — GUI Config Sections & Atomic Write

## 1. Purpose

B01 gives the desktop app durable config state the CLI never needed: seven TOML sections in the user `config.toml` (`[profiles.*]`, `[harnesses.*]`, `[favourites]`, `[routes.disabled]`, `[groups.*]`, `[gui]`, `[auth]`) with typed accessors and validated save-side setters in `internal/config/gui.go` and `internal/config/auth.go`, and the promotion of the CLI's unexported `atomicWrite` helper into `internal/config/write.go` as `AtomicWriteFile`, so every config write in the system (CLI `config set` and every B02+ service mutation) goes through one durable code path.

Depends on: nothing (wave-3 root). Consumed by: B02–B10, D00 §2.3/§2.7.

## 2. Behaviour

1. **Decode path.** Every accessor decodes its section from the merged raw document via the existing `Config.UnmarshalKey` (`internal/config/unmarshal.go`), so file layering (user → project → env) is inherited unchanged. A section absent from the document yields the section's defaults (`[gui]` and `[auth]`) or an empty map/zero value (all others) with no error.

2. **Closed schema inside owned sections.** Unknown keys inside a B01-owned section are errors (the existing `Undecoded()` check in `UnmarshalKey` — `ConfigError{KindInvalidValue}` naming the first offending key). Unknown keys in map-valued positions (`tier1`, `tier2`, `[routes.disabled]` provider keys) are data, not schema, and are validated semantically per §2.4.

3. **Defaults.** `[gui]` decodes through an unexported pointered mirror so each of the 13 keys independently falls back to its default (CONTRACTS §4) when unset — never zero-valued. `[auth]` uses the same pointered-mirror rule and defaults `use_keychain = true`. `DefaultGUIConfig()` and `DefaultAuthConfig()` return the full defaults; their load accessors on an empty config return exactly those values.

4. **Validation, fixed order.** Each `Load*` accessor and each `Set*` setter validates with the exact check order and error strings of CONTRACTS §5, so messages are golden-testable. Map iteration for validation is sorted ascending (slugs, provider ids, tier2 keys); list entries are checked in list order. Common rules:
   - Slugs (profile, harness, group, provider keys, harness `providers` entries) match `[a-z0-9_]+`.
   - `core_share`: int, `10 <= v <= 90`, `v % 5 == 0`.
   - `tier1`: keys exactly `{intelligence, cost, speed}` (no more, no fewer); each value int 1..5.
   - `tier2`: each key ∈ `categories ∪ slugs([groups.*])`, where `categories` is passed by the caller (canonically `pick.CategoryNames` — B01 never imports `internal/pick`, see Decisions); each value int 1..5. `tier2` may be absent or empty.
   - `[favourites].pins`: each entry matches the D00 §1 route-key grammar (`provider/model_id@reasoning`, reasoning ∈ the D00 closed enum); duplicates rejected.
   - `[routes.disabled]`: provider keys are slugs; each entry matches `model_id "@" reasoning` (route key minus the provider segment); duplicates per provider rejected.
   - `[groups.<slug>].benchmarks`: non-empty; entries non-empty strings; duplicates rejected.
   - `[gui]` enums: `layout`, `weight_control`, `holds`, `shortcut`, `auto_update_frequency` per D00 `GUISettings` value sets.

5. **Save-side setters mutate the raw document.** `Set*`/`Delete*` methods validate (setters only; deletes are idempotent and never error), then write plain `map[string]any`/`[]any` values into `Config`'s raw document (via the existing internal `setKey` machinery), so a subsequent `MarshalTOML` emits them while preserving every key B01 does not own. Setters never touch disk; persistence is the caller's `MarshalTOML` → `AtomicWriteFile` sequence (B00 §2.2). `SetGUI` writes all 13 GUI keys and `SetAuth` writes `use_keychain` explicitly (a saved config is self-describing; defaults apply only to absent keys).

6. **Marshal round-trip.** `MarshalTOML`'s fixed render list is extended to `usage, auth, scoring, strategy, bands, catalog, output, gui, profiles, groups, harnesses, favourites, routes, providers`; after the fixed list, any remaining top-level raw tables render in ascending name order. Consequence (and the golden test of CONTRACTS §6): a document containing all seven new sections plus unknown sections/keys survives load → marshal with no key lost, and marshal is byte-stable (marshalling the reloaded output reproduces it byte-for-byte).

7. **Not env-addressable.** No additions to the `envKeys` vocabulary (`internal/config/env.go`). A `WHICH_MODEL_*` variable that resolves into a B01 section under the existing suffix rules fails at the existing eager checks (`ApplyEnv` / `UnmarshalKey` unmatched-overlay), exactly as for any unknown env key today.

8. **AtomicWriteFile.** `AtomicWriteFile(path, data)` creates parent directories (0755), writes a temp file in the target directory with mode 0600, fsyncs it, closes, renames into place, then fsyncs the directory. On failure before rename the temp file is removed and the destination is untouched. If rename succeeds but directory fsync fails, return `CommittedWriteError`: the destination contains the new bytes, while crash durability remains unconfirmed. `pkg/whichmodel/config_cmd.go`'s `config set` calls it in place of the deleted local `atomicWrite`; behaviour visible to CLI users is unchanged except the 0600 file mode and dir fsync. `setNestedKey`/`parseTOMLValue` stay CLI-local.

9. **Builtin collisions are NOT checked here.** `internal/config` cannot know builtin profile/group/harness slugs (no `internal/pick` import). Rejecting a custom slug that collides with a builtin is the service layer's save-time `conflict` per B00 §6.4 (B03/B05/B07).

## 3. Error behaviour

- All errors are `*config.ConfigError` with `Kind: KindInvalidValue` and `Key` set to the offending dotted key, rendering as `config: invalid value for <key>: <detail>` — details verbatim in CONTRACTS §5. Decode failures keep the existing `UnmarshalKey` error shapes.
- `Load*` on an absent section never errors. `Delete*` on an absent slug never errors. `AtomicWriteFile` returns raw pre-commit `os` errors or a `CommittedWriteError` wrapping a post-rename directory-sync error; `WriteCommitted` identifies the latter. Callers must distinguish rollback from an already-visible write.
- Validation stops at the first failing check (fixed order); no multi-error aggregation.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Category list injection | `LoadProfiles`/`SetProfile` take a `categories []string` param; config unions it with `[groups.*]` slugs internally | `internal/config` must not import `internal/pick` (base-layer package); group slugs already live in the same document |
| tier1 keys exactly 3 | Missing OR extra tier1 key is `validation_failed` | Plan §4 B01; the DTO's "0 = removed" semantics (D00) apply to tier2 only — tier1 axes always participate |
| Weights stored 1..5 only | 0 never persisted; setters receive maps already stripped of zeros by the service (B00 Decisions) | TOML stays minimal; engine validation requires (0,5] |
| GUI defaults per-key | Pointered decode mirror, F19 `TOMLConfig`-style | A config with only `layout = "list"` must not zero the other 12 keys |
| Auth storage default | `use_keychain = true`; false is an explicit opt-out | Prefer platform-protected storage while keeping a deterministic file-only setting |
| Marshal render list + trailing unknowns | Fixed order then sorted remainder (§2.6) | Deterministic bytes for the golden test; unknown top-level sections were silently dropped before — a GUI that rewrites the whole file must not eat user data |
| Builtin collision deferred to service | §2.9 | Layering; matches B00 §6.4 (conflict at save time) |
| One atomic writer | `AtomicWriteFile` shared by CLI and services; 0600 + dir fsync added during promotion | D00 §2.3; config may hold credential paths — tighten mode while touching it |
| Route-key grammar re-declared | Regexp in `gui.go` with a comment citing D00 CONTRACTS §1 (kept in sync by convention) | `service.ParseRouteKey` lives above config; B00 §2.3 re-declare rule |

## 5. Out of scope

- Merging builtins with these sections, DTO conversion, events, disk persistence — B02+ (services).
- Harness seeding, command templating, launch — B07. History/pick counts — B11.
- `wm`-style env addressability for the new sections; any change to `envKeys`.
- `config show`/`set` CLI surface changes beyond the `atomicWrite` swap.

## Deviations

- **B00 SPEC §5 (file table).** B01 additionally owns two bounded change sites outside its listed files: the section render list + trailing-unknown-sections loop in `internal/config/marshal.go` (§2.6 — without it the new sections are dropped on marshal), and the one-line `atomicWrite` → `config.AtomicWriteFile` re-point (plus deletion of the local helper) in `pkg/whichmodel/config_cmd.go` (§2.8). No other feature may edit those sites.


## Atomic-write correction — #179 review

The former all-errors-leave-destination-untouched guarantee was impossible after
a successful rename. `CommittedWriteError` wraps the underlying error, and
`WriteCommitted(error) bool` reports whether the new bytes are visible.
`TestAtomicWritePostCommitError` pins error classification and file contents.
Harness mutations publish this committed state and notify listeners even while
reporting the durability error.

### Harness discovery correction

Per the owner's request to detect configured providers, harnesses distinguish an omitted `providers` list (automatic) from an explicit list (manual). `provider_overrides` persists individual switches so automatic discovery cannot re-enable a provider the user turned off. This extends the original harness schema without changing the public DTO.

## Incomplete benchmark recommendations (2026-09-05)

`gui.allow_incomplete_recommendations` / `GUISettings.allow_incomplete_recommendations` is a persisted boolean, default false. General displays **Allow recommendations with incomplete benchmarks**. Saving it emits the existing settings event and invalidates ranking immediately. The rank service passes it as `pick.RankOptions.AllowIncomplete`; enabling it uses available core scores, disabling it requires complete core scores. Catalog scores remain visible in either mode.

Both carousel and list show `Missing benchmark data: <axes>. Ranked using available scores.` for partial recommendations, using absent intelligence/cost/speed fields in that order; measured zero is present data. No RankedModel schema extension is needed. Tests must cover off→on→off persistence and ranking, blank speed preservation, and the warning in both layouts.

## Correction (2026-09-05)

The Profiles / Use Cases correction in `specs/desktop/backend/features/B03-profiles/SPEC.md` governs the new persisted profile selection and desktop terminology. The DTO extension is canonical in `specs/desktop/global/CONTRACTS.md`. Settings navigation now has both Profiles (curated defaults) and Use Cases (ranking presets).
