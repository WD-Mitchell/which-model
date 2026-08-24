// Menu-bar icon for the recommended provider (the CodexBar pattern: the status
// item wears the mark of whoever is about to serve the request, not a fixed app
// glyph). tray.go swaps the icon whenever the top pick's provider changes and
// falls back to the app glyph when a provider has no mark.
//
// The marks are monochrome single-path SVGs; AppKit loads SVG into NSImage
// directly (measured: _NSSVGImageRep on macOS 13+), and a template image uses
// only the alpha channel, so their fill colour is irrelevant — the menu bar
// tints the silhouette for light/dark itself. Wails' SetTemplateIcon hands the
// bytes to [[NSImage alloc] initWithData:] (systemtray_darwin.m imageFromBytes),
// which is why an SVG works there at all.
//
// Each file's viewBox is inset by 14% per side against the source art so the
// mark does not fill the whole 22pt slot — see assets/providers/PROVENANCE.md.
package main

import (
	"embed"
	"strings"
)

// providerIconFS holds one <name>.svg per provider mark.
//
//go:embed assets/providers/*.svg
var providerIconFS embed.FS

// providerIconAliases maps a provider id onto the mark that represents it,
// where the two names differ. Keys are normalised ids (lower-case, letters and
// digits only — see normaliseProviderID), so "opencode_go" and "OpenCodeGo"
// both arrive here as "opencodego".
//
// The left side is which-model's provider vocabulary (config.toml [providers.*]
// plus anything added from models.dev); the right side is a file in
// assets/providers.
var providerIconAliases = map[string]string{
	"anthropic":        "claude",
	"claudecode":       "claude",
	"openai":           "codex",
	"chatgpt":          "codex",
	"githubcopilot":    "copilot",
	"github":           "copilot",
	"google":           "gemini",
	"googlegemini":     "gemini",
	"opencodezen":      "opencode",
	"alibabatokenplan": "alibaba",
	"qwen":             "qwencloud",
	"alibabacloud":     "alibaba",
	"moonshot":         "kimi",
	"moonshotai":       "kimi",
	"xaigrok":          "grok",
}

// normaliseProviderID reduces a provider id to letters and digits, so the
// separator style of a config key ("opencode_go", "z_ai") cannot decide whether
// an icon is found.
func normaliseProviderID(id string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// providerIcon returns the SVG for a provider's menu-bar mark, or nil when the
// provider has none — a provider added from models.dev may be anything, and the
// caller then keeps the app glyph rather than showing a blank status item.
func providerIcon(id string) []byte {
	name := normaliseProviderID(id)
	if name == "" {
		return nil
	}
	if alias, ok := providerIconAliases[name]; ok {
		name = alias
	}
	data, err := providerIconFS.ReadFile("assets/providers/" + name + ".svg")
	if err != nil {
		return nil
	}
	return data
}
