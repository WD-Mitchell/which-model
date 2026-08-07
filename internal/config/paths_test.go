package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	getenv := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}

	darwin := ResolvePaths("darwin", "/Users/w", getenv(map[string]string{}))
	if darwin.ConfigDir != "/Users/w/Library/Application Support/which-model" {
		t.Fatalf("ConfigDir = %q", darwin.ConfigDir)
	}
	if darwin.CacheDir != "/Users/w/Library/Caches/which-model" {
		t.Fatalf("CacheDir = %q", darwin.CacheDir)
	}
	if darwin.StateDir != "/Users/w/Library/Application Support/which-model/state" {
		t.Fatalf("StateDir = %q", darwin.StateDir)
	}
	withXDG := ResolvePaths("darwin", "/Users/w", getenv(map[string]string{"XDG_CONFIG_HOME": "/xdg"}))
	if withXDG.ConfigDir != darwin.ConfigDir || withXDG.CacheDir != darwin.CacheDir || withXDG.StateDir != darwin.StateDir {
		t.Fatalf("darwin XDG values changed: %#v", withXDG)
	}

	linux := ResolvePaths("linux", "/home/w", getenv(map[string]string{
		"XDG_CONFIG_HOME": "/xdg/cfg",
	}))
	if linux.ConfigDir != "/xdg/cfg/which-model" || linux.UserConfigFile != "/xdg/cfg/which-model/config.toml" {
		t.Fatalf("linux config paths = %#v", linux)
	}
	linux = ResolvePaths("linux", "/home/w", getenv(map[string]string{
		"XDG_CONFIG_HOME": "/xdg/cfg",
		"XDG_CACHE_HOME":  "/xdg/ca",
		"XDG_STATE_HOME":  "/xdg/st",
	}))
	if linux.CacheDir != "/xdg/ca/which-model" || linux.StateDir != "/xdg/st/which-model" {
		t.Fatalf("linux cache/state paths = %#v", linux)
	}
	linux = ResolvePaths("linux", "/home/w", getenv(map[string]string{}))
	if linux.ConfigDir != "/home/w/.config/which-model" || linux.CacheDir != "/home/w/.cache/which-model" || linux.StateDir != "/home/w/.local/state/which-model" {
		t.Fatalf("linux defaults = %#v", linux)
	}
	if got := UserConfigFile("linux", "/home/w", getenv(map[string]string{})); got != "/home/w/.config/which-model/config.toml" || got != linux.UserConfigFile {
		t.Fatalf("UserConfigFile = %q", got)
	}
}

func TestProjectConfigFile(t *testing.T) {
	tests := []struct {
		name string
		setup func(t *testing.T, home, repo, cwd string)
		want string
		found bool
	}{
		{
			name: "cwd",
			setup: func(t *testing.T, home, repo, cwd string) {
				writeFile(t, cwd, ".which-model/config.toml", "[usage]\nenabled = true\n")
			},
			want: "cwd/.which-model/config.toml", found: true,
		},
		{
			name: "git-root",
			setup: func(t *testing.T, home, repo, cwd string) {
				writeFile(t, repo, ".which-model/config.toml", "[usage]\nenabled = true\n")
			},
			want: "repo/.which-model/config.toml", found: true,
		},
		{
			name: "above-git-root",
			setup: func(t *testing.T, home, repo, cwd string) {
				writeFile(t, filepath.Dir(repo), ".which-model/config.toml", "[usage]\nenabled = true\n")
			},
			want: "", found: false,
		},
		{
			name: "home-bound",
			setup: func(t *testing.T, home, repo, cwd string) {},
			want: "", found: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			repo := filepath.Join(home, "project")
			cwd := filepath.Join(repo, "sub", "deep")
			if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(cwd, 0o755); err != nil {
				t.Fatal(err)
			}
			tt.setup(t, home, repo, cwd)
			got, found := ProjectConfigFile(cwd, home)
			if found != tt.found {
				t.Fatalf("found = %v, want %v (path %q)", found, tt.found, got)
			}
			if !tt.found {
				if got != "" {
					t.Fatalf("path = %q, want empty", got)
				}
				return
			}
			var want string
			switch tt.name {
			case "cwd":
				want = filepath.Join(cwd, ".which-model", "config.toml")
			case "git-root":
				want = filepath.Join(repo, ".which-model", "config.toml")
			}
			if got != want {
				t.Fatalf("path = %q, want %q", got, want)
			}
		})
	}

	home := t.TempDir()
	got, found := ProjectConfigFile(home, home)
	if found || got != "" {
		t.Fatalf("home path = (%q, %v), want (empty, false)", got, found)
	}
}
