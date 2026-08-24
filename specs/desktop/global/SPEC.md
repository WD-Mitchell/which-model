---
kind: feature-spec
version: "1.0"
feature: D00-global
project: which-model-desktop
---

# D00-global — which-model Desktop

## 1. Purpose

A cross-platform desktop menu-bar/tray application over the existing which-model engine: a tray icon opening a 400px popover (profile search, complexity scale, custom-weights editor, ranked results, launch-in-harness) and an 820×520 Settings window (8 pages + 5 detail views). The engine remains CLI-first; the desktop app links the engine's `internal/*` packages in-process through a new Wails-free service layer so a future hosted API/MCP (`which-model serve`) can mount the same layer.

**Normative sources, in precedence order:**
1. This spec tree (`specs/desktop/**`). Deepest spec wins; a child overriding a parent clause MUST carry a `## Deviations` section naming the overridden clause.
2. The mockup `specs/desktop/mockup/demo.dc.html` — normative for all visuals, copy, geometry, and interaction behaviour. Its trailing `<script>` block is the behavioural reference implementation. Its hardcoded demo data (`MODELS`, fake usage numbers, "412 models") is NOT normative — real data replaces it.
3. The existing CLI tree `specs/global/SPEC.md` + `specs/global/CONTRACTS.md` — still binds all Go code (layering, decimal discipline, security invariants, error-code style).

The design-system stylesheet is vendored at `specs/desktop/mockup/nocturne.css` (reference copy; the buildable copy lives at `packages/ui/src/theme/nocturne.css`).

## 2. Behaviour

1. **Layering.** (a) `internal/service` imports engine internals; it MUST NOT import `github.com/wailsapp/*` or `pkg/whichmodel`. (b) `cmd/which-model-desktop` is the ONLY Go package that imports Wails. (c) `packages/ui` components receive data and callbacks via props only — no imports from `apps/*`, no direct binding or network calls. (d) `packages/core` has zero runtime dependencies (types + interfaces + mock fixtures only). (e) `apps/desktop` is the ONLY JS package importing generated Wails bindings. (f) State libraries (TanStack Query, Zustand) appear only in `apps/*`.

2. **Ephemeral vs persisted state.** Unsaved weight edits in the popover ("Overrides") never touch `config.toml` and never write pick history. Only explicit actions persist: Save as profile, settings toggles, pins, group edits, provider toggles, launches.

3. **Config writes.** Every config mutation is a whole-file atomic write (`internal/config.AtomicWriteFile`) of the full TOML document produced by `config.MarshalTOML`, which preserves unknown keys. Read-modify-write happens under the service layer's single writer lock.

4. **Events.** Every mutating service method emits exactly one event (from the closed enum in CONTRACTS §3) AFTER its write has been persisted and the in-memory state swapped. The frontend refetches on events; it never assumes local mutation success ordering.

5. **Monorepo layout.**
   ```
   cmd/which-model-desktop/     Wails v3 host (Go main; build assets under build/)
   internal/service/            service layer (engine-side, Wails-free)
   internal/config/gui.go       new config sections; write.go atomic write
   apps/desktop/                Vite React app (two entries: index.html popover, settings.html)
   packages/core/               @which-model/core — types, EngineHost, events, mock
   packages/ui/                 @which-model/ui — theme + components
   pnpm-workspace.yaml          packages: ["apps/*", "packages/*"]
   package.json                 root scripts: dev, build, typecheck, test
   Taskfile.yml                 tasks desktop:dev|build|package|bindings
   specs/desktop/               this tree
   ```

6. **Naming.** Go service methods `PascalCase`; DTO JSON tags `snake_case`; event names `domain:verb` lowercase; TS type names identical to Go DTO names; TS methods camelCase of Go names; component files `PascalCase.tsx`; slugs match `[a-z0-9_]+`. CSS uses nocturne custom properties only (`--color-*`, `--font-*`, `--radius-*`, `--shadow-*`, `--space-*`).

7. **Numbers at the boundary.** The engine's decimal discipline (`specs/global`) ends at the service boundary: scores cross bindings as `float64`/`number` already rounded to 2 decimal places by the backend; percentages cross as integers. The UI performs no score arithmetic beyond display formatting (`toFixed(2)`; percentages rendered as-is with `%`).

8. **Errors at the boundary.** Every error crossing the service boundary is an `ErrorDTO{code, message}` with `code` from the closed enum (CONTRACTS §4). Messages are human-readable, never contain credentials or absolute home paths beyond the config path the UI legitimately displays.

9. **Platforms.** macOS is the primary target (menu-bar UX, notarization out of scope). Windows and Linux MUST compile and basically run (tray icon, windows open); polish is out of scope. Platform-specific host code lives in `cmd/which-model-desktop/*_{darwin,windows,linux}.go`.

10. **Build tags.** The desktop binary always builds WITHOUT `-tags nousage`; usage features are gated at runtime by `[usage]` config via `toggle.ResolveUsageEnabled`, never at compile time. The host blank-imports `internal/usage/provider/{claude,codex,copilot}` to populate the registry.

## 3. Error behaviour

- Service-layer methods return Go errors that the boundary mapper converts to `ErrorDTO` (CONTRACTS §4). Unknown/unexpected errors map to code `io_error` with a generic message; the raw error is logged host-side, never surfaced raw to the UI.
- The frontend surfaces every failed mutation as a toast with `ErrorDTO.message`; read failures render inline empty/error states, never blank screens.
- A missing scores CSV is an explicit, user-visible error (code `io_error`, message contains the expected path) — never silently treated as an empty catalog.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Shell framework | Wails v3 (alpha), version pinned in go.mod, generated bindings committed | Web frontend reuses the mockup's design system near-verbatim; Go host links `internal/*` in-process; single codebase for a future Windows build |
| Frontend stack | React + TypeScript + Vite; pnpm workspaces | Mockup logic is already React-shaped; workspace layout shared with the future Next.js website |
| Engine access | New `internal/service` layer; `pkg/whichmodel` bypassed | CLI wrappers are io.Writer-based with package-level mutable state — not concurrency-safe |
| Host location | Inside this repo/module | `internal/*` is not importable from an external module |
| Reuse boundary | `packages/ui` props-only + `packages/core` `EngineHost` interface | The future website swaps the transport (HTTP for bindings) without touching components |
| Route identity | Single serialized grammar `provider/model_id@reasoning` everywhere | One parser, one formatter; favourites/disables/keys stay interoperable |
| Score rounding | Backend rounds to 2dp before the boundary | Keeps decimal discipline in Go; UI stays arithmetic-free |
| Spec governance | Deepest spec wins + mandatory Deviations note | Parallel authors can specialise without silently contradicting parents |
