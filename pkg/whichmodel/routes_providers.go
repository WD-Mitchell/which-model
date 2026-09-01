//go:build !nousage

package whichmodel

import "github.com/WD-Mitchell/which-model/internal/usage"

// The builtin provider ids, kinds, and window specs for route production.
// The id → models.dev slug mapping lives in internal/routing
// (routing.CatalogueSlugFor) — the single source both this package and the
// desktop's provider service consume.
var knownProviders = []routeProviderInfo{
	{ID: "claude", Kind: usage.KindSubscription, Windows: []usage.WindowSpec{
		// Union of the native adapter's descriptor windows and the
		// CodexBar normalizer's IDs — the backend's normalizer emits a
		// subset, and BindWindowIDs can only bind what is declared here.
		// Native: 5h, weekly, sonnet_7d, opus_7d, oauth_apps_7d,
		// routines_7d, extra_usage. CodexBar: 5h, weekly.
		{ID: "5h", Label: "five hour", Unit: usage.UnitPercent},
		{ID: "weekly", Label: "seven day", Unit: usage.UnitPercent},
		{ID: "sonnet_7d", Label: "seven day Sonnet", Unit: usage.UnitPercent},
		{ID: "opus_7d", Label: "seven day Opus", Unit: usage.UnitPercent},
		{ID: "oauth_apps_7d", Label: "seven day OAuth apps", Unit: usage.UnitPercent},
		{ID: "routines_7d", Label: "seven day Routines", Unit: usage.UnitPercent},
		{ID: "extra_usage", Label: "Extra usage", Unit: usage.UnitUSD},
	}},
	{ID: "codex", Kind: usage.KindSubscription, Windows: []usage.WindowSpec{
		// Native: 5h, weekly, credits. CodexBar: session, weekly.
		{ID: "5h", Label: "primary window", Unit: usage.UnitPercent},
		{ID: "session", Label: "Session", Unit: usage.UnitPercent},
		{ID: "weekly", Label: "Weekly", Unit: usage.UnitPercent},
		{ID: "credits", Label: "credits", Unit: usage.UnitCredits},
	}},
	{ID: "copilot", Kind: usage.KindSubscription, Windows: []usage.WindowSpec{
		// Native: premium, chat, completions (requests). CodexBar: monthly.
		{ID: "premium", Label: "premium interactions", Unit: usage.UnitRequests},
		{ID: "chat", Label: "chat", Unit: usage.UnitRequests},
		{ID: "completions", Label: "completions", Unit: usage.UnitRequests},
		{ID: "monthly", Label: "Monthly", Unit: usage.UnitPercent},
	}},
}

func providerExists(id string) bool {
	for _, provider := range knownProviders {
		if provider.ID == id {
			return true
		}
	}
	return false
}

func providerIDs() []string {
	ids := make([]string, 0, len(knownProviders))
	for _, provider := range knownProviders {
		ids = append(ids, provider.ID)
	}
	return ids
}

type routeProviderInfo struct {
	ID      string
	Kind    usage.Kind
	Windows []usage.WindowSpec
}

func routeProviders() []routeProviderInfo {
	out := make([]routeProviderInfo, len(knownProviders))
	copy(out, knownProviders)
	return out
}
