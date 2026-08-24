// Bootstrap ensures the minimum config/state tree exists so the desktop app
// starts cleanly from a cold install (no prior CLI usage). Called before
// config.Load; creates directories and a default config.toml if absent.
package main

import (
	"os"
	"path/filepath"

	"github.com/WD-Mitchell/which-model/internal/config"
)

// defaultConfigTOML is the starter config written on first launch.
//
// It declares the two first-party providers up front. The Providers page lists
// providerUniverse = [providers.*] config keys ∪ providers named in the route
// table (internal/service/providers.go providerUniverseLocked), and the route
// table does not exist until `which-model routes refresh` has run against an
// authenticated CLI. Without these entries a cold install shows an EMPTY
// providers list with nothing to act on. Declaring them means the page has rows
// from the first launch, and enabling one is a visible switch rather than a
// hand-edit of TOML.
//
// IDs are the usage-provider descriptor IDs, not vendor names: "claude" is
// Anthropic (internal/usage/provider/claude, DisplayName "Claude") and "codex"
// is OpenAI (internal/usage/provider/codex, DisplayName "Codex").
//
// enabled = false is deliberate and is the engine's default-deny posture: a
// provider is never read until the user turns it on.
const defaultConfigTOML = `# which-model configuration
# Edit this file or use the Settings window to configure the app.
# See: https://github.com/WD-Mitchell/which-model#configuration

# Providers are default-deny: switch one on in Settings > Providers (or set
# enabled = true here) before it is read. Routes come from the model catalogue —
# run "which-model routes refresh" once a provider's CLI is authenticated.
[providers.claude]
enabled = false
priority = 1

[providers.codex]
enabled = false
priority = 2
`

// bootstrapConfig ensures the config directory and a default config.toml
// exist. Returns the resolved paths and any non-fatal warning. A failed
// bootstrap is not fatal — config.Load with no explicit path will still
// produce defaults.
func bootstrapConfig(paths config.Paths) string {
	dir := filepath.Dir(paths.UserConfigFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "could not create config directory: " + err.Error()
	}
	// State + cache dirs used by the service layer.
	_ = os.MkdirAll(paths.StateDir, 0o700)
	_ = os.MkdirAll(paths.CacheDir, 0o700)
	_ = os.MkdirAll(filepath.Join(paths.CacheDir, "catalog"), 0o700)
	_ = os.MkdirAll(filepath.Join(paths.StateDir, "pick"), 0o700)

	if _, err := os.Stat(paths.UserConfigFile); err == nil {
		return "" // already exists
	}
	if err := os.WriteFile(paths.UserConfigFile, []byte(defaultConfigTOML), 0o644); err != nil {
		return "could not write default config: " + err.Error()
	}
	return ""
}
