// Embed the built frontend for production use. The build step copies
// apps/desktop/dist/ to cmd/which-model-desktop/frontend/dist/ before
// go build, so the embed directive resolves. In dev mode (FRONTEND_DEVSERVER_URL
// set by wails3 dev), AssetFileServerFS transparently proxies to the dev
// server instead of serving from the embedded FS.
package main

import "embed"

// frontend holds the production Vite build output. The directory is populated
// by `task desktop:build` (cp -r apps/desktop/dist cmd/which-model-desktop/frontend/dist).
// go:embed requires the directory to exist at build time.
//
//go:embed all:frontend/dist
var frontend embed.FS
