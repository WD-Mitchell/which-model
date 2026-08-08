package routing

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// ambiguityFixture builds the canonical ambiguous-provider fixture (F18-T4
// Instructions §1): "anthropic", KindSubscription, one models.dev entry with
// three same-name catalog levels that its (absent) declared reasoning list
// cannot disambiguate.
func ambiguityFixture() Input {
	return Input{
		Providers: []ProviderInput{
			{
				Provider: "anthropic",
				Kind:     usage.KindSubscription,
				ModelsDev: []ModelEntry{
					{ModelID: "claude-opus-4-5-20251101", Name: "Claude Opus 5", Reasoning: nil},
				},
			},
		},
		CatalogRows: []identity.Identity{
			{Model: "Claude Opus 5", Reasoning: "low"},
			{Model: "Claude Opus 5", Reasoning: "medium"},
			{Model: "Claude Opus 5", Reasoning: "high"},
		},
	}
}

func TestProduceRoutesAmbiguity(t *testing.T) {
	golden, err := os.ReadFile("testdata/ambiguity.golden")
	if err != nil {
		t.Fatalf("ReadFile(ambiguity.golden) error = %v", err)
	}
	want := strings.TrimSpace(string(golden))

	t.Run("case 1-3: AmbiguityError fields", func(t *testing.T) {
		result, err := ProduceRoutes(ambiguityFixture())
		var ambErr *AmbiguityError
		if !errors.As(err, &ambErr) {
			t.Fatalf("ProduceRoutes() error = %v (%T), want *AmbiguityError", err, err)
		}
		if got := ambErr.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
		if ambErr.Provider != "anthropic" {
			t.Errorf("Provider = %q, want anthropic", ambErr.Provider)
		}
		if ambErr.ModelID != "claude-opus-4-5-20251101" {
			t.Errorf("ModelID = %q, want claude-opus-4-5-20251101", ambErr.ModelID)
		}
		if ambErr.Name != "Claude Opus 5" {
			t.Errorf("Name = %q, want Claude Opus 5", ambErr.Name)
		}
		wantCandidates := []identity.Identity{
			{Model: "Claude Opus 5", Reasoning: "low"},
			{Model: "Claude Opus 5", Reasoning: "medium"},
			{Model: "Claude Opus 5", Reasoning: "high"},
		}
		if !reflect.DeepEqual(ambErr.Candidates, wantCandidates) {
			t.Errorf("Candidates = %+v, want %+v", ambErr.Candidates, wantCandidates)
		}
		if _ = result; false {
			// result unused beyond the error assertions above.
		}
	})

	t.Run("case 4: declared route for a different model does not suppress the underlying ambiguity error, but Routes carries only the declared route (no auto routes for the still-ambiguous provider)", func(t *testing.T) {
		in := ambiguityFixture()
		in.Providers[0].UserDeclared = []UserDeclaredRoute{
			{Provider: "anthropic", ModelID: "claude-haiku-4-5", Model: "Claude Haiku 5", Reasoning: "default"},
		}
		result, err := ProduceRoutes(in)
		var ambErr *AmbiguityError
		if !errors.As(err, &ambErr) {
			t.Fatalf("ProduceRoutes() error = %v, want *AmbiguityError (the opus model is still ambiguous)", err)
		}
		if len(result.Routes) != 1 {
			t.Fatalf("Routes = %+v, want exactly the declared route", result.Routes)
		}
		if result.Routes[0].ModelID != "claude-haiku-4-5" || result.Routes[0].Provenance != ProvenanceUserDeclared {
			t.Errorf("Routes[0] = %+v, want the declared claude-haiku-4-5 route", result.Routes[0])
		}
		if hasRoute(result.Routes, "anthropic", "claude-opus-4-5-20251101") {
			t.Error("Routes contains an auto route for the ambiguous model; provider errored, no auto routes expected")
		}
	})

	t.Run("case 5-6: two ambiguous providers", func(t *testing.T) {
		in := ambiguityFixture()
		in.Providers = append(in.Providers, ProviderInput{
			Provider: "openai",
			Kind:     usage.KindSubscription,
			ModelsDev: []ModelEntry{
				{ModelID: "gpt-6", Name: "GPT-6", Reasoning: nil},
			},
		})
		in.CatalogRows = append(in.CatalogRows,
			identity.Identity{Model: "GPT-6", Reasoning: "low"},
			identity.Identity{Model: "GPT-6", Reasoning: "high"},
		)
		result, err := ProduceRoutes(in)
		var ambErr *AmbiguityError
		if !errors.As(err, &ambErr) {
			t.Fatalf("ProduceRoutes() error = %v, want *AmbiguityError", err)
		}
		if ambErr.Provider != "anthropic" {
			t.Errorf("returned error provider = %q, want anthropic (first provider)", ambErr.Provider)
		}
		if len(result.Errors) != 2 {
			t.Fatalf("len(Errors) = %d, want 2", len(result.Errors))
		}
	})

	t.Run("case 6b: single ambiguous provider keyed error map", func(t *testing.T) {
		result, err := ProduceRoutes(ambiguityFixture())
		if err == nil {
			t.Fatal("ProduceRoutes() error = nil, want non-nil")
		}
		if len(result.Errors) != 1 {
			t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
		}
		if _, ok := result.Errors["anthropic"]; !ok {
			t.Errorf("Errors = %+v, want key %q", result.Errors, "anthropic")
		}
	})

	t.Run("case 7: declared override for the ambiguous model id resolves it", func(t *testing.T) {
		in := ambiguityFixture()
		in.Providers[0].UserDeclared = []UserDeclaredRoute{
			{Provider: "anthropic", ModelID: "claude-opus-4-5-20251101", Model: "Claude Opus 5", Reasoning: "high"},
		}
		result, err := ProduceRoutes(in)
		if err != nil {
			t.Fatalf("ProduceRoutes() error = %v, want nil", err)
		}
		found := false
		for _, route := range result.Routes {
			if route.Provider == "anthropic" && route.ModelID == "claude-opus-4-5-20251101" {
				found = true
				if route.Provenance != ProvenanceUserDeclared {
					t.Errorf("Provenance = %v, want %v", route.Provenance, ProvenanceUserDeclared)
				}
			}
		}
		if !found {
			t.Errorf("Routes = %+v, want the declared override route present", result.Routes)
		}
	})

	t.Run("case 8: ambiguity is not absence", func(t *testing.T) {
		result, _ := ProduceRoutes(ambiguityFixture())
		for _, u := range result.Unrouted {
			if u.Provider == "anthropic" && u.ModelID == "claude-opus-4-5-20251101" {
				t.Errorf("Unrouted contains the ambiguous model %+v; ambiguity must not be reported as absence", u)
			}
		}
	})
}
