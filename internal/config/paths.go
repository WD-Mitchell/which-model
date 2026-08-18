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
	configDir := filepath.Join(home, ".which-model")
	return Paths{
		UserConfigFile: filepath.Join(configDir, "config.toml"),
		ConfigDir:      configDir,
		CacheDir:       filepath.Join(configDir, "cache"),
		StateDir:       filepath.Join(configDir, "state"),
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
