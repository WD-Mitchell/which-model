package routing

import (
	"errors"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"reflect"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
)

func TestJoinModel(t *testing.T) {
	cases := []struct {
		name          string
		entry         ModelEntry
		rows          []identity.Identity
		wantLevels    []joinedLevel
		wantCands     []identity.Identity
		wantUnmatched []string
	}{
		{
			name:       "non reasoning default",
			entry:      ModelEntry{ModelID: "m1", Name: "Claude Opus 5"},
			rows:       []identity.Identity{{Model: "Claude Opus 5", Reasoning: "high"}},
			wantLevels: []joinedLevel{{level: "default", row: identity.Identity{Model: "Claude Opus 5", Reasoning: "high"}}},
			wantCands:  []identity.Identity{{Model: "Claude Opus 5", Reasoning: "high"}},
		},
		{
			name:       "annotation cleaned",
			entry:      ModelEntry{ModelID: "m1", Name: "Claude Opus 5 (2025-11-01)"},
			rows:       []identity.Identity{{Model: "Claude Opus 5", Reasoning: "high"}},
			wantLevels: []joinedLevel{{level: "default", row: identity.Identity{Model: "Claude Opus 5", Reasoning: "high"}}},
			wantCands:  []identity.Identity{{Model: "Claude Opus 5", Reasoning: "high"}},
		},
		{
			name:  "declared levels",
			entry: ModelEntry{ModelID: "m1", Name: "x", Reasoning: []string{"low", "medium", "high"}},
			rows: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "medium"},
				{Model: "x", Reasoning: "high"},
			},
			wantLevels: []joinedLevel{
				{level: "low", row: identity.Identity{Model: "x", Reasoning: "low"}},
				{level: "medium", row: identity.Identity{Model: "x", Reasoning: "medium"}},
				{level: "high", row: identity.Identity{Model: "x", Reasoning: "high"}},
			},
			wantCands: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "medium"},
				{Model: "x", Reasoning: "high"},
			},
		},
		{
			name:  "one level absent",
			entry: ModelEntry{ModelID: "m1", Name: "x", Reasoning: []string{"low", "medium", "high"}},
			rows: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "high"},
			},
			wantLevels: []joinedLevel{
				{level: "low", row: identity.Identity{Model: "x", Reasoning: "low"}},
				{level: "high", row: identity.Identity{Model: "x", Reasoning: "high"}},
			},
			wantCands: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "high"},
			},
			wantUnmatched: []string{"medium"},
		},
		{
			name:  "default matches high among candidates",
			entry: ModelEntry{ModelID: "m1", Name: "x"},
			rows: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "medium"},
				{Model: "x", Reasoning: "high"},
			},
			wantLevels: []joinedLevel{{level: "default", row: identity.Identity{Model: "x", Reasoning: "high"}}},
			wantCands: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "medium"},
				{Model: "x", Reasoning: "high"},
			},
		},
		{
			name:  "two levels covered of three",
			entry: ModelEntry{ModelID: "m1", Name: "x", Reasoning: []string{"low", "high"}},
			rows: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "medium"},
				{Model: "x", Reasoning: "high"},
			},
			wantLevels: []joinedLevel{
				{level: "low", row: identity.Identity{Model: "x", Reasoning: "low"}},
				{level: "high", row: identity.Identity{Model: "x", Reasoning: "high"}},
			},
			wantCands: []identity.Identity{
				{Model: "x", Reasoning: "low"},
				{Model: "x", Reasoning: "medium"},
				{Model: "x", Reasoning: "high"},
			},
		},
		{
			name:       "explicit default collapse",
			entry:      ModelEntry{ModelID: "m1", Name: "x", Reasoning: []string{"default"}},
			rows:       []identity.Identity{{Model: "x", Reasoning: "high"}},
			wantLevels: []joinedLevel{{level: "default", row: identity.Identity{Model: "x", Reasoning: "high"}}},
			wantCands:  []identity.Identity{{Model: "x", Reasoning: "high"}},
		},
		{
			name:          "no rows",
			entry:         ModelEntry{ModelID: "m1", Name: "x"},
			wantUnmatched: []string{"default"},
		},
		{
			name:          "declared level absent",
			entry:         ModelEntry{ModelID: "m1", Name: "x", Reasoning: []string{"low"}},
			rows:          []identity.Identity{{Model: "x", Reasoning: "medium"}},
			wantCands:     []identity.Identity{{Model: "x", Reasoning: "medium"}},
			wantUnmatched: []string{"low"},
		},
		{
			name:  "other model excluded from candidates",
			entry: ModelEntry{ModelID: "m1", Name: "x"},
			rows: []identity.Identity{
				{Model: "y", Reasoning: "low"},
				{Model: "x", Reasoning: "high"},
			},
			wantLevels: []joinedLevel{{level: "default", row: identity.Identity{Model: "x", Reasoning: "high"}}},
			wantCands:  []identity.Identity{{Model: "x", Reasoning: "high"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			levels, candidates, unmatched := joinModel(tc.entry, tc.rows)
			if !reflect.DeepEqual(levels, tc.wantLevels) {
				t.Fatalf("levels = %#v, want %#v", levels, tc.wantLevels)
			}
			if !reflect.DeepEqual(candidates, tc.wantCands) {
				t.Fatalf("candidates = %#v, want %#v", candidates, tc.wantCands)
			}
			if !reflect.DeepEqual(unmatched, tc.wantUnmatched) {
				t.Fatalf("unmatched = %#v, want %#v", unmatched, tc.wantUnmatched)
			}
		})
	}
}

func TestAmbiguityErrorError(t *testing.T) {
	err := &AmbiguityError{
		Provider: "anthropic",
		ModelID:  "claude-opus-4-5-20251101",
		Name:     "Claude Opus 5",
		Candidates: []identity.Identity{
			{Model: "Claude Opus 5", Reasoning: "low"},
			{Model: "Claude Opus 5", Reasoning: "medium"},
			{Model: "Claude Opus 5", Reasoning: "high"},
		},
	}
	const want = "ambiguous route for anthropic/claude-opus-4-5-20251101: Claude Opus 5 matches catalog identities [(Claude Opus 5, low), (Claude Opus 5, medium), (Claude Opus 5, high)] that declared effort levels cannot disambiguate; add a manual override in routes.toml"
	if got := err.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestProduceRoutesCatalogBucketsPreserveOrderAndIsolation(t *testing.T) {
	rows := []identity.Identity{{Model: "Model (annotated)", Reasoning: "low"}, {Model: "Other", Reasoning: "high"}, {Model: "Model", Reasoning: "high"}, {Model: "Solo", Reasoning: "xhigh"}}
	before := append([]identity.Identity(nil), rows...)
	in := Input{CatalogRows: rows, Providers: []ProviderInput{
		{Provider: "explicit", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{{ModelID: "model", Name: "Model [provider]", Reasoning: []string{"high", "low", "max"}}, {ModelID: "missing", Name: "Missing"}, {ModelID: "solo", Name: "Solo"}}},
		{Provider: "ambiguous", Kind: usage.KindSubscription, ModelsDev: []ModelEntry{{ModelID: "amb", Name: "Model"}}},
	}}
	result, err := ProduceRoutes(in)
	var ambiguity *AmbiguityError
	if !errors.As(err, &ambiguity) {
		t.Fatalf("ambiguity = %v", err)
	}
	if !reflect.DeepEqual(ambiguity.Candidates, []identity.Identity{rows[0], rows[2]}) {
		t.Fatalf("candidate order = %+v", ambiguity.Candidates)
	}
	wantRoutes := []Route{
		{Provider: "explicit", ModelID: "model", Model: "Model", Reasoning: "high", Provenance: ProvenanceModelsDev, WindowIDs: []string{}},
		{Provider: "explicit", ModelID: "model", Model: "Model", Reasoning: "low", Provenance: ProvenanceModelsDev, WindowIDs: []string{}},
		{Provider: "explicit", ModelID: "solo", Model: "Solo", Reasoning: "xhigh", Provenance: ProvenanceModelsDev, WindowIDs: []string{}},
	}
	if !reflect.DeepEqual(result.Routes, wantRoutes) {
		t.Fatalf("routes = %#v", result.Routes)
	}
	wantUnrouted := []UnroutedModel{{Provider: "explicit", ModelID: "model", Name: "Model", Reasoning: "max", Reason: "no_catalog_row"}, {Provider: "explicit", ModelID: "missing", Name: "Missing", Reason: "no_catalog_row"}}
	if !reflect.DeepEqual(result.Unrouted, wantUnrouted) {
		t.Fatalf("unrouted = %#v", result.Unrouted)
	}
	wantWarnings := []string{"unrouted provider model explicit/model (Model, max): no catalog row matches", "unrouted provider model explicit/missing (Missing): no catalog row matches"}
	if !reflect.DeepEqual(result.Warnings, wantWarnings) || len(result.Errors) != 1 || result.Errors["ambiguous"] != err {
		t.Fatalf("warnings/errors: %#v %#v", result.Warnings, result.Errors)
	}
	if !reflect.DeepEqual(rows, before) {
		t.Fatal("input catalog mutated")
	}
	in.CatalogRows = []identity.Identity{{Model: "Model", Reasoning: "low"}}
	result, err = ProduceRoutes(in)
	if err != nil {
		t.Fatalf("catalog index retained stale ambiguity: %v", err)
	}
	for _, route := range result.Routes {
		if route.Reasoning != "low" {
			t.Fatalf("stale catalog identity retained: %+v", route)
		}
	}
}
