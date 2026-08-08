package routing

import (
	"strings"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// BindWindowIDs derives the gating windows for one route from the provider's
// descriptor windows (F11 usage.WindowSpec, file internal/usage/descriptor.go).
// Account-level windows (empty ModelScope) are included unconditionally; a
// model-scoped window is included when any ModelScope entry is a
// case-insensitive exact or substring match of modelID or model. A route
// matching zero scoped windows still gets the account-level windows; a route
// matching several gets all of them (annex-b §7.3).
// Result order: account-level windows first, then matched scoped windows,
// both in descriptor declaration order; duplicates removed (first occurrence
// wins).
func BindWindowIDs(providerWindows []usage.WindowSpec, modelID, model string) []string {
	windowIDs := make([]string, 0, len(providerWindows))
	seen := make(map[string]struct{}, len(providerWindows))
	appendWindow := func(id string) {
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		windowIDs = append(windowIDs, id)
	}

	for _, ws := range providerWindows {
		if len(ws.ModelScope) == 0 {
			appendWindow(ws.ID)
		}
	}

	lowerModelID := strings.ToLower(modelID)
	lowerModel := strings.ToLower(model)
	for _, ws := range providerWindows {
		if len(ws.ModelScope) == 0 {
			continue
		}
		matched := false
		for _, scope := range ws.ModelScope {
			if strings.EqualFold(scope, modelID) || strings.EqualFold(scope, model) ||
				strings.Contains(lowerModelID, strings.ToLower(scope)) ||
				strings.Contains(lowerModel, strings.ToLower(scope)) {
				matched = true
				break
			}
		}
		if matched {
			appendWindow(ws.ID)
		}
	}
	return windowIDs
}
