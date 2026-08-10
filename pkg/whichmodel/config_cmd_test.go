package whichmodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

// setupHome points HOME at a temp dir and returns the resolved user config
// path for the current GOOS (darwin: Library/Application Support; others:
// .config — specs/features/F01-config/SPEC.md §1, D4).
func setupHome(t *testing.T) (home, userConfig string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	userConfig = config.ResolvePaths(runtime.GOOS, home, nil).UserConfigFile
	return home, userConfig
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfigCmd(t *testing.T) {
	t.Run("path command", func(t *testing.T) {
		_, userConfig := setupHome(t)
		code, out, _ := captureExecute(t, []string{"config", "path"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if strings.TrimSpace(out) != userConfig {
			t.Errorf("stdout = %q, want %q", strings.TrimSpace(out), userConfig)
		}
	})

	t.Run("path with --config", func(t *testing.T) {
		setupHome(t)
		explicit := filepath.Join(t.TempDir(), "custom.toml")
		code, out, _ := captureExecute(t, []string{"config", "path", "--config", explicit})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if strings.TrimSpace(out) != explicit {
			t.Errorf("stdout = %q, want %q", strings.TrimSpace(out), explicit)
		}
	})

	t.Run("show text", func(t *testing.T) {
		_, userConfig := setupHome(t)
		writeFixture(t, userConfig, "[output]\ncolor = \"never\"\n")
		code, out, _ := captureExecute(t, []string{"config", "show"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "[output]") || !strings.Contains(out, "color = \"never\"") {
			t.Errorf("show output missing [output] section: %q", out)
		}
	})

	t.Run("show json", func(t *testing.T) {
		_, userConfig := setupHome(t)
		writeFixture(t, userConfig, "[output]\ncolor = \"never\"\n")
		code, out, _ := captureExecute(t, []string{"config", "show", "--json"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("stdout not JSON: %v (%q)", err, out)
		}
		if doc["schema_version"] != "2.0" {
			t.Errorf("schema_version = %v, want 2.0", doc["schema_version"])
		}
		sources, ok := doc["_sources"].(map[string]any)
		if !ok {
			t.Fatalf("_sources missing or not an object: %v", doc["_sources"])
		}
		ucf, _ := sources["user_config_file"].(string)
		if !strings.HasSuffix(ucf, "config.toml") {
			t.Errorf("_sources.user_config_file = %q, want suffix config.toml", ucf)
		}
	})

	t.Run("set creates file", func(t *testing.T) {
		_, userConfig := setupHome(t)
		code, out, _ := captureExecute(t, []string{"config", "set", "output.color", "never"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stdout %q)", code, out)
		}
		data, err := os.ReadFile(userConfig)
		if err != nil {
			t.Fatalf("config file not created: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "[output]") || !strings.Contains(content, "color = \"never\"") {
			t.Errorf("file content = %q", content)
		}
		if !strings.Contains(out, "wrote "+userConfig) {
			t.Errorf("stdout = %q, want `wrote %s`", out, userConfig)
		}
	})

	t.Run("set preserves keys", func(t *testing.T) {
		_, userConfig := setupHome(t)
		writeFixture(t, userConfig, "[usage]\nenabled = false\n")
		code, _, _ := captureExecute(t, []string{"config", "set", "output.color", "always"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		data, _ := os.ReadFile(userConfig)
		content := string(data)
		if !strings.Contains(content, "enabled = false") {
			t.Errorf("usage.enabled lost: %q", content)
		}
		if !strings.Contains(content, "color = \"always\"") {
			t.Errorf("new key missing: %q", content)
		}
	})

	t.Run("set value typing", func(t *testing.T) {
		_, userConfig := setupHome(t)
		for _, kv := range [][2]string{{"bands.weights.a", "2"}, {"bands.weights.b", "true"}, {"bands.weights.c", "hello"}} {
			code, _, _ := captureExecute(t, []string{"config", "set", kv[0], kv[1]})
			if code != 0 {
				t.Fatalf("set %s: exit = %d, want 0", kv[0], code)
			}
		}
		data, _ := os.ReadFile(userConfig)
		content := string(data)
		if !strings.Contains(content, "a = 2") {
			t.Errorf("int typing failed: %q", content)
		}
		if !strings.Contains(content, "b = true") {
			t.Errorf("bool typing failed: %q", content)
		}
		if !strings.Contains(content, "c = \"hello\"") {
			t.Errorf("string typing failed: %q", content)
		}
	})

	t.Run("set bad key", func(t *testing.T) {
		setupHome(t)
		for _, key := range []string{"", "a..b"} {
			code, _, errOut := captureExecute(t, []string{"config", "set", key, "x"})
			if code != 2 {
				t.Errorf("set %q: exit = %d, want 2", key, code)
			}
			if !strings.Contains(errOut, "[arguments]") {
				t.Errorf("set %q stderr = %q, want [arguments]", key, errOut)
			}
		}
	})

	t.Run("set array key rejected", func(t *testing.T) {
		_, userConfig := setupHome(t)
		writeFixture(t, userConfig, "[bands]\nweights = [1, 2]\n")
		code, _, _ := captureExecute(t, []string{"config", "set", "bands.weights", "3"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
	})

	t.Run("validate ok", func(t *testing.T) {
		_, userConfig := setupHome(t)
		writeFixture(t, userConfig, "[output]\ncolor = \"never\"\n")
		code, out, _ := captureExecute(t, []string{"config", "validate"})
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stdout %q)", code, out)
		}
		if strings.TrimSpace(out) != "config is valid" {
			t.Errorf("stdout = %q, want `config is valid`", out)
		}
	})

	t.Run("validate bad output section", func(t *testing.T) {
		_, userConfig := setupHome(t)
		// Unknown key inside F22's [output] section → UnmarshalKey rejects.
		writeFixture(t, userConfig, "[output]\ncolour = \"auto\"\n")
		code, _, errOut := captureExecute(t, []string{"config", "validate"})
		if code != 1 {
			t.Errorf("exit = %d, want 1 (annex-d §2.7)", code)
		}
		if errOut == "" {
			t.Error("stderr must carry the message")
		}
	})

	t.Run("show schema hook", func(t *testing.T) {
		setupHome(t)
		code, out, _ := captureExecute(t, []string{"config", "show", "--schema"})
		if code != 0 {
			t.Fatalf("hook exit = %d, want 0", code)
		}
		if !strings.Contains(out, `"type":"object"`) {
			t.Errorf("hook output is not a schema: %q", out)
		}
	})
}
