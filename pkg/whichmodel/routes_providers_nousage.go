//go:build nousage

package whichmodel

import "github.com/WD-Mitchell/which-model/internal/usage"

// F27 provider registry adapters (nousage build): the binary contains no
// provider adapters, so every id is unknown and the production adapter has
// no descriptors to seed (F11 stub surface).

// providerExists reports whether id is a registered provider id: never under
// nousage.
func providerExists(id string) bool { return false }

// providerIDs returns every registered provider id: none under nousage.
func providerIDs() []string { return nil }

// routeProviderInfo is the F27 view of one registry descriptor needed by the
// F18 production adapter (kind + window specs for routing.ProviderInput).
type routeProviderInfo struct {
	ID      string
	Kind    usage.Kind
	Windows []usage.WindowSpec
}

// routeProviders returns every registered provider descriptor: none under
// nousage (usage.Registry returns nil).
func routeProviders() []routeProviderInfo { return nil }
