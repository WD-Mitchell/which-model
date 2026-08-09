package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Entry is one owned hook as recorded in the claude manifest and mirrored in
// settings.json.
type Entry struct {
	ID      string `json:"id"`
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Timeout int    `json:"timeout"`
	Command string `json:"command"`
}

// Manifest is .claude/which-model-hooks.json.
type Manifest struct {
	Version         int     `json:"version"`
	CreatedSettings bool    `json:"created_settings"`
	Hooks           []Entry `json:"hooks"`
}

// LoadManifest reads the ownership manifest. A missing file is nil, nil.
func LoadManifest(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return &m, nil
}

// SaveManifest writes the manifest compact (JSON + "\n"), mode 0600.
func SaveManifest(path string, m *Manifest) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// SaveManifestClaude assembles the manifest for a claude install (variant
// assembly): version 1, CreatedSettings preserved from an existing manifest
// (nil m → false), hooks from e. The caller folds in the fresh-file flag.
func SaveManifestClaude(m *Manifest, e []Entry) (*Manifest, error) {
	created := false
	if m != nil {
		created = m.CreatedSettings
	}
	return &Manifest{Version: 1, CreatedSettings: created, Hooks: e}, nil
}

// claudeSettings holds the repo .claude/settings.json (generic map to
// preserve foreign keys; semantic round-trip only).
type claudeSettings map[string]any

// loadClaudeSettings reads settings.json; a missing file is an empty map.
// Invalid JSON errors naming the path.
func loadClaudeSettings(path string) (claudeSettings, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return claudeSettings{}, nil
		}
		return nil, err
	}
	s := claudeSettings{}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// mergeOwned replaces/appends only the entries whose (event, matcher,
// command) are owned (SPEC behaviour 10): an existing matcher object gets
// the command appended to its hooks array (or is left untouched when the
// command is already present); a missing matcher object is appended.
func (s claudeSettings) mergeOwned(entries []Entry) {
	hooksAny, _ := s["hooks"].(map[string]any)
	if hooksAny == nil {
		hooksAny = map[string]any{}
		s["hooks"] = hooksAny
	}
	for _, e := range entries {
		eventArr, _ := hooksAny[e.Event].([]any)
		matcherObj := findMatcherObj(eventArr, e.Matcher)
		if matcherObj == nil {
			matcherObj = map[string]any{"matcher": e.Matcher}
			eventArr = append(eventArr, matcherObj)
			hooksAny[e.Event] = eventArr
		}
		hookArr, _ := matcherObj["hooks"].([]any)
		present := false
		for _, item := range hookArr {
			if m, ok := item.(map[string]any); ok && m["command"] == e.Command {
				present = true
				break
			}
		}
		if present {
			continue // idempotence: already owned, untouched
		}
		matcherObj["hooks"] = append(hookArr, map[string]any{
			"type":    "command",
			"command": e.Command,
			"timeout": e.Timeout,
		})
	}
}

// removeOwned deletes only the (event, matcher, command) triples listed; a
// matcher object is dropped when its hooks array becomes empty, an event key
// when its array becomes empty. Foreign entries are never touched.
func (s claudeSettings) removeOwned(entries []Entry) {
	hooksAny, _ := s["hooks"].(map[string]any)
	if hooksAny == nil {
		return
	}
	for _, e := range entries {
		eventArr, _ := hooksAny[e.Event].([]any)
		var newArr []any
		for _, item := range eventArr {
			m, ok := item.(map[string]any)
			if !ok {
				newArr = append(newArr, item)
				continue
			}
			if m["matcher"] != e.Matcher {
				newArr = append(newArr, item)
				continue
			}
			hookArr, _ := m["hooks"].([]any)
			var newHooks []any
			for _, h := range hookArr {
				if hm, ok := h.(map[string]any); ok && hm["command"] == e.Command {
					continue // owned: drop
				}
				newHooks = append(newHooks, h)
			}
			if len(newHooks) == 0 {
				continue // matcher object now empty: drop it
			}
			m["hooks"] = newHooks
			newArr = append(newArr, m)
		}
		if len(newArr) == 0 {
			delete(hooksAny, e.Event)
		} else {
			hooksAny[e.Event] = newArr
		}
	}
	if len(hooksAny) == 0 {
		delete(s, "hooks")
	}
}

// empty reports whether the settings map has no keys at all.
func (s claudeSettings) empty() bool { return len(s) == 0 }

// saveClaudeSettings writes settings.json with MarshalIndent 2-space plus a
// trailing newline, mode 0644.
func saveClaudeSettings(path string, s claudeSettings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// findMatcherObj returns the hooks-array object whose "matcher" field equals
// matcher, or nil.
func findMatcherObj(eventArr []any, matcher string) map[string]any {
	for _, item := range eventArr {
		if m, ok := item.(map[string]any); ok && m["matcher"] == matcher {
			return m
		}
	}
	return nil
}
