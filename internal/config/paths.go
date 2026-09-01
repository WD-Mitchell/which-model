package config

import (
	"os"
	"path/filepath"
)

type Paths struct {
	UserConfigFile string
	ConfigDir      string
	CacheDir       string
	StateDir       string
}

func ResolvePaths(goos, home string, getenv func(string) string) Paths {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	// annex-d §4.5: darwin uses the macOS column unconditionally (XDG_*
	// ignored, matching platform convention); every other GOOS follows the
	// XDG base directories with their documented defaults.
	if goos == "darwin" {
		configDir := filepath.Join(home, "Library", "Application Support", "which-model")
		return Paths{
			UserConfigFile: filepath.Join(configDir, "config.toml"),
			ConfigDir:      configDir,
			CacheDir:       filepath.Join(home, "Library", "Caches", "which-model"),
			StateDir:       filepath.Join(configDir, "state"),
		}
	}
	configBase := getenv("XDG_CONFIG_HOME")
	if configBase == "" {
		configBase = filepath.Join(home, ".config")
	}
	cacheBase := getenv("XDG_CACHE_HOME")
	if cacheBase == "" {
		cacheBase = filepath.Join(home, ".cache")
	}
	stateBase := getenv("XDG_STATE_HOME")
	if stateBase == "" {
		stateBase = filepath.Join(home, ".local", "state")
	}
	configDir := filepath.Join(configBase, "which-model")
	return Paths{
		UserConfigFile: filepath.Join(configDir, "config.toml"),
		ConfigDir:      configDir,
		CacheDir:       filepath.Join(cacheBase, "which-model"),
		StateDir:       filepath.Join(stateBase, "which-model"),
	}
}

func UserConfigFile(goos, home string, getenv func(string) string) string {
	return ResolvePaths(goos, home, getenv).UserConfigFile
}

func ProjectConfigFile(cwd, home string) (string, bool) {
	dir := cwd
	for {
		candidate := filepath.Join(dir, ".which-model", "config.toml")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "", false
		}
		if dir == home {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
