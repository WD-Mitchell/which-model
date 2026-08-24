---
kind: feature-contracts
version: "1.0"
feature: S01-scaffold
project: which-model-desktop
---

# S01-scaffold — Contracts

## 1. Package and files (lock table)

| File | Contents |
|---|---|
| `pnpm-workspace.yaml` | §2 verbatim |
| `package.json` (root) | §2 verbatim |
| `Taskfile.yml` (root) | §6 tasks |
| `apps/desktop/package.json` | §3 |
| `apps/desktop/tsconfig.json` | §4 app variant |
| `apps/desktop/vite.config.ts` | §5 |
| `apps/desktop/index.html`, `apps/desktop/settings.html` | §5 skeletons |
| `apps/desktop/src/main-popover.tsx`, `src/main-settings.tsx` | §5 stubs (content later replaced by S04/U05/U07) |
| `packages/core/{package.json,tsconfig.json,src/index.ts}` | §4 stubs |
| `packages/ui/{package.json,tsconfig.json,src/index.ts}` | §4 stubs |
| `cmd/which-model-desktop/build/config.yml` | §7 |
| `cmd/which-model-desktop/main.go` | §8 stub (body replaced by S02) |
| `go.mod` / `go.sum` | wails v3 require, exact alpha pin (SPEC §2.8) |
| `.gitignore` | additions in §9 only |

**Not owned by S01:** `apps/desktop/src/bindings/**`, `src/host/**`, `check-host-surface.mjs` (S04); tray/popover/settings-window Go files and icon assets (S02/S03); real contents of `packages/core|ui` sources beyond the stub `index.ts` (U01/U02); `internal/service/**` (B02).

## 2. Workspace root (exact contents)

```yaml
# pnpm-workspace.yaml
packages:
  - "apps/*"
  - "packages/*"
```

```json
{
  "name": "which-model-workspace",
  "private": true,
  "packageManager": "pnpm@11.5.0",
  "engines": { "node": ">=20" },
  "scripts": {
    "dev": "pnpm --filter desktop dev",
    "build": "pnpm -r build",
    "typecheck": "pnpm -r typecheck",
    "test": "pnpm -r test"
  }
}
```

## 3. `apps/desktop/package.json` (shape; implementer resolves current stable semver ranges)

```json
{
  "name": "desktop",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "typecheck": "tsc -b --noEmit",
    "test": "vitest run"
  },
  "dependencies": {
    "@fontsource/inter": "<pin>",
    "@tanstack/react-query": "<pin>",
    "@which-model/core": "workspace:*",
    "@which-model/ui": "workspace:*",
    "react": "<pin>",
    "react-dom": "<pin>",
    "zustand": "<pin>"
  },
  "devDependencies": {
    "@testing-library/react": "<pin>",
    "@vitejs/plugin-react": "<pin>",
    "jsdom": "<pin>",
    "typescript": "<pin>",
    "vite": "<pin>",
    "vitest": "<pin>"
  }
}
```

## 4. Package stubs

`packages/core/package.json` and `packages/ui/package.json` (identical shape; differences noted):

```json
{
  "name": "@which-model/core",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "main": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": { ".": "./dist/index.js" },
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "typecheck": "tsc -p tsconfig.json --noEmit",
    "test": "vitest run --passWithNoTests"
  },
  "devDependencies": { "typescript": "<pin>", "vitest": "<pin>" }
}
```

`@which-model/ui` additionally carries `"peerDependencies": { "react": ">=18", "react-dom": ">=18" }` (U00 CONTRACTS §1); `@dnd-kit/*` deps and the CSS `exports` map are added by U02.

Shared `tsconfig.json` (both packages, exact; app variant adds `"jsx": "react-jsx"`, `"types": ["vite/client"]`, drops `outDir`/`declaration`, sets `"noEmit": true`):

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "declaration": true,
    "outDir": "dist",
    "rootDir": "src",
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

`src/index.ts` in both packages: the single line `export {};` (placeholder so `tsc` emits; replaced by U01/U02).

## 5. Vite config and entries

```ts
// apps/desktop/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true },
  build: {
    rollupOptions: {
      input: {
        popover: resolve(__dirname, 'index.html'),
        settings: resolve(__dirname, 'settings.html'),
      },
    },
  },
  test: { environment: 'jsdom' },
})
```

`index.html` / `settings.html` (identical apart from title and script src):

```html
<!doctype html>
<html lang="en">
  <head><meta charset="utf-8" /><title>which-model</title></head>
  <body><div id="root"></div>
    <script type="module" src="/src/main-popover.tsx"></script></body>
</html>
```

`src/main-popover.tsx` (settings twin renders `which-model settings stub`):

```tsx
import { createRoot } from 'react-dom/client'
createRoot(document.getElementById('root')!).render(<div>which-model popover stub</div>)
```

## 6. Root `Taskfile.yml` (S00 CONTRACTS §2; semantics normative, YAML shape canonical)

```yaml
version: '3'
tasks:
  desktop:dev:
    cmds:
      - wails3 dev -config cmd/which-model-desktop/build/config.yml
  desktop:build:
    cmds:
      - pnpm -r build
      - wails3 build -config cmd/which-model-desktop/build/config.yml
  desktop:package:
    deps: [desktop:build]
    cmds:
      - wails3 package -config cmd/which-model-desktop/build/config.yml
  desktop:bindings:
    cmds:
      - wails3 generate bindings -d apps/desktop/src/bindings
```

`desktop:dev` relies on `wails3 dev` starting the frontend dev command; if the pinned alpha does not, the task instead runs `pnpm --filter desktop dev` as a background/parallel step before `wails3 dev` — implementer documents which shipped in a Deviations note. Flag names follow the pinned alpha (`wails3 <cmd> --help` is authoritative); task NAMES and effects are the contract.

## 7. `cmd/which-model-desktop/build/config.yml` (minimal; keys per pinned alpha docs)

```yaml
name: which-model
outputfilename: which-model-desktop
frontend:
  devserverurl: http://localhost:5173
  dist: ../../../apps/desktop/dist
```

Key spelling may vary across alphas; the four FACTS pinned here — app name `which-model`, binary `which-model-desktop`, dev URL `http://localhost:5173`, dist `apps/desktop/dist` — are normative regardless of key names. Deviations note records the final schema.

## 8. `cmd/which-model-desktop/main.go` (stub shape)

```go
// Package main is the Wails v3 desktop host. S01 stub: one empty window;
// S02 replaces this body with the full bootstrap (S00 SPEC §2.1).
package main

import "github.com/wailsapp/wails/v3/pkg/application"
func main() {
    app := application.New(application.Options{Name: "which-model"})
    app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
        Title: "which-model",
    })
    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

Exact option-struct fields track the pinned alpha; the contract is: app name `which-model`, one visible window titled `which-model`, loading the Vite frontend, no other features.

## 9. `.gitignore` additions (appended verbatim, existing rules untouched)

```gitignore
node_modules/
apps/desktop/dist/
bin/
cmd/which-model-desktop/build/bin/
```

## 10. Test fixtures / verification

No unit tests — this feature is verified by the G1 stub gate (SPEC §2.10): `pnpm install && pnpm -r build && go build ./...` green; `task desktop:dev` shows the stub window; `pnpm --filter desktop dev` serves the popover stub in a browser. `go vet ./...` must also stay green.

## Deviations (recorded at implementation, 2026-08-18)

- **Wails pin (SPEC §2.8):** `wails3@latest` resolved to **`v3.0.0-beta.9`** (the project has moved from alpha to beta). `go.mod` requires `github.com/wailsapp/wails/v3 v3.0.0-beta.9` exactly; the `wails3` CLI at the same version is installed via `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`.
- **`config.yml` schema (§7):** beta.9 does not use `name`/`outputfilename`/`frontend.devserverurl` keys. Its schema is `version` + `info:` (product metadata) + `dev_mode:` (an atterpac/refresh watcher config with an `executes` list). The four normative facts are preserved as follows: app name `which-model` → `info.productName` (and `application.Options.Name` in main.go); binary `which-model-desktop` → the `dev_mode.executes` build step outputs `bin/which-model-desktop`; dev URL `http://localhost:5173` → `wails3 dev ... -port 5173` (Taskfile) exports `FRONTEND_DEVSERVER_URL=http://localhost:5173`, which the beta.9 runtime honours, with Vite `strictPort: 5173` on the other side; dist `apps/desktop/dist` → produced by `vite build`; production embedding of it lands with S02+ (the S01 stub is dev-mode only).
- **`desktop:dev` shape (§6):** beta.9's `wails3 dev` does not start the frontend by itself; it runs the `dev_mode.executes` commands. Per the §6 fallback, the Vite dev server is started as the `background` execute (`pnpm --filter desktop dev`), the Go build as `blocking`, and the binary as `primary` — semantically "vite in parallel with the host", as allowed. The `-port 5173` flag was added to the task command.
- **`main.go` API (§8):** beta.9 removed `app.NewWebviewWindowWithOptions`; the stub uses the equivalent `app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "which-model"})`.
- **Dependency pins (§3):** react/react-dom ^19.2.8, @tanstack/react-query ^5.101.4, zustand ^5.0.15, @fontsource/inter ^5.3.0, vite ^7.1.3, vitest ^3.2.4, typescript ^5.9.3, @testing-library/react ^16.3.2, jsdom ^30.0.1. `@vitejs/plugin-react` is ^5.1.2 (v6 requires Vite 8). `@types/react` + `@types/react-dom` ^19.2.0 were added to devDependencies — required for `tsc -b` under `strict` (not listed in §3 but necessary for the pinned React 19 types).
- **`go.sum` repair:** the pre-existing `go.sum` carried a stale `github.com/pborman/getopt` go.mod hash that conflicted with sum.golang.org; the stale lines were removed and re-verified hashes recorded by `go get`. `go get` also bumped `github.com/spf13/pflag` v1.0.9 → v1.0.10 (wails transitive requirement).
- **pnpm workspace:** `pnpm-workspace.yaml` additionally carries `allowBuilds: { esbuild: true }` — pnpm 11 blocks dependency build scripts by default and esbuild's postinstall is required by Vite.
