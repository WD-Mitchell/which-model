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

	paths := ResolvePaths("darwin", "/Users/w", getenv(map[string]string{}))
	if paths.ConfigDir != "/Users/w/.which-model" {
		t.Fatalf("ConfigDir = %q", paths.ConfigDir)
	}
	if paths.UserConfigFile != "/Users/w/.which-model/config.toml" {
		t.Fatalf("UserConfigFile = %q", paths.UserConfigFile)
	}
	if paths.CacheDir != "/Users/w/.which-model/cache" {
		t.Fatalf("CacheDir = %q", paths.CacheDir)
	}
	if paths.StateDir != "/Users/w/.which-model/state" {
		t.Fatalf("StateDir = %q", paths.StateDir)
	}
	if got := UserConfigFile("darwin", "/Users/w", getenv(map[string]string{})); got != "/Users/w/.which-model/config.toml" {
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
