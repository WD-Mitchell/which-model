//go:build !nousage

package fetch

import (
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// sourceMatchesKind reports whether one credential-chain link can produce
// the canonical source (the same mapping fetch.SourceFor applies to a
// resolved credential, and Descriptor.AuthSources relies on).
func sourceMatchesKind(kind usage.AuthKind, source usage.Source) bool {
	switch kind {
	case usage.AuthOAuthDeviceFlow, usage.AuthOAuthRefreshGrant:
		return source == usage.SourceOAuth
	case usage.AuthCLIShellOut, usage.AuthSubprocessRPC:
		return source == usage.SourceCLI
	case usage.AuthBrowserCookie:
		return source == usage.SourceWeb
	default:
		// env/file/keychain families all land on SourceAPI in SourceFor.
		return source == usage.SourceAPI
	}
}

// filterChainForSource restricts a provider's credential chain to the links
// that can produce the forced canonical source (issue #28 review P1):
// `usage copilot --source cli` must never resolve an env/API credential.
// An empty source (auto) or the cache pseudo-source returns the chain
// unchanged — cache-only enforcement happens at the cache layer.
func filterChainForSource(chain []usage.AuthSource, source usage.Source) []usage.AuthSource {
	if source == "" || source == usage.SourceCache {
		return chain
	}
	filtered := make([]usage.AuthSource, 0, len(chain))
	for _, link := range chain {
		if sourceMatchesKind(link.Kind, source) {
			filtered = append(filtered, link)
		}
	}
	return filtered
}

// cacheOnlySource reports whether the forced source pins the run to the
// usage cache (`--source cache`, D-7): no credentials, no live fetch.
func cacheOnlySource(source usage.Source) bool {
	return source == usage.SourceCache
}
