//go:build !nousage

package codexbar

import (
	"strings"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// companySource recognizes the reviewed provider labels, not arbitrary suffixes
// or labels borrowed from another provider. See F14/APPROVED-CODEXBAR.md.
func companySource(provider, label string) (usage.Source, bool) {
	label = strings.ToLower(label)
	switch label {
	case "oauth", "api", "web", "cli", "local":
		return sourceFor(label), true
	}
	switch provider {
	case "antigravity":
		if label == "app" || label == "ide" {
			return usage.SourceLocal, true
		}
	case "codex":
		switch label {
		case "pat":
			return usage.SourceAPI, true
		case "openai-web":
			return usage.SourceWeb, true
		case "codex-cli":
			return usage.SourceCLI, true
		}
	case "claude":
		switch label {
		case "claude", "claude-cli":
			return usage.SourceCLI, true
		case "admin-api":
			return usage.SourceAPI, true
		}
	case "windsurf":
		if label == "windsurf-web" {
			return usage.SourceWeb, true
		}
	}
	return "", false
}

// CompanySourceMatches applies the same source selection rule to approved live
// results and their cached provenance. These two providers' upstream CLI mode
// includes local probes; retain SourceLocal rather than relabeling their evidence.
func CompanySourceMatches(provider string, actual, requested usage.Source) bool {
	if requested == "" || actual == requested {
		return true
	}
	return requested == usage.SourceCLI && actual == usage.SourceLocal &&
		(provider == "antigravity" || provider == "windsurf")
}
