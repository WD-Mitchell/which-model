---
kind: feature-spec
version: "1.0"
feature: U14-page-favourites-agent
project: which-model-desktop
---

# U14-page-favourites-agent — Favourites and Agent Integration Pages

## 1. Purpose

Two settings pages: `FavouritesPage` (`apps/desktop/src/settings/pages/favourites/`) lists pinned models with unpin and an "Add model" page action; `AgentPage` (`apps/desktop/src/settings/pages/agent/`) holds the three agent-integration toggles and the shell-hook snippet box.

Depends on: U02, U07. Inherits D00 + U00. Mockup `onFavourites` and `onAgent` blocks of `specs/desktop/mockup/demo.dc.html` are normative.

## 2. Behaviour — FavouritesPage

1. **Data.** Query `['favourites']` → `host.favourites.list()`. Pin/unpin emit `config:changed`, which invalidates `['favourites']` (U00 CONTRACTS §5) — no manual refetch.

2. **Table.** Section label `pinned models`, then a header row (mono uppercase 9px): `model` at 200px fixed, `route` flex. One row per `Favourite`, in returned order: `model_name` (200px, 12.5px), `route_label` (mono 11px, 55% text — already formatted `provider · reasoning` per D00 §2), a `pinned` accent tag, and a right-aligned ghost `Unpin` button calling `host.favourites.unpin(route_key)`.

3. **Footnote** (verbatim): "A favourite is only offered when the profile's weights still rank it in range — pinning never forces a model that does not fit the task."

4. **Page chrome** (U07 registry): title "Favourites", blurb "Pinned models are offered first when they rank in range for the profile.", page action label `Add model`.

5. **Add model action.** On click: fetch `host.pick.rank({ profile_slug: 'balanced_implementation', holds: 0 })`; walk `candidates` in rank order and pin the FIRST whose `route_key` is not already a favourite `route_key` (dedupe by route key): `favourites.pin(route_key)` then toast `pinned {model_id}`. If every candidate is already pinned, do nothing but toast `every ranked model is already pinned`.

6. **Empty list** → `EmptyState` (U02) with text `no pinned models` (footnote still shown).

## 3. Behaviour — AgentPage

1. **"integrations" section.** Uppercase label `integrations`, then three toggle rows (name 12.5px bright-when-on, note 11px muted; `Toggle` right). Names/notes verbatim, bound to `GUISettings` fields:
   | Name | Note | Field |
   |---|---|---|
   | Expose as an MCP server | Agents can ask which-model for a pick mid-session instead of being told one up front. | `mcp_server` |
   | Write a CLAUDE.md hint | Adds a short note to new repositories describing the available profiles. | `claude_md_hint` |
   | Shell alias wm | wm research launches the top pick for a profile without the popover. | `shell_alias` |
   Binding is identical to U12 SPEC §2.1: read `['settings']`, write `settings.set` whole-struct with only that field changed, per-toggle handlers, no debounce.

2. **"shell hook" section.** Uppercase label `shell hook`, then a mono box (11px, radius 8, 6%-text background) rendering query `['snippets']` → `host.settings.shellSnippets()`: line 1 `Alias`, line 2 `Preview` (the `$ wm {slug}  →  {model_id}  ({route})` line the mockup shows as `agentSnippet`). Text is display-only.

3. **Page chrome**: title "Agent integration", blurb "How coding agents reach which-model without the popover.", no page action.

## 4. Error behaviour

- Rejected `unpin`/`pin`/`settings.set` toasts `ErrorDTO.message`; UI re-renders from query cache.
- Add-model: a rejected `pick.rank` toasts the message and pins nothing.
- `['favourites']` / `['settings']` / `['snippets']` query errors → inline error state with retry (U00 SPEC §3).

## 5. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Add-model ranking profile | Fixed slug `balanced_implementation`, `holds: 0` ([gui].holds) | The settings window has no "current profile"; a fixed built-in slug is the simplest deterministic choice (the mockup used the popover's live profile, unavailable here) |
| Dedupe key | `Favourite.route_key` vs `RankedModel.route_key` | Route key is the only serialised route identity (D00 CONTRACTS §1) |
| Toast copy | `pinned {model_id}` / `every ranked model is already pinned` | Mockup `pageAction` verbatim (model id, not name) |
| Snippet box content | `ShellSnippets.Alias` then `.Preview`, two lines | Preview alone (mockup) omits the actual hook to install; Alias + Preview shows both |
| Agent toggles binding | Same one-delta whole-struct write as U12 | Single settings write path; one contract to test |
