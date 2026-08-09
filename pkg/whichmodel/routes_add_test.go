package whichmodel

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/routing"
)

// claudeFixture is the shared existing-route fixture (provider_live).
func claudeFixture() routing.Route {
	return routing.Route{
		Provider:   "claude",
		ModelID:    "claude-sonnet-4-5",
		Model:      "claude-sonnet-4-5",
		Reasoning:  "default",
		Provenance: routing.ProvenanceProviderLive,
	}
}

func runAdd(t *testing.T, args RouteAddArgs) (err error, out, errOut *strings.Builder) {
	t.Helper()
	var o, e strings.Builder
	return RunRouteAdd(args, &o, &e), &o, &e
}

// F27-T2 row 1 (validation): unknown provider and empty --model/--model-id
// both fail as *UsageError with the exact messages.
func TestRunRouteAddValidation(t *testing.T) {
	requireUsageRegistry(t)
	cfg := routesTestConfig(t, t.TempDir())

	err, _, _ := runAdd(t, RouteAddArgs{Provider: "not-a-provider", ModelID: "x", Model: "y", ConfigPath: cfg})
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("unknown provider: err = %v (%T), want *UsageError", err, err)
	}
	if !strings.Contains(ue.Message, `unknown provider "not-a-provider"`) || !strings.Contains(ue.Message, "valid providers:") {
		t.Errorf("unknown provider: message = %q", ue.Message)
	}
	if ExitCodeFor(err) != 2 {
		t.Errorf("unknown provider: exit = %d, want 2", ExitCodeFor(err))
	}

	err, _, _ = runAdd(t, RouteAddArgs{Provider: "claude", ModelID: "", Model: "", ConfigPath: cfg})
	if !errors.As(err, &ue) || ue.Message != "--model-id and --model are required" {
		t.Fatalf("empty model fields: err = %v, want *UsageError with %q", err, "--model-id and --model are required")
	}
}

// F27-T2 row 2 (write): a valid add appends a user_declared route, saves the
// loaded table with 2 routes, and stays silent on stdout.
func TestRunRouteAddWrites(t *testing.T) {
	requireUsageRegistry(t)
	cfg := routesTestConfig(t, t.TempDir())
	existing := claudeFixture()
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(existing), nil })
	var saved []routing.Table
	setSaveRoutes(t, func(path string, tbl routing.Table) error { saved = append(saved, tbl); return nil })

	err, out, _ := runAdd(t, RouteAddArgs{
		Provider:   "codex",
		ModelID:    "gpt-5-codex",
		Model:      "gpt-5-codex",
		Reasoning:  "default",
		Windows:    []string{},
		ConfigPath: cfg,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if len(saved) != 1 {
		t.Fatalf("save calls = %d, want 1", len(saved))
	}
	got := saved[0]
	if len(got.Routes) != 2 {
		t.Fatalf("saved routes = %d, want 2", len(got.Routes))
	}
	if !reflect.DeepEqual(got.Routes[0], existing) {
		t.Errorf("route[0] = %+v, want existing %+v (order must be preserved)", got.Routes[0], existing)
	}
	added := got.Routes[1]
	if added.Provider != "codex" || added.ModelID != "gpt-5-codex" || added.Model != "gpt-5-codex" {
		t.Errorf("added route identity = %+v", added)
	}
	if added.Reasoning != "default" {
		t.Errorf("added reasoning = %q, want default", added.Reasoning)
	}
	if added.Provenance != routing.ProvenanceUserDeclared || string(added.Provenance) != "user_declared" {
		t.Errorf("added provenance = %q, want user_declared", added.Provenance)
	}
	if len(added.WindowIDs) != 0 {
		t.Errorf("added window_ids = %v, want empty", added.WindowIDs)
	}
}

// F27-T2 row 3 (passthrough): --reasoning and --window values land on the
// saved route.
func TestRunRouteAddReasoningWindowsPassthrough(t *testing.T) {
	requireUsageRegistry(t)
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(), nil })
	var saved []routing.Table
	setSaveRoutes(t, func(path string, tbl routing.Table) error { saved = append(saved, tbl); return nil })

	err, _, _ := runAdd(t, RouteAddArgs{
		Provider:   "codex",
		ModelID:    "gpt-5-codex",
		Model:      "gpt-5-codex",
		Reasoning:  "fast",
		Windows:    []string{"5h", "7d"},
		ConfigPath: cfg,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	added := saved[0].Routes[0]
	if added.Reasoning != "fast" {
		t.Errorf("reasoning = %q, want fast", added.Reasoning)
	}
	if !reflect.DeepEqual(added.WindowIDs, []string{"5h", "7d"}) {
		t.Errorf("window_ids = %v, want [5h 7d]", added.WindowIDs)
	}
}

// F27-T2 row 4 (duplicate): an existing (provider, model-id) pair is a
// UsageError and never reaches save.
func TestRunRouteAddDuplicate(t *testing.T) {
	requireUsageRegistry(t)
	cfg := routesTestConfig(t, t.TempDir())
	dup := routing.Route{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared}
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(dup), nil })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runAdd(t, RouteAddArgs{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", ConfigPath: cfg})
	var ue *UsageError
	if !errors.As(err, &ue) || ue.Message != `route "codex:gpt-5-codex" already exists; remove it first` {
		t.Fatalf("err = %v, want *UsageError with already-exists message", err)
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}

// F27-T2 row 5 (load error): an unreadable routes file is a runtime
// CodedError, exit 1, surfacing the underlying message.
func TestRunRouteAddLoadError(t *testing.T) {
	requireUsageRegistry(t)
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, errors.New("corrupt") })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runAdd(t, RouteAddArgs{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "runtime" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: runtime}", err, err)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("message = %q, want mention of corrupt", err.Error())
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}
