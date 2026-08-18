---
kind: feature-spec
version: "1.0"
feature: S01-scaffold
project: which-model-desktop
---

# S01-scaffold — Monorepo & Wails Scaffolding

## 1. Purpose

Bring the repo from "Go CLI only" to a building monorepo: pnpm workspace (root `package.json`, `pnpm-workspace.yaml`), the `apps/desktop` Vite React app with its two HTML entries rendering placeholder stubs, buildable `packages/core` and `packages/ui` stubs (so `pnpm -r build` is green before U01/U02 land), the root `Taskfile.yml` implementing the S00 CONTRACTS §2 task interface, the Wails project config under `cmd/which-model-desktop/build/`, a minimal `main.go` that opens one empty window, and `.gitignore` additions. No tray, no bindings, no services — S02/S04 own those.

Inherits `specs/desktop/global/*` and `specs/desktop/shell/*`. Depends on: D00 (spec only).

## 2. Behaviour

1. **Workspace.** `pnpm-workspace.yaml` at repo root declares `apps/*` and `packages/*` (D00 §2.5). Root `package.json` is `"private": true`, has no dependencies, pins pnpm via `packageManager`, and its `dev`/`build`/`typecheck`/`test` scripts delegate to `pnpm -r` (recursive; `dev` filters to `desktop` — CONTRACTS §2 exact content). It exists purely as workspace root; nothing imports it.

2. **Desktop app package.** `apps/desktop/package.json` (name `desktop`, private) depends on `react`, `react-dom`, `@tanstack/react-query`, `zustand`, `@fontsource/inter`, and the workspace packages `@which-model/core` + `@which-model/ui` via `workspace:*`; dev-deps `vite`, `@vitejs/plugin-react`, `typescript`, `vitest`, `@testing-library/react`, `jsdom` (CONTRACTS §3). State libraries live only here per D00 §2.1f.

3. **Two entries.** `apps/desktop/vite.config.ts` uses the react plugin and declares exactly two rollup inputs — `index.html` (popover) and `settings.html` — plus vitest config (`environment: 'jsdom'`). Each HTML file is a minimal skeleton: `<div id="root">` + a module script tag loading `src/main-popover.tsx` / `src/main-settings.tsx`. Each entry mounts via `createRoot` and renders the literal placeholder text `which-model popover stub` / `which-model settings stub` — real views arrive in U05/U07; the entry files are OWNED here but their content is replaced by later features (host-switch import added by S04).

4. **Package stubs.** `packages/core` and `packages/ui` get the exact package names, `type: module`, build script `tsc -p tsconfig.json`, strict ES2022 tsconfig, and a one-line `src/index.ts` (`export {}`) each — the shapes U00 CONTRACTS §1 pins (names, workspace protocol, build tool) are honoured now so U01/U02 replace contents without touching manifests. `packages/ui` declares its react peer-deps up front; `@dnd-kit/*` runtime deps are added by U02, not here.

5. **Task interface.** Root `Taskfile.yml` implements all four S00 CONTRACTS §2 tasks. `desktop:dev` runs the Vite dev server (`pnpm --filter desktop dev`, port 5173) and `wails3 dev` against it; `desktop:build` runs `pnpm -r build` then `wails3 build`; `desktop:package` runs `wails3 package`; `desktop:bindings` runs `wails3 generate bindings -d apps/desktop/src/bindings` (directory created by the task; populated meaningfully only after S04 registers services). All `wails3` invocations run from repo root against the config in clause 6.

6. **Wails project config.** `cmd/which-model-desktop/build/config.yml` is the single Wails v3 project config: app name `which-model`, `outputfilename: which-model-desktop`, frontend dev server URL `http://localhost:5173`, dist dir `apps/desktop/dist` (CONTRACTS §7 for exact minimal content). No other Wails config files (`Taskfile.yml` inside `cmd/` from `wails3 init` templates is NOT used — the root Taskfile is the only task entry point).

7. **Host stub.** `cmd/which-model-desktop/main.go` creates `application.New` with name `which-model` and one default WebviewWindow titled `which-model` loading the frontend, then `app.Run()`. No tray (S02), no settings window (S03), no services/bindings (S04), no config.Load bootstrap yet — S02 introduces the full S00 §2.1 bootstrap order and replaces this file's body.

8. **Dependency pinning.** `go.mod` gains `github.com/wailsapp/wails/v3` at an EXACT alpha version: the implementer pins whatever `wails3 version` resolves at implementation time (install per Decisions), records the chosen version in a `## Deviations`-style note appended to this spec's Decisions table row, and never uses a floating `latest`. `go build ./...` must stay green for the whole repo (the CLI is untouched).

9. **Ignore rules.** `.gitignore` gains: `node_modules/`, `apps/desktop/dist/`, `bin/`, `cmd/which-model-desktop/build/bin/`. The existing root-anchored `/dist/` rule does NOT cover `apps/desktop/dist/` — the new rule is required, and existing rules are not modified.

10. **Verification (G1 stub gate).** From a clean checkout: `pnpm install && pnpm -r build && go build ./...` succeeds; `task desktop:dev` opens the stub window titled `which-model` showing "which-model popover stub"; plain `pnpm --filter desktop dev` serves the popover stub at `http://localhost:5173` in an ordinary browser (S00 §2.7 browser mode works from day one).

## 3. Error behaviour

- Missing `wails3` or `task` CLI: `task desktop:*` fails with the shell's command-not-found — the Decisions table documents install commands; the Taskfile does not attempt auto-install.
- `pnpm -r build` must fail loudly (non-zero) if any workspace package fails to compile; no `|| true` anywhere in scripts or tasks.
- The main.go stub performs no error handling beyond `application.New` fatal errors surfacing as Wails' default panic/log — acceptable only until S02 introduces the fatal-dialog bootstrap (S00 §2.1).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| CLI availability (checked 2026-08-18) | `pnpm` 11.5.0 present at `~/Library/pnpm/bin/pnpm`; `wails3` NOT on PATH; `task` NOT on PATH | Repo reality check; drives the install steps below |
| wails3 install | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`, then pin go.mod to the exact version `wails3 version` reports; record it in Decisions when implementing | Tool and library versions must match; alpha churn breaks silently otherwise (S00 §4) |
| task install | `go install github.com/go-task/task/v3/cmd/task@latest` (or `brew install go-task`) | Not on PATH; required by the S00 §2 task interface |
| pnpm pin | `packageManager: "pnpm@11.5.0"` in root package.json | Version found on the dev machine; corepack/pnpm enforce it for every contributor |
| Node version floor | `"engines": { "node": ">=20" }` in root package.json (advisory, not enforced) | Vite 6 and vitest require Node 20+ |
| Root Taskfile only | Single `Taskfile.yml` at repo root; no per-directory Taskfiles from `wails3 init` templates | One task entry point (D00 §2.5); avoids drift between duplicated task definitions |
| Vite port | 5173 fixed (`server.port`, `strictPort: true`) | The Wails config hardcodes the dev URL; a silently shifted port would blank the webview |
| Entry ownership | S01 owns entry FILES; S04 rewrites their content for the host switch | Keeps the CONTRACTS §1 lock table disjoint per plan process rule 6 |
| Stub window | Plain `application.New` + one window, no tray | Proves toolchain end-to-end at G1 without pre-empting S02's tray design |

## 5. Out of scope

Tray/popover (S02); settings window (S03); services, bindings, host switch, `check-host-surface.mjs` (S04); shortcuts/login item (S05); real content of `packages/core|ui` (U01/U02); nocturne CSS vendoring (U02); any `internal/service` code (B02).
