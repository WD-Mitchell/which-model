//go:build !nousage

package usage

import (
	"fmt"
	"sort"
)

// UnknownProviderError is the typed error returned by Get for unknown IDs.
// Match with errors.As(&usage.UnknownProviderError{}).
type UnknownProviderError struct {
	ID string
}

func (e *UnknownProviderError) Error() string {
	return fmt.Sprintf("unknown provider %q", e.ID)
}

// registry holds every provider descriptor (annex-a §5). Registration is
// init-time only, so no mutex is needed (SPEC D10).
type registry struct {
	descs map[string]Descriptor
	order []string
}

var defaultRegistry = &registry{descs: make(map[string]Descriptor)}

// Register adds a Descriptor to the global registry. Called from each
// provider package's init(). PANICS on a duplicate ID (programming error).
func Register(d Descriptor) {
	if _, exists := defaultRegistry.descs[d.ID]; exists {
		panic(fmt.Sprintf("usage: duplicate provider id %q", d.ID))
	}
	defaultRegistry.descs[d.ID] = d
	defaultRegistry.order = append(defaultRegistry.order, d.ID)
}

// Get returns the Descriptor for id, or *UnknownProviderError when unknown.
func Get(id string) (Descriptor, error) {
	if d, ok := defaultRegistry.descs[id]; ok {
		return d, nil
	}
	return Descriptor{}, &UnknownProviderError{ID: id}
}

// All returns every registered Descriptor, sorted lexicographically by ID.
func All() []Descriptor {
	ids := IDs()
	out := make([]Descriptor, 0, len(ids))
	for _, id := range ids {
		out = append(out, defaultRegistry.descs[id])
	}
	return out
}

// AuthSources returns the ordered, deduplicated canonical Source values the
// provider's credential chain can produce (F24 CONTRACTS §8) — the set of
// valid forced --source values for the provider. The kind→Source mapping
// mirrors fetch.SourceFor's canonical table; SourceCache is never included
// because it is stamped by the cache layer, not a chain origin. A
// KindLocalTool descriptor additionally declares SourceLocal.
func (d Descriptor) AuthSources() []Source {
	seen := make(map[Source]bool, len(d.Auth))
	out := make([]Source, 0, len(d.Auth))
	for _, link := range d.Auth {
		var s Source
		switch link.Kind {
		case AuthOAuthDeviceFlow, AuthOAuthRefreshGrant:
			s = SourceOAuth
		case AuthCLIShellOut, AuthSubprocessRPC:
			s = SourceCLI
		case AuthBrowserCookie:
			s = SourceWeb
		default:
			s = SourceAPI
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if d.Kind == KindLocalTool && !seen[SourceLocal] {
		out = append(out, SourceLocal)
	}
	return out
}

// IDs returns every registered provider ID, sorted lexicographically.
func IDs() []string {
	ids := make([]string, 0, len(defaultRegistry.descs))
	for id := range defaultRegistry.descs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
