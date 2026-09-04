package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileAndLoad(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "valid",
			run: func(t *testing.T) {
				dir := t.TempDir()
				path := writeFile(t, dir, "valid.toml", "[usage]\nenabled = true\n[providers.claude]\nenabled = true\n")
				cfg, err := LoadFile(path)
				if err != nil {
					t.Fatalf("LoadFile: %v", err)
				}
				if cfg.Usage.Enabled != UsageTrue || !cfg.Providers["claude"].Enabled || cfg.Providers["claude"].Weight.String() != "1" {
					t.Fatalf("cfg = %#v", cfg)
				}
			},
		},
		{
			name: "missing file",
			run: func(t *testing.T) {
				_, err := LoadFile(filepath.Join(t.TempDir(), "missing.toml"))
				assertLoadError(t, err, KindNotFound, "", true)
			},
		},
		{
			name: "malformed",
			run: func(t *testing.T) {
				path := writeFile(t, t.TempDir(), "malformed.toml", "[usage\n")
				_, err := LoadFile(path)
				assertLoadError(t, err, KindInvalidTOML, "", true)
			},
		},
		{
			name: "bad usage",
			run: func(t *testing.T) {
				path := writeFile(t, t.TempDir(), "bad.toml", "[usage]\nenabled = \"banana\"\n")
				_, err := LoadFile(path)
				assertLoadError(t, err, KindInvalidValue, "usage.enabled", true)
			},
		},
		{
			name: "unknown usage key",
			run: func(t *testing.T) {
				path := writeFile(t, t.TempDir(), "bad.toml", "[usage]\nfoo = 1\n")
				_, err := LoadFile(path)
				assertLoadError(t, err, KindInvalidValue, "usage.foo", false)
			},
		},
		{
			name: "unknown provider key",
			run: func(t *testing.T) {
				path := writeFile(t, t.TempDir(), "bad.toml", "[providers.claude]\nbanana = 1\n")
				_, err := LoadFile(path)
				assertLoadError(t, err, KindInvalidValue, "providers.claude.banana", false)
			},
		},
		{
			name: "unreadable",
			run: func(t *testing.T) {
				if os.Geteuid() == 0 {
					t.Skip("permission checks are ineffective as root")
				}
				dir := t.TempDir()
				path := writeFile(t, dir, "unreadable.toml", "[usage]\nenabled = true\n")
				if err := os.Chmod(path, 0o000); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(path, 0o644)
				_, err := LoadFile(path)
				assertLoadError(t, err, KindUnreadable, "", false)
			},
		},
		{
			name: "explicit bypass",
			run: func(t *testing.T) {
				home := t.TempDir()
				repo := filepath.Join(home, "repo")
				cwd := filepath.Join(repo, "deep")
				if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(cwd, 0o755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, repo, ".which-model/config.toml", "[usage]\nenabled = false\n")
				writeFile(t, home, ".which-model/config.toml", "[usage]\nenabled = false\n")
				explicit := writeFile(t, t.TempDir(), "explicit.toml", "[usage]\nenabled = true\n")
				cfg, err := Load(LoadOptions{Path: explicit, CWD: cwd, Home: home, GOOS: "linux"})
				if err != nil || cfg.Usage.Enabled != UsageTrue {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "explicit missing",
			run: func(t *testing.T) {
				_, err := Load(LoadOptions{Path: filepath.Join(t.TempDir(), "missing.toml"), CWD: t.TempDir(), Home: t.TempDir(), GOOS: "linux"})
				assertLoadError(t, err, KindNotFound, "", true)
			},
		},
		{
			name: "explicit env override",
			run: func(t *testing.T) {
				t.Setenv("WHICH_MODEL_USAGE_ENABLED", "false")
				path := writeFile(t, t.TempDir(), "explicit.toml", "[usage]\nenabled = true\n")
				fake := func(name string) string {
					if name == "WHICH_MODEL_USAGE_ENABLED" {
						return "false"
					}
					return ""
				}
				cfg, err := Load(LoadOptions{Path: path, Getenv: fake, CWD: t.TempDir(), Home: t.TempDir(), GOOS: "linux"})
				if err != nil || cfg.Usage.Enabled != UsageFalse {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
		{
			name: "negative weight",
			run: func(t *testing.T) {
				path := writeFile(t, t.TempDir(), "bad.toml", "[providers.claude]\nweight = -1\n")
				_, err := Load(LoadOptions{Path: path, CWD: t.TempDir(), Home: t.TempDir(), GOOS: "linux"})
				assertLoadError(t, err, KindInvalidValue, "providers.claude.weight", false)
			},
		},
		{
			name: "flag path precedence",
			run: func(t *testing.T) {
				path := writeFile(t, t.TempDir(), "valid.toml", "[usage]\nenabled = true\n")
				fake := func(name string) string {
					if name == "WHICH_MODEL_CONFIG" {
						return "/elsewhere.toml"
					}
					return ""
				}
				cfg, err := Load(LoadOptions{Path: path, Getenv: fake, CWD: t.TempDir(), Home: t.TempDir(), GOOS: "linux"})
				if err != nil || cfg.Usage.Enabled != UsageTrue {
					t.Fatalf("cfg = %#v, err = %v", cfg, err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func assertLoadError(t *testing.T, err error, kind ErrorKind, key string, checkExit bool) {
	t.Helper()
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want ConfigError", err)
	}
	if ce.Kind != kind {
		t.Fatalf("Kind = %v, want %v", ce.Kind, kind)
	}
	if key != "" && ce.Key != key {
		t.Fatalf("Key = %q, want %q", ce.Key, key)
	}
	if checkExit && ce.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", ce.ExitCode())
	}
}

func TestLoadLeavesDocumentedAuthRuntimeEnvironmentToItsOwner(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"WHICH_MODEL_CLAUDE_OAUTH_TOKEN", "synthetic-native-auth-canary"},
		{"WHICH_MODEL_NONINTERACTIVE", "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.name, tc.value)
			path := writeFile(t, t.TempDir(), "config.toml", "[usage]\nbackend = \"native\"\n[providers.claude]\nenabled = true\n")
			cfg, err := Load(LoadOptions{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			data, err := cfg.MarshalTOML()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), tc.name) || strings.Contains(string(data), "synthetic-native-auth-canary") {
				t.Fatal("auth runtime input leaked into saved configuration")
			}
			if os.Getenv(tc.name) != tc.value {
				t.Fatal("config loading changed runtime input")
			}
		})
	}
	t.Setenv("WHICH_MODEL_CLAUDE_OAUTH_TOKENS", "synthetic-typo-canary")
	path := writeFile(t, t.TempDir(), "config.toml", "")
	if _, err := Load(LoadOptions{Path: path}); err == nil {
		t.Fatal("runtime-looking typo was accepted")
	}
}
