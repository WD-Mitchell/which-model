package hooks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Variant mirrors SPEC behaviour 9.
type Variant int

const (
	VariantAuto    Variant = iota // resolved by the CLI layer via usage.Enabled
	VariantUsage                  // all four hooks
	VariantNoUsage                // spawn-gate + model-audit only
)

// Installed returns the owned entries for a variant, with exact commands
// (SPEC behaviour 9). Commands reference `which-model hooks run <id>`, plus
// variant-B passthrough args (--no-usage --profile balanced_implementation
// --quiet / --last).
func Installed(v Variant) []Entry {
	if v == VariantNoUsage {
		return []Entry{
			{ID: "spawn-gate", Event: "UserPromptSubmit", Matcher: "*", Timeout: 10,
				Command: "which-model hooks run spawn-gate --no-usage --profile balanced_implementation --quiet"},
			{ID: "model-audit", Event: "PostToolUse", Matcher: "Task", Timeout: 5,
				Command: "which-model hooks run model-audit --last"},
		}
	}
	return []Entry{
		{ID: "usage-refresh", Event: "SessionStart", Matcher: "*", Timeout: 5, Command: "which-model hooks run usage-refresh"},
		{ID: "quota-guard", Event: "SessionStart", Matcher: "*", Timeout: 5, Command: "which-model hooks run quota-guard"},
		{ID: "spawn-gate", Event: "PreToolUse", Matcher: "Task", Timeout: 8, Command: "which-model hooks run spawn-gate"},
		{ID: "model-audit", Event: "PostToolUse", Matcher: "Task", Timeout: 5, Command: "which-model hooks run model-audit"},
	}
}

const (
	markerStart = "# === which-model managed hooks (do not edit) ===\n"
	markerEnd   = "# === end which-model managed hooks ===\n"
)

// genericEvent maps a claude event name to the generic hooks.toml event
// (SPEC behaviour 10).
func genericEvent(event string) string {
	switch event {
	case "SessionStart":
		return "session_start"
	case "PreToolUse", "UserPromptSubmit":
		return "pre_dispatch"
	case "PostToolUse":
		return "post_dispatch"
	}
	return event
}

// genericInjectAs returns the inject_as key for a hook id ("" = none).
func genericInjectAs(id string) string {
	switch id {
	case "quota-guard":
		return "context.which_model_quota_guard"
	case "spawn-gate":
		return "context.which_model_pick"
	}
	return ""
}

// Install merges the variant's entries into the target config (SPEC
// behaviour 10). target "claude" | "generic". Returns a human summary line
// per hook.
func Install(target string, entries []Entry, repoRoot string) ([]string, error) {
	switch target {
	case "claude":
		return installClaude(entries, repoRoot)
	case "generic":
		return installGeneric(entries, repoRoot)
	default:
		return nil, fmt.Errorf("unknown target: %s", target)
	}
}

func installClaude(entries []Entry, repoRoot string) ([]string, error) {
	settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
	manifestPath := filepath.Join(repoRoot, ".claude", "which-model-hooks.json")

	m, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	settingsFileWasAbsent := false
	if _, err := os.Stat(settingsPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		settingsFileWasAbsent = true
	}
	s, err := loadClaudeSettings(settingsPath)
	if err != nil {
		return nil, err
	}
	s.mergeOwned(entries)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return nil, err
	}
	if err := saveClaudeSettings(settingsPath, s); err != nil {
		return nil, err
	}
	manifest, err := SaveManifestClaude(m, entries)
	if err != nil {
		return nil, err
	}
	if settingsFileWasAbsent || manifest.CreatedSettings {
		manifest.CreatedSettings = true
	}
	if err := SaveManifest(manifestPath, manifest); err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("installed %s (claude: %s/%s, timeout %ds)", e.ID, e.Event, e.Matcher, e.Timeout))
	}
	return lines, nil
}

func installGeneric(entries []Entry, repoRoot string) ([]string, error) {
	hooksPath := filepath.Join(repoRoot, "agents", "hooks.toml")
	existing, err := os.ReadFile(hooksPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(markerStart)
	for _, e := range entries {
		fmt.Fprintf(&buf, "[[hooks]]\nevent = %q\ncommand = %q\ntimeout_ms = %d\non_failure = %q\n",
			genericEvent(e.Event), e.Command, e.Timeout*1000, "ignore")
		if inject := genericInjectAs(e.ID); inject != "" {
			fmt.Fprintf(&buf, "inject_as = %q\n", inject)
		}
	}
	buf.WriteString(markerEnd)
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(hooksPath, spliceMarkers(existing, buf.Bytes()), 0o644); err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("installed %s (generic: %s/%s, timeout %ds)", e.ID, genericEvent(e.Event), e.Matcher, e.Timeout))
	}
	return lines, nil
}

// spliceMarkers replaces the managed marker block in content with block
// (install), or deletes markers AND content when block is nil (remove, SPEC
// behaviour 11: no marker residue). Content outside the markers is preserved
// byte-for-byte; a missing marker block appends block at the end.
func spliceMarkers(content, block []byte) []byte {
	if start := bytes.Index(content, []byte(markerStart)); start >= 0 {
		rest := content[start+len(markerStart):]
		if end := bytes.Index(rest, []byte(markerEnd)); end >= 0 {
			end += start + len(markerStart) + len(markerEnd)
			if block == nil {
				return append(append([]byte{}, content[:start]...), content[end:]...)
			}
			out := append([]byte{}, content[:start]...)
			out = append(out, block...)
			return append(out, content[end:]...)
		}
	}
	if block == nil {
		return content // nothing to remove
	}
	out := append([]byte{}, content...)
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return append(out, block...)
}

// Remove deletes owned entries only (SPEC behaviour 11). Returns a human
// summary. Nothing installed → no-op success.
func Remove(target string, repoRoot string) ([]string, error) {
	switch target {
	case "claude":
		manifestPath := filepath.Join(repoRoot, ".claude", "which-model-hooks.json")
		m, err := LoadManifest(manifestPath)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return []string{"no which-model hooks installed (nothing to remove)"}, nil
		}
		settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
		s, err := loadClaudeSettings(settingsPath)
		if err != nil {
			return nil, err
		}
		s.removeOwned(m.Hooks)
		if s.empty() {
			if m.CreatedSettings {
				if err := os.Remove(settingsPath); err != nil && !os.IsNotExist(err) {
					return nil, err
				}
			} else {
				if err := saveClaudeSettings(settingsPath, s); err != nil {
					return nil, err
				}
			}
		} else {
			if err := saveClaudeSettings(settingsPath, s); err != nil {
				return nil, err
			}
		}
		if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return []string{"removed " + strconv.Itoa(len(m.Hooks)) + " which-model hook(s)"}, nil
	case "generic":
		hooksPath := filepath.Join(repoRoot, "agents", "hooks.toml")
		b, err := os.ReadFile(hooksPath)
		if err != nil {
			if os.IsNotExist(err) {
				return []string{"no which-model hooks installed (nothing to remove)"}, nil
			}
			return nil, err
		}
		removed := spliceMarkers(b, nil) // drop the block content
		if bytes.Equal(removed, b) {
			return []string{"no which-model hooks installed (nothing to remove)"}, nil
		}
		if len(bytes.TrimSpace(removed)) == 0 {
			if err := os.Remove(hooksPath); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		} else {
			if err := os.WriteFile(hooksPath, removed, 0o644); err != nil {
				return nil, err
			}
		}
		return []string{"removed which-model hooks from agents/hooks.toml"}, nil
	default:
		return nil, fmt.Errorf("unknown target: %s", target)
	}
}
