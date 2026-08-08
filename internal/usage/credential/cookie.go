//go:build !nousage

package credential

import (
	"context"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// CookieResolver is the interface browser-cookie extraction will implement
// in M5 (github.com/browserutils/kooky, annex-a §4). F12 ships no
// implementation; ResolveChain treats AuthBrowserCookie as ErrNotFound
// (SPEC D3 — stated explicitly: F12 performs no browser access).
type CookieResolver interface {
	Resolve(ctx context.Context, spec usage.CookieSpec) (usage.Credential, error)
}
