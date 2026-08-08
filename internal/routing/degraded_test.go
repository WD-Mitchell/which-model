package routing

import (
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

const degradedWarning = "live provider model lists unavailable; routes built from models-dev and user-declared sources only"

func countWarning(warnings []string, want string) int {
	n := 0
	for _, w := range warnings {
		if w == want {
			n++
		}
	}
	return n
}

func TestProduceRoutesDegraded(t *testing.T) {
	t.Run("case 1: degraded skips LiveModels, keeps ModelsDev", func(t *testing.T) {
		in := Input{
			Providers: []ProviderInput{
				{
					Provider:  "anthropic",
					Kind:      usage.KindSubscription,
					ModelsDev: []ModelEntry{{ModelID: "m1", Name: "Model One"}},
					LiveModels: []ModelEntry{{ModelID: "m2", Name: "Model Two"}},
				},
			},
			CatalogRows: []identity.Identity{
				{Model: "Model One", Reasoning: "default"},
				{Model: "Model Two", Reasoning: "default"},
			},
			Degraded: true,
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		if !hasRoute(result.Routes, "anthropic", "m1") {
			t.Errorf("Routes = %+v, want m1 routed", result.Routes)
		}
		if hasRoute(result.Routes, "anthropic", "m2") {
			t.Errorf("Routes = %+v, want m2 (live-only) absent", result.Routes)
		}
		for _, route := range result.Routes {
			if route.ModelID == "m1" && route.Provenance != ProvenanceModelsDev {
				t.Errorf("m1 Provenance = %v, want %v", route.Provenance, ProvenanceModelsDev)
			}
		}
	})

	t.Run("case 2: model only in LiveModels is unrouted, not ambiguity", func(t *testing.T) {
		in := Input{
			Providers: []ProviderInput{
				{
					Provider:   "anthropic",
					Kind:       usage.KindSubscription,
					LiveModels: []ModelEntry{{ModelID: "m2", Name: "Model Two"}},
				},
			},
			CatalogRows: []identity.Identity{{Model: "Model Two", Reasoning: "default"}},
			Degraded:    true,
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		found := false
		for _, u := range result.Unrouted {
			if u.Provider == "anthropic" && u.ModelID == "m2" {
				found = true
				if u.Reason != "no_catalog_row" {
					t.Errorf("Reason = %q, want no_catalog_row", u.Reason)
				}
			}
		}
		if !found {
			t.Errorf("Unrouted = %+v, want m2 present", result.Unrouted)
		}
		if !strings.Contains(strings.Join(result.Warnings, "\n"), degradedWarning) {
			t.Errorf("Warnings = %+v, want the degraded warning present", result.Warnings)
		}
	})

	t.Run("case 3: exactly one degraded warning for two providers", func(t *testing.T) {
		in := Input{
			Providers: []ProviderInput{
				{Provider: "anthropic", Kind: usage.KindSubscription},
				{Provider: "openai", Kind: usage.KindSubscription},
			},
			Degraded: true,
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		if n := countWarning(result.Warnings, degradedWarning); n != 1 {
			t.Errorf("degraded warning count = %d, want 1; warnings = %+v", n, result.Warnings)
		}
	})

	t.Run("case 4: degraded with zero providers emits zero warnings", func(t *testing.T) {
		result, err := ProduceRoutes(Input{Degraded: true})
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		if len(result.Warnings) != 0 {
			t.Errorf("Warnings = %+v, want empty", result.Warnings)
		}
	})

	t.Run("case 5: non-degraded with nil LiveModels emits zero degraded warnings", func(t *testing.T) {
		in := Input{
			Providers: []ProviderInput{
				{Provider: "anthropic", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{{ModelID: "m1", Name: "Model One"}}},
			},
			CatalogRows: []identity.Identity{{Model: "Model One", Reasoning: "default"}},
			Degraded:    false,
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		if n := countWarning(result.Warnings, degradedWarning); n != 0 {
			t.Errorf("degraded warning count = %d, want 0; warnings = %+v", n, result.Warnings)
		}
	})

	t.Run("case 6: degraded gateway provider keeps declared routes, no auto routes, one warning", func(t *testing.T) {
		in := Input{
			Providers: []ProviderInput{
				{
					Provider:  "openrouter",
					Kind:      usage.KindGateway,
					ModelsDev: []ModelEntry{{ModelID: "m1", Name: "Model One"}},
					UserDeclared: []UserDeclaredRoute{
						{Provider: "openrouter", ModelID: "declared-1", Model: "Declared One", Reasoning: "default"},
					},
				},
			},
			CatalogRows: []identity.Identity{{Model: "Model One", Reasoning: "default"}},
			Degraded:    true,
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		if !hasRoute(result.Routes, "openrouter", "declared-1") {
			t.Errorf("Routes = %+v, want the declared route present", result.Routes)
		}
		if hasRoute(result.Routes, "openrouter", "m1") {
			t.Errorf("Routes = %+v, want no auto route (gateway kind is ineligible for auto routing)", result.Routes)
		}
		if n := countWarning(result.Warnings, degradedWarning); n != 1 {
			t.Errorf("degraded warning count = %d, want 1; warnings = %+v", n, result.Warnings)
		}
	})

	t.Run("case 7: warnings contain the degraded string once plus the unrouted string", func(t *testing.T) {
		in := Input{
			Providers: []ProviderInput{
				{
					Provider:  "anthropic",
					Kind:      usage.KindSubscription,
					ModelsDev: []ModelEntry{{ModelID: "m-unrouted", Name: "No Match"}},
				},
			},
			Degraded: true,
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		if n := countWarning(result.Warnings, degradedWarning); n != 1 {
			t.Errorf("degraded warning count = %d, want 1; warnings = %+v", n, result.Warnings)
		}
		wantUnrouted := "unrouted provider model anthropic/m-unrouted (No Match): no catalog row matches"
		if n := countWarning(result.Warnings, wantUnrouted); n != 1 {
			t.Errorf("unrouted warning count = %d, want 1; warnings = %+v", n, result.Warnings)
		}
	})

	t.Run("case 8: ProvenanceCounts on degraded-derived routes shows only models_dev and user_declared", func(t *testing.T) {
		in := Input{
			Providers: []ProviderInput{
				{
					Provider:  "anthropic",
					Kind:      usage.KindSubscription,
					ModelsDev: []ModelEntry{{ModelID: "m1", Name: "Model One"}},
					LiveModels: []ModelEntry{{ModelID: "m2", Name: "Model Two"}},
					UserDeclared: []UserDeclaredRoute{
						{Provider: "anthropic", ModelID: "declared-1", Model: "Declared One", Reasoning: "default"},
					},
				},
			},
			CatalogRows: []identity.Identity{
				{Model: "Model One", Reasoning: "default"},
				{Model: "Model Two", Reasoning: "default"},
			},
			Degraded: true,
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		table := Table{Routes: result.Routes}
		counts := table.ProvenanceCounts()
		if _, ok := counts[ProvenanceProviderLive]; ok {
			t.Errorf("ProvenanceCounts() = %+v, want no provider_live key (live source skipped)", counts)
		}
		if counts[ProvenanceModelsDev] != 1 {
			t.Errorf("ProvenanceCounts()[models_dev] = %d, want 1", counts[ProvenanceModelsDev])
		}
		if counts[ProvenanceUserDeclared] != 1 {
			t.Errorf("ProvenanceCounts()[user_declared] = %d, want 1", counts[ProvenanceUserDeclared])
		}
	})
}
