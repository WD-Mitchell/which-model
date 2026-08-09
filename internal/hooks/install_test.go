// F29-T6/T7: install/remove tests (specs/features/F29-agent-hooks/TASKS.md
// tasks F29-T6 and F29-T7, remove half).
package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRepo makes a temp dir that looks like a repo root.
func fakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

// countHooks walks the settings.json shape and counts owned hook commands.
func countHooks(t *testing.T, settingsPath string) (perEvent map[string]int, commands []string) {
	t.Helper()
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	readJSON(t, settingsPath, &doc)
	perEvent = map[string]int{}
	for event, matchers := range doc.Hooks {
		for _, m := range matchers {
			perEvent[event] += len(m.Hooks)
			for _, h := range m.Hooks {
				commands = append(commands, h.Command)
				if h.Type != "command" {
					t.Errorf("%s: hook type = %q, want command", event, h.Type)
				}
			}
		}
	}
	return perEvent, commands
}

// Test 1: claude install on a fresh repo (variant A) → settings.json +
// manifest created; 4 entries; created_settings true; summary 4 lines.
func TestInstallClaudeFresh(t *testing.T) {
	root := fakeRepo(t)
	lines, err := Install("claude", Installed(VariantUsage), root)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if len(lines) != 4 {
		t.Errorf("summary has %d lines, want 4", len(lines))
	}
	perEvent, commands := countHooks(t, filepath.Join(root, ".claude", "settings.json"))
	if perEvent["SessionStart"] != 2 || perEvent["PreToolUse"] != 1 || perEvent["PostToolUse"] != 1 {
		t.Errorf("hooks per event = %v, want SessionStart 2, PreToolUse 1, PostToolUse 1", perEvent)
	}
	if len(commands) != 4 {
		t.Errorf("commands = %v, want 4 entries", commands)
	}
	var m Manifest
	readJSON(t, filepath.Join(root, ".claude", "which-model-hooks.json"), &m)
	if m.Version != 1 {
		t.Errorf("manifest version = %d, want 1", m.Version)
	}
	if !m.CreatedSettings {
		t.Error("manifest created_settings = false, want true")
	}
	if len(m.Hooks) != 4 {
		t.Errorf("manifest hooks = %d, want 4", len(m.Hooks))
	}
}

// Test 2: a second install of the same variant is byte-identical.
func TestInstallClaudeIdempotent(t *testing.T) {
	root := fakeRepo(t)
	if _, err := Install("claude", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	settings1, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest1, err := os.ReadFile(filepath.Join(root, ".claude", "which-model-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install("claude", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	settings2, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest2, err := os.ReadFile(filepath.Join(root, ".claude", "which-model-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(settings1) != string(settings2) {
		t.Error("re-install changed settings.json bytes")
	}
	if string(manifest1) != string(manifest2) {
		t.Errorf("re-install changed manifest bytes:\n%s\nvs\n%s", manifest1, manifest2)
	}
}

// Test 3: install over a foreign settings.json preserves foreign keys and
// entries; created_settings false.
func TestInstallClaudeForeign(t *testing.T) {
	root := fakeRepo(t)
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := `{"permissions":{"allow":["Bash(*)"]},"hooks":{"SessionStart":[{"matcher":"*","hooks":[{"type":"command","command":"echo hello","timeout":1}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install("claude", Installed(VariantUsage), root); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	var doc map[string]any
	readJSON(t, settingsPath, &doc)
	if _, ok := doc["permissions"]; !ok {
		t.Error("permissions key lost after install")
	}
	perEvent, commands := countHooks(t, settingsPath)
	if perEvent["SessionStart"] != 3 {
		t.Errorf("SessionStart hooks = %d, want 3 (foreign echo + 2 owned)", perEvent["SessionStart"])
	}
	found := false
	for _, c := range commands {
		if c == "echo hello" {
			found = true
		}
	}
	if !found {
		t.Errorf("foreign echo hook lost: %v", commands)
	}
	var m Manifest
	readJSON(t, filepath.Join(root, ".claude", "which-model-hooks.json"), &m)
	if m.CreatedSettings {
		t.Error("manifest created_settings = true, want false (settings.json pre-existed)")
	}
}

// Test 4: Installed(VariantNoUsage) → exactly 2 entries with exact commands.
func TestInstalledNoUsage(t *testing.T) {
	entries := Installed(VariantNoUsage)
	if len(entries) != 2 {
		t.Fatalf("Installed(VariantNoUsage) = %d entries, want 2", len(entries))
	}
	if entries[0].ID != "spawn-gate" || entries[0].Event != "UserPromptSubmit" || entries[0].Matcher != "*" || entries[0].Timeout != 10 {
		t.Errorf("spawn-gate entry = %+v", entries[0])
	}
	if entries[0].Command != "which-model hooks run spawn-gate --no-usage --profile balanced_implementation --quiet" {
		t.Errorf("spawn-gate command = %q", entries[0].Command)
	}
	if entries[1].ID != "model-audit" || entries[1].Event != "PostToolUse" || entries[1].Matcher != "Task" || entries[1].Timeout != 5 {
		t.Errorf("model-audit entry = %+v", entries[1])
	}
	if !strings.HasSuffix(entries[1].Command, "--last") {
		t.Errorf("model-audit command = %q, want --last suffix", entries[1].Command)
	}
}

// Test 5: generic install on a fresh repo → toml with 4 blocks, markers
// once, inject_as lines present.
func TestInstallGenericFresh(t *testing.T) {
	root := fakeRepo(t)
	lines, err := Install("generic", Installed(VariantUsage), root)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if len(lines) != 4 {
		t.Errorf("summary has %d lines, want 4", len(lines))
	}
	content, err := os.ReadFile(filepath.Join(root, "agents", "hooks.toml"))
	if err != nil {
		t.Fatalf("hooks.toml missing: %v", err)
	}
	s := string(content)
	if strings.Count(s, `event = "session_start"`) != 2 {
		t.Errorf("session_start events = %d, want 2\n%s", strings.Count(s, `event = "session_start"`), s)
	}
	if strings.Count(s, `event = "pre_dispatch"`) != 1 {
		t.Errorf("pre_dispatch events = %d, want 1", strings.Count(s, `event = "pre_dispatch"`))
	}
	if strings.Count(s, `event = "post_dispatch"`) != 1 {
		t.Errorf("post_dispatch events = %d, want 1", strings.Count(s, `event = "post_dispatch"`))
	}
	if strings.Count(s, `on_failure = "ignore"`) != 4 {
		t.Errorf("on_failure lines = %d, want 4", strings.Count(s, `on_failure = "ignore"`))
	}
	if !strings.Contains(s, `inject_as = "context.which_model_quota_guard"`) {
		t.Error("missing quota-guard inject_as")
	}
	if !strings.Contains(s, `inject_as = "context.which_model_pick"`) {
		t.Error("missing spawn-gate inject_as")
	}
	if strings.Count(s, markerStart) != 1 || strings.Count(s, markerEnd) != 1 {
		t.Errorf("marker lines = %d/%d, want 1/1", strings.Count(s, markerStart), strings.Count(s, markerEnd))
	}
}

// Test 6: generic install twice → byte-identical.
func TestInstallGenericIdempotent(t *testing.T) {
	root := fakeRepo(t)
	if _, err := Install("generic", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(root, "agents", "hooks.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install("generic", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(root, "agents", "hooks.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("re-install changed hooks.toml bytes")
	}
}

// Test 7: generic install over foreign toml preserves the foreign text.
func TestInstallGenericForeign(t *testing.T) {
	root := fakeRepo(t)
	hooksPath := filepath.Join(root, "agents", "hooks.toml")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := "# my own hooks\n[[hooks]]\nevent = \"session_start\"\ncommand = \"echo custom\"\ntimeout_ms = 100\non_failure = \"continue\"\n"
	if err := os.WriteFile(hooksPath, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install("generic", Installed(VariantUsage), root); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	s, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(s)
	if !strings.Contains(content, "echo custom") {
		t.Errorf("foreign text lost:\n%s", content)
	}
	if strings.Count(content, markerStart) != 1 {
		t.Errorf("marker start count = %d, want 1", strings.Count(content, markerStart))
	}
}

// Test 8: settings.json that is not valid JSON → error naming the path.
func TestInstallClaudeInvalidJSON(t *testing.T) {
	root := fakeRepo(t)
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("not json{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Install("claude", Installed(VariantUsage), root)
	if err == nil {
		t.Fatal("Install() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), settingsPath) {
		t.Errorf("error %q does not name the path %s", err, settingsPath)
	}
}

// Test 9: unknown target → error containing "unknown target".
func TestInstallUnknownTarget(t *testing.T) {
	root := fakeRepo(t)
	_, err := Install("codex", Installed(VariantUsage), root)
	if err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("Install(\"codex\") error = %v, want unknown target", err)
	}
}

// ---- F29-T7 remove tests ----

// Test 10: claude install (created_settings=true) then remove → both files
// absent; summary; second remove → nothing to remove.
func TestRemoveClaudeCreated(t *testing.T) {
	root := fakeRepo(t)
	if _, err := Install("claude", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	lines, err := Remove("claude", root)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if len(lines) != 1 || lines[0] != "removed 4 which-model hook(s)" {
		t.Errorf("remove summary = %v, want [removed 4 which-model hook(s)]", lines)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Errorf("settings.json still exists after remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "which-model-hooks.json")); !os.IsNotExist(err) {
		t.Errorf("manifest still exists after remove: %v", err)
	}
	lines, err = Remove("claude", root)
	if err != nil {
		t.Fatalf("second Remove() error = %v", err)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "nothing to remove") {
		t.Errorf("second remove summary = %v, want nothing to remove", lines)
	}
}

// Test 11: over a foreign settings.json: install then remove → foreign
// content preserved, manifest gone, no owned command remains.
func TestRemoveClaudeForeign(t *testing.T) {
	root := fakeRepo(t)
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := `{"permissions":{"allow":["Bash(*)"]},"hooks":{"SessionStart":[{"matcher":"*","hooks":[{"type":"command","command":"echo hello","timeout":1}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install("claude", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	if _, err := Remove("claude", root); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "which-model-hooks.json")); !os.IsNotExist(err) {
		t.Error("manifest still exists after remove")
	}
	var doc map[string]any
	readJSON(t, settingsPath, &doc)
	if _, ok := doc["permissions"]; !ok {
		t.Error("foreign permissions key lost after remove")
	}
	_, commands := countHooks(t, settingsPath)
	if len(commands) != 1 || commands[0] != "echo hello" {
		t.Errorf("commands after remove = %v, want only [echo hello]", commands)
	}
}

// Test 12: generic install then remove → toml deleted; second remove →
// nothing to remove.
func TestRemoveGeneric(t *testing.T) {
	root := fakeRepo(t)
	if _, err := Install("generic", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	lines, err := Remove("generic", root)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if len(lines) != 1 || lines[0] != "removed which-model hooks from agents/hooks.toml" {
		t.Errorf("remove summary = %v", lines)
	}
	if _, err := os.Stat(filepath.Join(root, "agents", "hooks.toml")); !os.IsNotExist(err) {
		t.Errorf("hooks.toml still exists after remove: %v", err)
	}
	lines, err = Remove("generic", root)
	if err != nil {
		t.Fatalf("second Remove() error = %v", err)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "nothing to remove") {
		t.Errorf("second remove summary = %v", lines)
	}
}

// Test 13: generic foreign toml: install then remove → foreign text intact,
// no marker residue.
func TestRemoveGenericForeign(t *testing.T) {
	root := fakeRepo(t)
	hooksPath := filepath.Join(root, "agents", "hooks.toml")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := "# my own hooks\n[[hooks]]\nevent = \"session_start\"\ncommand = \"echo custom\"\ntimeout_ms = 100\non_failure = \"continue\"\n"
	if err := os.WriteFile(hooksPath, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install("generic", Installed(VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	if _, err := Remove("generic", root); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	s, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.toml missing after remove: %v", err)
	}
	content := string(s)
	if !strings.Contains(content, "echo custom") {
		t.Errorf("foreign text lost:\n%s", content)
	}
	if strings.Contains(content, "# === which-model") {
		t.Errorf("marker residue remains:\n%s", content)
	}
}

// Test 14: Remove with unknown target → error containing "unknown target".
func TestRemoveUnknownTarget(t *testing.T) {
	root := fakeRepo(t)
	_, err := Remove("codex", root)
	if err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("Remove(\"codex\") error = %v, want unknown target", err)
	}
}
