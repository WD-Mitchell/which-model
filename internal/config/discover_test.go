package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDiscovery(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, home, repo, cwd string) (string, map[string]string)
		check func(t *testing.T, cfg *Config, err error)
	}{
		{
			name: "defaults",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Usage.Enabled != UsageAuto || len(cfg.Providers) != 0 {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "user",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, home, ".which-model/config.toml", "[usage]\nenabled = false\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Usage.Enabled != UsageFalse {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "project walk",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, repo, ".which-model/config.toml", "[usage]\nenabled = true\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Usage.Enabled != UsageTrue {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "nearest project",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, cwd, ".which-model/config.toml", "[usage]\nenabled = true\n")
				writeFile(t, repo, ".which-model/config.toml", "[usage]\nenabled = false\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Usage.Enabled != UsageTrue {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "project beats user",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, home, ".which-model/config.toml", "[usage]\nenabled = false\n")
				writeFile(t, repo, ".which-model/config.toml", "[usage]\nenabled = \"auto\"\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Usage.Enabled != UsageAuto {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "provider merge",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, home, ".which-model/config.toml", "[providers.claude]\nenabled = true\npriority = 10\n")
				writeFile(t, repo, ".which-model/config.toml", "[providers.claude]\npriority = 5\n[providers.codex]\nenabled = true\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				claude := cfg.Providers["claude"]
				if err != nil || !claude.Enabled || claude.Priority != 5 || !cfg.Providers["codex"].Enabled {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "env path",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				path := writeFile(t, t.TempDir(), "env.toml", "[usage]\nenabled = true\n")
				writeFile(t, home, ".which-model/config.toml", "[usage]\nenabled = false\n")
				return "", map[string]string{"WHICH_MODEL_CONFIG": path}
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Usage.Enabled != UsageTrue {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "env path missing",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				return "", map[string]string{"WHICH_MODEL_CONFIG": filepath.Join(t.TempDir(), "missing.toml")}
			},
			check: func(t *testing.T, cfg *Config, err error) {
				var ce *ConfigError
				if cfg != nil || !errors.As(err, &ce) || ce.Kind != KindNotFound || ce.ExitCode() != 2 {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "bounded at git root",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, filepath.Dir(repo), ".which-model/config.toml", "[usage]\nenabled = true\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Usage.Enabled != UsageAuto {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "provider default deny",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, repo, ".which-model/config.toml", "[providers.claude]\npriority = 5\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || cfg.Providers["claude"].Enabled {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "unlisted absent",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, repo, ".which-model/config.toml", "[providers.claude]\nenabled = true\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				if err != nil || len(cfg.Providers) != 1 || !cfg.Providers["claude"].Enabled {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
			}
			},
		},
		{
			name: "malformed user",
			setup: func(t *testing.T, home, repo, cwd string) (string, map[string]string) {
				writeFile(t, home, ".which-model/config.toml", "[usage\n")
				return "", nil
			},
			check: func(t *testing.T, cfg *Config, err error) {
				var ce *ConfigError
				if cfg != nil || !errors.As(err, &ce) || ce.Kind != KindInvalidTOML {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			repo := filepath.Join(t.TempDir(), "work", "project")
			cwd := filepath.Join(repo, "sub", "deep")
			if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(cwd, 0o755); err != nil {
				t.Fatal(err)
			}
			path, values := tt.setup(t, home, repo, cwd)
			getenv := func(name string) string { return values[name] }
			cfg, err := Load(LoadOptions{Path: path, Getenv: getenv, CWD: cwd, Home: home, GOOS: "linux"})
			tt.check(t, cfg, err)
		})
	}
}
