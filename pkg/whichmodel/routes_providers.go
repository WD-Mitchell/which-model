//go:build !nousage

package whichmodel

import "github.com/WD-Mitchell/which-model/internal/usage"

var knownProviders = []routeProviderInfo{
	{ID: "claude", Kind: usage.KindSubscription, Windows: []usage.WindowSpec{
		{ID: "5h", Label: "5-hour session", Unit: usage.UnitPercent},
		{ID: "weekly", Label: "Weekly", Unit: usage.UnitPercent},
	}},
	{ID: "codex", Kind: usage.KindSubscription, Windows: []usage.WindowSpec{
		{ID: "session", Label: "Session", Unit: usage.UnitPercent},
		{ID: "weekly", Label: "Weekly", Unit: usage.UnitPercent},
	}},
	{ID: "copilot", Kind: usage.KindSubscription, Windows: []usage.WindowSpec{
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
