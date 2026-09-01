package whichmodel

import (
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// routes_providers.go seeds routing's ProviderInput.Windows, which
// BindWindowIDs uses to derive every route's gating windows. Those IDs must
// match what the selected backend's normalizer actually emits:
//   - native backend → the provider adapters' descriptor windows
//     (codex: 5h/weekly/credits, copilot: premium/chat/completions),
//   - codexbar backend → the CodexBar normalizer's IDs (codex: session,
//     copilot: monthly — claude is 5h/weekly in both).
//
// Issue #30: the declared set was codex session/weekly, copilot monthly —
// the CodexBar shapes — so under the default native backend the codex 5h
// lane was never gated and copilot had zero overlapping windows (pressure
// always unknown, never gated; least-used systematically preferred copilot).
// The declared set is therefore the union of both backends; each backend's
// normalizer emits a subset. These tests pin the union membership so the
// surfaces cannot drift apart silently again.
func TestRouteWindowIDsMatchNativeAdapters(t *testing.T) {
	descriptors := map[string]usage.Descriptor{}
	for _, d := range usage.All() {
		descriptors[d.ID] = d
	}
	for _, p := range knownProviders {
		t.Run(p.ID, func(t *testing.T) {
			d, ok := descriptors[p.ID]
			if !ok {
				t.Fatalf("no registered descriptor for %q", p.ID)
			}
			declared := map[string]bool{}
			for _, ws := range p.Windows {
				declared[ws.ID] = true
			}
			for _, ws := range d.Windows {
				if !declared[ws.ID] {
					t.Errorf("descriptor window %q missing from route provider windows %v", ws.ID, keysOf(declared))
				}
			}
		})
	}
}

func TestRouteWindowIDsMatchCodexBarBackend(t *testing.T) {
	want := map[string][]string{
		"claude":  {"5h", "weekly"},
		"codex":   {"session", "weekly"},
		"copilot": {"monthly"},
	}
	declared := map[string][]string{}
	for _, p := range knownProviders {
		ids := make([]string, 0, len(p.Windows))
		for _, ws := range p.Windows {
			ids = append(ids, ws.ID)
		}
		declared[p.ID] = ids
	}
	for provider, ids := range want {
		t.Run(provider, func(t *testing.T) {
			have := map[string]bool{}
			for _, id := range declared[provider] {
				have[id] = true
			}
			for _, id := range ids {
				if !have[id] {
					t.Errorf("codexbar window %q missing from route provider windows %v", id, declared[provider])
				}
			}
		})
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
