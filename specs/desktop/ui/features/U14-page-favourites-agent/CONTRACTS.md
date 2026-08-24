---
kind: feature-contracts
version: "1.0"
feature: U14-page-favourites-agent
project: which-model-desktop
---

# U14-page-favourites-agent — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `apps/desktop/src/settings/pages/favourites/FavouritesPage.tsx` | `FavouritesPage` + registry entry (incl. page action handler) |
| `apps/desktop/src/settings/pages/favourites/FavouritesPage.module.css` | table/row styles |
| `apps/desktop/src/settings/pages/favourites/FavouritesPage.test.tsx` | fixtures §4 |
| `apps/desktop/src/settings/pages/agent/AgentPage.tsx` | `AgentPage` + registry entry |
| `apps/desktop/src/settings/pages/agent/AgentPage.module.css` | toggle-row/snippet styles |
| `apps/desktop/src/settings/pages/agent/AgentPage.test.tsx` | fixtures §4 |

## 2. Exports

```ts
// Both pages: no props; host via app context (U07).
// FavouritesPage queries: ['favourites']; page action additionally calls
// pick.rank({ profile_slug: 'balanced_implementation', holds: 0 }) on demand.
export function FavouritesPage(): JSX.Element

// AgentPage queries: ['settings'], ['snippets'].
export function AgentPage(): JSX.Element
```

The add-model constant is exported for tests:

```ts
// Profile used to determine "top-ranked" for the Add model action (SPEC §5 Decisions).
export const ADD_MODEL_PROFILE = 'balanced_implementation'
```

## 3. Copy (verbatim)

| Slot | Text |
|---|---|
| Favourites section label | `pinned models` |
| Table headers | `model` (200px), `route` (flex) |
| Row tag / button | `pinned` / `Unpin` |
| Footnote | `A favourite is only offered when the profile's weights still rank it in range — pinning never forces a model that does not fit the task.` |
| Toasts | `pinned {model_id}`, `every ranked model is already pinned` |
| Empty state | `no pinned models` |
| Agent section labels | `integrations`, `shell hook` |
| Toggle 1 | `Expose as an MCP server` / `Agents can ask which-model for a pick mid-session instead of being told one up front.` |
| Toggle 2 | `Write a CLAUDE.md hint` / `Adds a short note to new repositories describing the available profiles.` |
| Toggle 3 | `Shell alias wm` / `wm research launches the top pick for a profile without the popover.` |

## 4. Test fixtures (vitest + testing-library + `createMockEngineHost`)

FavouritesPage:
- Renders one row per `Favourite` with `model_name`, `route_label`, `pinned` tag; `Unpin` calls `favourites.unpin(route_key)` once.
- **Add-model dedupe.** With rank candidates `[A, B, C]` (route keys) and favourites `[A]`: the action calls `pick.rank` with `{ profile_slug: ADD_MODEL_PROFILE, holds: 0 }`, then `favourites.pin(B)` once, and toasts `pinned {B.model_id}`.
- With all candidates already pinned: no `pin` call; toast `every ranked model is already pinned`.
- Empty favourites → empty state `no pinned models`.
- After `config:changed` fires on the mock host, `favourites.list` is refetched.

AgentPage:
- **Toggle writes.** Each of the three toggles calls `settings.set` exactly once with the full current `GUISettings` in which only its field (`mcp_server` / `claude_md_hint` / `shell_alias`) is flipped (deep-equal on every other field).
- Snippet box renders `ShellSnippets.Alias` and `ShellSnippets.Preview` from `settings.shellSnippets()`.
