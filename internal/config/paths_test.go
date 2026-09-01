package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	getenv := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	g := getenv(map[string]string{})
	x := getenv(map[string]string{"XDG_CONFIG_HOME": "/xdg/cfg", "XDG_CACHE_HOME": "/xdg/ca", "XDG_STATE_HOME": "/xdg/st"})

	// F01-T4 cases 1-4: darwin uses the macOS column unconditionally —
	// XDG_* env vars are ignored (annex-d §4.5).
	darwin := ResolvePaths("darwin", "/Users/w", getenv(map[string]string{"XDG_CONFIG_HOME": "/xdg"}))
	if darwin.ConfigDir != "/Users/w/Library/Application Support/which-model" {
		t.Errorf("darwin ConfigDir = %q", darwin.ConfigDir)
	}
	if darwin.UserConfigFile != "/Users/w/Library/Application Support/which-model/config.toml" {
		t.Errorf("darwin UserConfigFile = %q", darwin.UserConfigFile)
	}
	if darwin.CacheDir != "/Users/w/Library/Caches/which-model" {
		t.Errorf("darwin CacheDir = %q", darwin.CacheDir)
	}
	if darwin.StateDir != "/Users/w/Library/Application Support/which-model/state" {
		t.Errorf("darwin StateDir = %q", darwin.StateDir)
	}

	// F01-T4 case 5-6: linux honours the XDG vars.
	linuxX := ResolvePaths("linux", "/home/w", x)
	if linuxX.ConfigDir != "/xdg/cfg/which-model" {
		t.Errorf("linuxX ConfigDir = %q", linuxX.ConfigDir)
	}
	if linuxX.UserConfigFile != "/xdg/cfg/which-model/config.toml" {
		t.Errorf("linuxX UserConfigFile = %q", linuxX.UserConfigFile)
	}
	if linuxX.CacheDir != "/xdg/ca/which-model" {
		t.Errorf("linuxX CacheDir = %q", linuxX.CacheDir)
	}
	if linuxX.StateDir != "/xdg/st/which-model" {
		t.Errorf("linuxX StateDir = %q", linuxX.StateDir)
	}

	// F01-T4 case 7-8: XDG defaults when unset.
	linux := ResolvePaths("linux", "/home/w", g)
	if linux.ConfigDir != "/home/w/.config/which-model" {
		t.Errorf("linux ConfigDir = %q", linux.ConfigDir)
	}
	if linux.CacheDir != "/home/w/.cache/which-model" {
		t.Errorf("linux CacheDir = %q", linux.CacheDir)
	}
	if linux.StateDir != "/home/w/.local/state/which-model" {
		t.Errorf("linux StateDir = %q", linux.StateDir)
	}
	if got := UserConfigFile("linux", "/home/w", g); got != linux.UserConfigFile {
		t.Errorf("UserConfigFile(linux) = %q, want %q", got, linux.UserConfigFile)
	}
	if got := UserConfigFile("darwin", "/Users/w", g); got != darwin.UserConfigFile {
		t.Errorf("UserConfigFile(darwin) = %q, want %q", got, darwin.UserConfigFile)
	}
}

// The real-process view on darwin must resolve under ~/Library, matching the
// spec table the desktop app and CLI rely on.
func TestResolvePathsDarwinRuntime(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only expectation")
	}
	home := t.TempDir()
	paths := ResolvePaths(runtime.GOOS, home, os.Getenv)
	if paths.CacheDir != filepath.Join(home, "Library", "Caches", "which-model") {
		t.Errorf("CacheDir = %q", paths.CacheDir)
	}
	if paths.StateDir != filepath.Join(home, "Library", "Application Support", "which-model", "state") {
		t.Errorf("StateDir = %q", paths.StateDir)
	}
}

func TestProjectConfigFile(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, home, repo, cwd string)
		want  string
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
			name:  "home-bound",
			setup: func(t *testing.T, home, repo, cwd string) {},
			want:  "", found: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			repo := filepath.Join(home, "repo")
			cwd := filepath.Join(repo, "deep", "sub")
			if err := os.MkdirAll(cwd, 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, repo, ".git/HEAD", "ref: refs/heads/main\n")
			tt.setup(t, home, repo, cwd)
			got, found := ProjectConfigFile(cwd, home)
			want := tt.want
			if want == "cwd/.which-model/config.toml" {
				want = filepath.Join(cwd, ".which-model", "config.toml")
			} else if want == "repo/.which-model/config.toml" {
				want = filepath.Join(repo, ".which-model", "config.toml")
			}
			if found != tt.found || got != want {
				t.Fatalf("ProjectConfigFile = (%q, %v), want (%q, %v)", got, found, want, tt.found)
			}
		})
	}

	home := t.TempDir()
	got, found := ProjectConfigFile(home, home)
	if found || got != "" {
		t.Fatalf("home path = (%q, %v), want (empty, false)", got, found)
	}
}
