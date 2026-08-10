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

// IDs returns every registered provider ID, sorted lexicographically.
func IDs() []string {
	ids := make([]string, 0, len(defaultRegistry.descs))
	for id := range defaultRegistry.descs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
