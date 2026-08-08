package routing

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestProduceRoutes(t *testing.T) {
	row := func(model, reasoning string) identity.Identity {
		return identity.Identity{Model: model, Reasoning: reasoning}
	}
	entry := func(id, name string) ModelEntry {
		return ModelEntry{ModelID: id, Name: name}
	}
	declared := func(provider, id, model, reasoning string) UserDeclaredRoute {
		return UserDeclaredRoute{Provider: provider, ModelID: id, Model: model, Reasoning: reasoning}
	}

	tests := []struct {
		name string
		input Input
		check func(t *testing.T, result BuildResult, err error)
	}{
		{
			name: "models dev route",
			input: Input{
				Providers: []ProviderInput{{Provider: "p", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{entry("m1", "M1")}}},
				CatalogRows: []identity.Identity{row("M1", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil {
					t.Fatalf("ProduceRoutes error = %v", err)
				}
				want := []Route{{Provider: "p", ModelID: "m1", Model: "M1", Reasoning: "default", WindowIDs: []string{}, Provenance: ProvenanceModelsDev}}
				if !reflect.DeepEqual(result.Routes, want) {
					t.Fatalf("Routes = %#v, want %#v", result.Routes, want)
				}
			},
		},
		{
			name: "live upgrades models dev",
			input: Input{
				Providers: []ProviderInput{{Provider: "p", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{entry("m1", "M1")}, LiveModels: []ModelEntry{entry("m1", "M1")}}},
				CatalogRows: []identity.Identity{row("M1", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 1 {
					t.Fatalf("ProduceRoutes = routes %#v, error %v", result.Routes, err)
				}
				if got := result.Routes[0].Provenance; got != ProvenanceProviderLive {
					t.Fatalf("Provenance = %q, want %q", got, ProvenanceProviderLive)
				}
			},
		},
		{
			name: "user declaration wins",
			input: Input{
				Providers: []ProviderInput{{
					Provider: "p", Kind: usage.KindSubscription,
					ModelsDev: []ModelEntry{entry("m1", "M1")}, LiveModels: []ModelEntry{entry("m1", "M1")},
					UserDeclared: []UserDeclaredRoute{declared("p", "m1", "M1", "default")},
				}},
				CatalogRows: []identity.Identity{row("M1", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 1 {
					t.Fatalf("ProduceRoutes = routes %#v, error %v", result.Routes, err)
				}
				if got := result.Routes[0].Provenance; got != ProvenanceUserDeclared {
					t.Fatalf("Provenance = %q, want %q", got, ProvenanceUserDeclared)
				}
			},
		},
		{
			name: "live only",
			input: Input{
				Providers: []ProviderInput{{Provider: "p", Kind: usage.KindSubscription, LiveModels: []ModelEntry{entry("m2", "M2")}}},
				CatalogRows: []identity.Identity{row("M2", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 1 {
					t.Fatalf("ProduceRoutes = routes %#v, error %v", result.Routes, err)
				}
				if got := result.Routes[0].Provenance; got != ProvenanceProviderLive {
					t.Fatalf("Provenance = %q, want %q", got, ProvenanceProviderLive)
				}
			},
		},
		{
			name: "excluded auto model",
			input: Input{
				Providers: []ProviderInput{{Provider: "p", Kind: usage.KindSubscription, ExcludedModelIDs: []string{"m1"}, ModelsDev: []ModelEntry{entry("m1", "M1")}, LiveModels: []ModelEntry{entry("m1", "M1")}}},
				CatalogRows: []identity.Identity{row("M1", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 0 {
					t.Fatalf("ProduceRoutes = routes %#v, error %v", result.Routes, err)
				}
			},
		},
		{
			name: "excluded model declared",
			input: Input{
				Providers: []ProviderInput{{Provider: "p", Kind: usage.KindSubscription, ExcludedModelIDs: []string{"m1"}, ModelsDev: []ModelEntry{entry("m1", "M1")}, UserDeclared: []UserDeclaredRoute{declared("p", "m1", "M1", "default")}}},
				CatalogRows: []identity.Identity{row("M1", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 1 || result.Routes[0].Provenance != ProvenanceUserDeclared {
					t.Fatalf("ProduceRoutes = routes %#v, error %v", result.Routes, err)
				}
			},
		},
		{
			name: "gateway declaration only",
			input: Input{
				Providers: []ProviderInput{{Provider: "p", Kind: usage.KindGateway, ModelsDev: []ModelEntry{entry("m1", "M1")}, UserDeclared: []UserDeclaredRoute{declared("p", "manual", "Manual", "default")}}},
				CatalogRows: []identity.Identity{row("M1", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 1 || result.Routes[0].ModelID != "manual" {
					t.Fatalf("ProduceRoutes = routes %#v, error %v", result.Routes, err)
				}
			},
		},
		{
			name: "duplicate declared",
			input: Input{
				Providers: []ProviderInput{{Provider: "p", Kind: usage.KindSubscription, UserDeclared: []UserDeclaredRoute{declared("p", "m1", "M1", "default"), declared("p", "m1", "M1", "high")}}},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 1 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "duplicate user-declared") {
					t.Fatalf("ProduceRoutes = routes %#v, warnings %#v, error %v", result.Routes, result.Warnings, err)
				}
			},
		},
		{
			name: "provider order",
			input: Input{
				Providers: []ProviderInput{
					{Provider: "a", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{entry("m1", "M1"), entry("m2", "M2")}},
					{Provider: "b", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{entry("m3", "M3")}},
				},
				CatalogRows: []identity.Identity{row("M1", "high"), row("M2", "high"), row("M3", "high")},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil {
					t.Fatalf("ProduceRoutes error = %v", err)
				}
				want := []string{"a/m1", "a/m2", "b/m3"}
				got := make([]string, len(result.Routes))
				for i, route := range result.Routes {
					got[i] = route.Provider + "/" + route.ModelID
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("route order = %#v, want %#v", got, want)
				}
			},
		},
		{
			name:  "empty input",
			input: Input{},
			check: func(t *testing.T, result BuildResult, err error) {
				if err != nil || len(result.Routes) != 0 || len(result.Unrouted) != 0 || len(result.Warnings) != 0 || result.Errors != nil {
					t.Fatalf("ProduceRoutes = %#v, error %v", result, err)
				}
			},
		},
		{
			name: "ambiguous provider keeps declared and others",
			input: Input{
				Providers: []ProviderInput{
					{Provider: "a", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{entry("m2", "M2")}, UserDeclared: []UserDeclaredRoute{declared("a", "manual", "Manual", "default")}},
					{Provider: "b", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{entry("m3", "M3")}},
				},
				CatalogRows: []identity.Identity{
					row("M2", "low"), row("M2", "high"), row("M3", "high"),
				},
			},
			check: func(t *testing.T, result BuildResult, err error) {
				if err == nil {
					t.Fatal("ProduceRoutes error = nil, want ambiguity")
				}
				var ambiguity *AmbiguityError
				if !errors.As(err, &ambiguity) || ambiguity.Provider != "a" {
					t.Fatalf("error = %T %v, want *AmbiguityError for a", err, err)
				}
				if len(result.Errors) != 1 || result.Errors["a"] == nil {
					t.Fatalf("Errors = %#v, want provider a", result.Errors)
				}
				for _, route := range result.Routes {
					if route.Provider == "a" && route.ModelID == "m2" {
						t.Fatal("ambiguous auto route was produced")
					}
				}
				if !hasRoute(result.Routes, "a", "manual") || !hasRoute(result.Routes, "b", "m3") {
					t.Fatalf("Routes = %#v, want declared a/manual and auto b/m3", result.Routes)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ProduceRoutes(tc.input)
			tc.check(t, result, err)
		})
	}
}

func hasRoute(routes []Route, provider, modelID string) bool {
	for _, route := range routes {
		if route.Provider == provider && route.ModelID == modelID {
			return true
		}
	}
	return false
}
