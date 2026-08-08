//go:build !nousage

package copilot

import (
	"context"
	"net/http"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// fetchUsage is the port of fetchCopilotUsage (copilot.mjs:111-120):
// GET CopilotUsageURL, allow-list [CopilotUsageURL], exactly six headers per
// CONTRACTS §3 (verbatim copilotUsageHeaders, copilot.mjs:93-97, including
// the Bearer scheme per SPEC D5). Non-200 → Error per
// mapStatus("GitHub Copilot", status); 200 → NormalizeUsage(body). The
// identity gate MUST have passed for this token in the same run before this
// call is made (SPEC §2.6).
func fetchUsage(ctx context.Context, token string, client *http.Client) ([]usage.Window, error) {
	status, raw, err := requestJSON(ctx, client, CopilotUsageURL, []string{CopilotUsageURL}, map[string]string{
		"Accept":                "application/vnd.github+json",
		"Authorization":         "Bearer " + token,
		"Editor-Version":        "vscode/1.96.2",
		"Editor-Plugin-Version": "copilot-chat/0.26.7",
		"User-Agent":            "GitHubCopilotChat/0.26.7",
		"X-GitHub-Api-Version":  APIVersion,
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, mapStatus("GitHub Copilot", status)
	}
	return NormalizeUsage(raw)
}
