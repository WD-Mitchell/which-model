package whichmodel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/routing"
)

func runRouteList(t *testing.T, args RouteListArgs) (err error, out, errOut *strings.Builder) {
	t.Helper()
	var o, e strings.Builder
	return RunRouteList(args, &o, &e), &o, &e
}

// routesListFixture is the shared two-route fixture: claude (windows) +
// codex (no windows).
func routesListFixture() routing.Table {
	return routesTable(
		routing.Route{
			Provider:   "claude",
			ModelID:    "claude-sonnet-4-5",
			Model:      "claude-sonnet-4-5",
			Reasoning:  "default",
			WindowIDs:  []string{"5h", "7d"},
			Provenance: routing.ProvenanceProviderLive,
		},
		routing.Route{
			Provider:   "codex",
			ModelID:    "gpt-5-codex",
			Model:      "gpt-5-codex",
			Reasoning:  "default",
			Provenance: routing.ProvenanceUserDeclared,
		},
	)
}

// F27-T4 row 1 (text golden): text/tabwriter output with padding 2; windows
// comma-joined or "-". Column starts 0/10/28/46/57/66 (tabwriter includes
// padding in the column width, so full-width cells are followed by 2 spaces).
func TestRunRouteListTextGolden(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesListFixture(), nil })

	err, out, _ := runRouteList(t, RouteListArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := strings.Join([]string{
		"provider  model_id           model              reasoning  windows  provenance",
		"claude    claude-sonnet-4-5  claude-sonnet-4-5  default    5h,7d    provider_live",
		"codex     gpt-5-codex        gpt-5-codex        default    -        user_declared",
		"",
	}, "\n")
	if got := trimTrailingSpaces(out.String()); got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

// F27-T4 row 2 (filter): --provider filters rows; an unknown provider is a
// UsageError naming it.
func TestRunRouteListFilter(t *testing.T) {
	requireUsageRegistry(t)
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesListFixture(), nil })

	err, out, _ := runRouteList(t, RouteListArgs{Provider: "claude", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := strings.Join([]string{
		"provider  model_id           model              reasoning  windows  provenance",
		"claude    claude-sonnet-4-5  claude-sonnet-4-5  default    5h,7d    provider_live",
		"",
	}, "\n")
	if got := trimTrailingSpaces(out.String()); got != want {
		t.Errorf("table = %q, want %q", got, want)
	}

	err, _, _ = runRouteList(t, RouteListArgs{Provider: "x", ConfigPath: cfg})
	var ue *UsageError
	if !errors.As(err, &ue) || !strings.Contains(ue.Message, `unknown provider "x"`) {
		t.Fatalf("err = %v, want *UsageError naming unknown provider x", err)
	}
	if ExitCodeFor(err) != 2 {
		t.Errorf("exit = %d, want 2", ExitCodeFor(err))
	}
}

// F27-T4 row 3 (JSON): the list document carries schema_version "2.0" and the
// canonical F18 route tags, including window_ids and provenance.
func TestRunRouteListJSON(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesListFixture(), nil })

	err, out, _ := runRouteList(t, RouteListArgs{JSON: true, ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out.String())
	}
	if doc["schema_version"] != "2.0" {
		t.Errorf("schema_version = %v, want 2.0", doc["schema_version"])
	}
	routes, ok := doc["routes"].([]any)
	if !ok || len(routes) != 2 {
		t.Fatalf("routes = %v, want 2 entries", doc["routes"])
	}
	first, ok := routes[0].(map[string]any)
	if !ok {
		t.Fatalf("route[0] = %T", routes[0])
	}
	wantTags := map[string]any{
		"provider":   "claude",
		"model_id":   "claude-sonnet-4-5",
		"model":      "claude-sonnet-4-5",
		"reasoning":  "default",
		"window_ids": []any{"5h", "7d"},
		"provenance": "provider_live",
	}
	if len(first) != len(wantTags) {
		t.Errorf("route[0] has %d keys, want %d: %v", len(first), len(wantTags), first)
	}
	for k, want := range wantTags {
		got, present := first[k]
		if !present {
			t.Errorf("route[0] missing key %q", k)
			continue
		}
		if wj, err := json.Marshal(want); err == nil {
			if gj, err := json.Marshal(got); err == nil && string(gj) != string(wj) {
				t.Errorf("route[0].%s = %v, want %v", k, got, want)
			}
		}
	}
}

// F27-T4 row 4 (missing file): the load seam returns an empty table; text is
// header-only and JSON routes is an empty array.
func TestRunRouteListMissingFile(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, nil })

	err, out, _ := runRouteList(t, RouteListArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "provider  model_id  model  reasoning  windows  provenance\n"
	if got := trimTrailingSpaces(out.String()); got != want {
		t.Errorf("table = %q, want %q", got, want)
	}

	err, out, _ = runRouteList(t, RouteListArgs{JSON: true, ConfigPath: cfg})
	if err != nil {
		t.Fatalf("json err = %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out.String())
	}
	if routes, ok := doc["routes"].([]any); !ok || len(routes) != 0 {
		t.Errorf("routes = %v, want empty array", doc["routes"])
	}
}

// F27-T4 row 5 (load error): an unreadable routes file is a runtime
// CodedError, exit 1.
func TestRunRouteListLoadError(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, errors.New("corrupt") })

	err, _, _ := runRouteList(t, RouteListArgs{ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "runtime" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: runtime}", err, err)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
}
