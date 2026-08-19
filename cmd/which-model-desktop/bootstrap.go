// Bootstrap ensures the minimum config/state tree exists so the desktop app
// starts cleanly from a cold install (no prior CLI usage). Called before
// config.Load; creates directories and a default config.toml if absent.
package main

import (
	"os"
	"path/filepath"

	"github.com/WD-Mitchell/which-model/internal/config"
)

// defaultConfigTOML is the starter config written on first launch. It's
// intentionally minimal — the app works with all defaults.
const defaultConfigTOML = `# which-model configuration
# Edit this file or use the Settings window to configure the app.
# See: https://github.com/WD-Mitchell/which-model#configuration
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
