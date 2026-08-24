package main

import (
	"strings"
	"testing"
)

// Every provider this binary ships support for, and every provider id the
// shipped config declares, must resolve to a mark — those are the ids that can
// actually turn up as a top pick on a normal install. A miss here means the
// menu bar silently falls back to the app glyph for a first-class provider.
func TestProviderIconCoversShippedProviders(t *testing.T) {
	for _, id := range []string{
		"claude", "codex", "copilot", "cursor",
		"opencode", "opencode_go", "opencode_zen",
		"z_ai", "alibaba_token_plan",
	} {
		if providerIcon(id) == nil {
			t.Errorf("providerIcon(%q) = nil, want a mark", id)
		}
	}
}

// The id's separator style must not decide whether a mark is found: config
// keys, models.dev slugs and route providers do not agree on one.
func TestProviderIconNormalisesIDs(t *testing.T) {
	want := providerIcon("opencodego")
	if want == nil {
		t.Fatal("providerIcon(\"opencodego\") = nil, want a mark")
	}
	for _, id := range []string{"opencode_go", "OpenCode-Go", "open code go", "OPENCODE_GO"} {
		if got := providerIcon(id); string(got) != string(want) {
			t.Errorf("providerIcon(%q) did not resolve to the opencodego mark", id)
		}
	}
}

// Aliases exist so a provider named for its vendor still gets the mark the
// product is known by.
func TestProviderIconAliases(t *testing.T) {
	cases := map[string]string{
		"anthropic":      "claude",
		"openai":         "codex",
		"github_copilot": "copilot",
		"google":         "gemini",
	}
	for id, target := range cases {
		got, want := providerIcon(id), providerIcon(target)
		if want == nil {
			t.Fatalf("alias target %q has no mark", target)
		}
		if string(got) != string(want) {
			t.Errorf("providerIcon(%q) did not resolve to %q's mark", id, target)
		}
	}
}

// An unknown provider yields nil rather than an empty or broken image: the
// caller keeps the app glyph, and Wails' SetTemplateIcon would index [0] of an
// empty slice.
func TestProviderIconUnknownIsNil(t *testing.T) {
	for _, id := range []string{"", "   ", "no-such-provider", "../../etc/passwd"} {
		if got := providerIcon(id); got != nil {
			t.Errorf("providerIcon(%q) = %d bytes, want nil", id, len(got))
		}
	}
}

// The marks are drawn by AppKit, which needs a viewBox to scale them into the
// 22pt menu-bar slot, and inset so they do not fill it edge to edge.
func TestProviderIconsAreInsetSVG(t *testing.T) {
	entries, err := providerIconFS.ReadDir("assets/providers")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	found := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		found++
		data, err := providerIconFS.ReadFile("assets/providers/" + e.Name())
		if err != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), err)
		}
		body := string(data)
		if !strings.Contains(body, "viewBox=\"") {
			t.Errorf("%s has no viewBox; AppKit cannot scale it", e.Name())
		}
		// Every mark must have been through the inset step, or it fills the
		// whole 22pt slot and reads as oversized (PROVENANCE.md).
		if !strings.Contains(body, "menu-bar mark: viewBox inset") {
			t.Errorf("%s is not marked as inset; re-prepare it per PROVENANCE.md", e.Name())
		}
	}
	if found == 0 {
		t.Fatal("no provider marks are embedded")
	}
}
