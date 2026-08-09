package whichmodel

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/routing"
)

func runRemove(t *testing.T, args RouteRemoveArgs) (err error, out, errOut *strings.Builder) {
	t.Helper()
	var o, e strings.Builder
	return RunRouteRemove(args, &o, &e), &o, &e
}

// F27-T3 row 1 (exact match): removing an existing route filters it out and
// saves the remaining routes; silent success.
func TestRunRouteRemoveExactMatch(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	codex := routing.Route{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared}
	claude := claudeFixture()
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(codex, claude), nil })
	var saved []routing.Table
	setSaveRoutes(t, func(path string, tbl routing.Table) error { saved = append(saved, tbl); return nil })

	err, out, _ := runRemove(t, RouteRemoveArgs{Provider: "codex", ModelID: "gpt-5-codex", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if len(saved) != 1 || len(saved[0].Routes) != 1 {
		t.Fatalf("saved tables = %d with %d routes, want 1 with 1 route", len(saved), len(saved[0].Routes))
	}
	if !reflect.DeepEqual(saved[0].Routes[0], claude) {
		t.Errorf("remaining route = %+v, want claude fixture", saved[0].Routes[0])
	}
}

// F27-T3 row 2 (provider_live): removing a provider_live route succeeds the
// same way (down to an empty route list).
func TestRunRouteRemoveProviderLive(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	live := routing.Route{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceProviderLive}
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(live), nil })
	var saved []routing.Table
	setSaveRoutes(t, func(path string, tbl routing.Table) error { saved = append(saved, tbl); return nil })

	err, _, _ := runRemove(t, RouteRemoveArgs{Provider: "codex", ModelID: "gpt-5-codex", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(saved) != 1 || len(saved[0].Routes) != 0 {
		t.Fatalf("saved tables = %d with %d routes, want 1 with 0 routes", len(saved), len(saved[0].Routes))
	}
}

// F27-T3 row 3 (missing): no exact (provider, model-id) match is a no_route
// CodedError, exit 1, and save is never called.
func TestRunRouteRemoveNoMatch(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(claudeFixture()), nil })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runRemove(t, RouteRemoveArgs{Provider: "codex", ModelID: "gpt-5-codex", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "no_route" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: no_route}", err, err)
	}
	if err.Error() != `no route "codex:gpt-5-codex"` {
		t.Errorf("message = %q, want %q", err.Error(), `no route "codex:gpt-5-codex"`)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}

// F27-T3 row 4 (partial match): a route with the same provider but a
// different model-id does not satisfy the exact match.
func TestRunRouteRemovePartialMatchIgnored(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	other := routing.Route{Provider: "codex", ModelID: "gpt-4o", Model: "gpt-4o", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared}
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(other), nil })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runRemove(t, RouteRemoveArgs{Provider: "codex", ModelID: "gpt-5-codex", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "no_route" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: no_route}", err, err)
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}

// F27-T3 row 5 (save error): a failing save is a runtime CodedError, exit 1,
// surfacing the underlying message.
func TestRunRouteRemoveSaveError(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(claudeFixture()), nil })
	setSaveRoutes(t, func(string, routing.Table) error { return errors.New("disk full") })

	err, _, _ := runRemove(t, RouteRemoveArgs{Provider: "claude", ModelID: "claude-sonnet-4-5", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "runtime" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: runtime}", err, err)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("message = %q, want mention of disk full", err.Error())
	}
}
