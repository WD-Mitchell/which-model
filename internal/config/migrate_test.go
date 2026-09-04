package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// legacyHome builds a temp home containing the pre-#39 layout:
// ~/.which-model/{config.toml,cache,score…,state}.
func newLegacyHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	legacy := filepath.Join(home, ".which-model")
	if err := os.MkdirAll(filepath.Join(legacy, "cache", "catalog"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(legacy, "state", "pick"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "config.toml"), []byte("[usage]\nenabled = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "cache", "routes.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "state", "pick", "history.jsonl"), []byte("line1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// EnsureLegacyMigration moves a pre-platform-layout ~/.which-model tree into
// the resolved per-OS locations exactly once: the user config file, the
// cache tree, and the state tree. When the canonical config already exists
// the migration is skipped (the new layout wins); migration failures are
// returned to the caller for surfacing — never silently swallowed.
func TestEnsureLegacyMigrationMovesTree(t *testing.T) {
	home := newLegacyHome(t)
	paths := ResolvePaths("darwin", home, nil)

	if err := EnsureLegacyMigration(paths, home); err != nil {
		t.Fatalf("EnsureLegacyMigration: %v", err)
	}
	if !fileExists(paths.UserConfigFile) {
		t.Errorf("config not migrated to %s", paths.UserConfigFile)
	}
	data, err := os.ReadFile(paths.UserConfigFile)
	if err != nil || string(data) != "[usage]\nenabled = false\n" {
		t.Errorf("migrated config = %q, err = %v", data, err)
	}
	if !fileExists(filepath.Join(paths.CacheDir, "routes.json")) {
		t.Errorf("cache not migrated to %s", paths.CacheDir)
	}
	if !fileExists(filepath.Join(paths.StateDir, "pick", "history.jsonl")) {
		t.Errorf("state not migrated to %s", paths.StateDir)
	}
	// The legacy tree is gone (moved, not copied).
	if _, err := os.Stat(filepath.Join(home, ".which-model")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("legacy tree still present: %v", err)
	}
	// Idempotent: a second run is a no-op.
	if err := EnsureLegacyMigration(paths, home); err != nil {
		t.Fatalf("second EnsureLegacyMigration: %v", err)
	}
}

func TestEnsureLegacyMigrationSkipsWhenCanonicalExists(t *testing.T) {
	home := newLegacyHome(t)
	paths := ResolvePaths("darwin", home, nil)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigFile, []byte("[usage]\nenabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnsureLegacyMigration(paths, home); err != nil {
		t.Fatalf("EnsureLegacyMigration: %v", err)
	}
	// Canonical config untouched.
	data, err := os.ReadFile(paths.UserConfigFile)
	if err != nil || string(data) != "[usage]\nenabled = true\n" {
		t.Errorf("canonical config = %q, err = %v; must win over legacy", data, err)
	}
	// Legacy tree left in place for manual recovery.
	if _, err := os.Stat(filepath.Join(home, ".which-model")); err != nil {
		t.Error("legacy tree removed even though canonical config existed")
	}
}

func TestEnsureLegacyMigrationNoLegacyTree(t *testing.T) {
	home := t.TempDir()
	paths := ResolvePaths("darwin", home, nil)
	if err := EnsureLegacyMigration(paths, home); err != nil {
		t.Fatalf("EnsureLegacyMigration with no legacy tree: %v", err)
	}
	if _, err := os.Stat(paths.ConfigDir); !errors.Is(err, os.ErrNotExist) {
		t.Error("migration created the config dir without anything to migrate")
	}
}

// copyTree is the migration's move helper: recursive copy then source removal.
func TestCopyTreeMovesNestedFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	nested := filepath.Join(src, "a", "b")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mergeTree(src, filepath.Join(dst, "moved")); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "moved", "a", "b", "f.txt"))
	if err != nil || string(got) != "x" {
		t.Errorf("copied file = %q, err = %v", got, err)
	}
}

func TestLoadMigratesLinuxLegacyHome(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "defaults", true: "xdg"}[custom], func(t *testing.T) {
			home := newLegacyHome(t)
			xdg := t.TempDir()
			getenv := func(key string) string {
				if custom && (key == "XDG_CONFIG_HOME" || key == "XDG_CACHE_HOME" || key == "XDG_STATE_HOME") {
					return filepath.Join(xdg, key)
				}
				return ""
			}
			opts := LoadOptions{GOOS: "linux", Home: home, CWD: t.TempDir(), Getenv: getenv}
			cfg, err := Load(opts)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Usage.Enabled != UsageFalse {
				t.Fatalf("legacy usage lost: %s", cfg.Usage.Enabled)
			}
			paths := ResolvePaths("linux", home, getenv)
			for path, want := range map[string]string{paths.UserConfigFile: "[usage]\nenabled = false\n", filepath.Join(paths.CacheDir, "routes.json"): "{}", filepath.Join(paths.StateDir, "pick", "history.jsonl"): "line1\n"} {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != want {
					t.Errorf("migrated file %q = %q, %v", path, data, err)
				}
			}
			if _, err := os.Stat(filepath.Join(home, ".which-model")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("legacy tree remains: %v", err)
			}
			if _, err := Load(opts); err != nil {
				t.Fatal(err)
			}
		})
	}
}
