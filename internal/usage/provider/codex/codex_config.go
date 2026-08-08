//go:build !nousage

package codex

import (
	"regexp"
	"strings"
)

// sectionRe matches a provider section header (codex.mjs:8-35).
var sectionRe = regexp.MustCompile(`^\[model_providers\.([A-Za-z0-9_-]+)\]$`)

// assignmentRe is the RE2-compatible port of
// ^(model_provider|base_url)\s*=\s*(["'])(.*?)\2\s*(?:#.*)?$ — the closing
// quote and trailing comment are validated by parseAssignment.
var assignmentRe = regexp.MustCompile(`^(model_provider|base_url)\s*=\s*(["'])(.*)$`)

// parseAssignment extracts (key, value) from a quoted assignment line.
// It returns ok=false unless the value is quoted with the same quote char at
// both ends and the tail is whitespace plus an optional "# comment".
func parseAssignment(line string) (key, value string, ok bool) {
	m := assignmentRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	key, quote, rest := m[1], m[2], m[3]
	end := strings.Index(rest, quote)
	if end < 0 {
		return "", "", false
	}
	value = rest[:end]
	tail := strings.TrimSpace(rest[end+1:])
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return "", "", false
	}
	return key, value, true
}

// ParseConfig is the port of parseCodexConfig (codex.mjs:8-35): returns the
// active provider's [model_providers.<id>] base_url, else the root base_url,
// else "" (no error for absent values). Within a provider section only the
// first base_url assignment is recorded (F16-T2 case 9).
func ParseConfig(text string) string {
	section := ""
	var activeProvider string
	var rootBaseURL string
	providerURLs := make(map[string]string)
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			section = m[1]
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = "__other__"
			continue
		}
		key, value, ok := parseAssignment(line)
		if !ok {
			continue
		}
		if section == "" && key == "model_provider" {
			activeProvider = value
		}
		if section == "" && key == "base_url" {
			rootBaseURL = value
		}
		if section != "" && section != "__other__" && key == "base_url" {
			if _, exists := providerURLs[section]; !exists {
				providerURLs[section] = value
			}
		}
	}
	if activeProvider != "" {
		if u, exists := providerURLs[activeProvider]; exists {
			return u
		}
	}
	return rootBaseURL
}
