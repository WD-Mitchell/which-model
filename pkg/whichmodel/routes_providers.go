//go:build !nousage

package whichmodel

import "github.com/WD-Mitchell/which-model/internal/usage"

// F27 provider registry adapters (usage build): validation and descriptor
// surface for routes add/list/refresh (F11 registry).

// providerExists reports whether id is a registered provider id.
func providerExists(id string) bool {
	_, err := usage.Get(id)
	return err == nil
}

// providerIDs returns every registered provider id, sorted lexicographically.
func providerIDs() []string { return usage.IDs() }

// routeProviderInfo is the F27 view of one registry descriptor needed by the
// F18 production adapter (kind + window specs for routing.ProviderInput).
type routeProviderInfo struct {
	ID      string
	Kind    usage.Kind
	Windows []usage.WindowSpec
}

// routeProviders returns every registered provider descriptor.
func routeProviders() []routeProviderInfo {
	descs := usage.All()
	out := make([]routeProviderInfo, 0, len(descs))
	for _, d := range descs {
		out = append(out, routeProviderInfo{ID: d.ID, Kind: d.Kind, Windows: d.Windows})
	}
	return out
}
