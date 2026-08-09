package whichmodel

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

func runRouteRefresh(t *testing.T, args RouteRefreshArgs) (err error, out, errOut *strings.Builder) {
	t.Helper()
	var o, e strings.Builder
	return RunRouteRefresh(args, &o, &e), &o, &e
}

// F27-T5 row 1 (produce + persist): refresh saves exactly what the producer
// returned — no reordering, no filtering — and stays silent on stdout.
func TestRunRouteRefreshProducesAndPersists(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })
	produced := []routing.Route{
		{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
		{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceProviderLive},
	}
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) { return produced, nil })
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, nil })
	var saved []routing.Table
	setSaveRoutes(t, func(path string, tbl routing.Table) error { saved = append(saved, tbl); return nil })

	err, out, _ := runRouteRefresh(t, RouteRefreshArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if len(saved) != 1 {
		t.Fatalf("save calls = %d, want 1", len(saved))
	}
	if !reflect.DeepEqual(saved[0].Routes, produced) {
		t.Errorf("saved routes = %+v, want exactly the produced routes", saved[0].Routes)
	}
}

// F27-T5 fidelity pin: a preserved user route inside the producer output
// stays untouched, in position.
func TestRunRouteRefreshUserDeclaredFidelity(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })
	produced := []routing.Route{
		{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceProviderLive},
		{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
		{Provider: "copilot", ModelID: "gpt-4o", Model: "gpt-4o", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
	}
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) { return produced, nil })
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, nil })
	var saved []routing.Table
	setSaveRoutes(t, func(path string, tbl routing.Table) error { saved = append(saved, tbl); return nil })

	err, _, _ := runRouteRefresh(t, RouteRefreshArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(saved[0].Routes, produced) {
		t.Errorf("saved routes = %+v, want the produced routes in order (user route preserved)", saved[0].Routes)
	}
}

// F27-T5 row 2 (idempotent): when the loaded table already matches what
// refresh would save (schema version, hash, routes), save is skipped.
func TestRunRouteRefreshIdempotent(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })
	produced := []routing.Route{
		{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceProviderLive},
		{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
	}
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) { return produced, nil })
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(produced...), nil })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runRouteRefresh(t, RouteRefreshArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0 (compare-and-skip)", saves)
	}
}

// F27-T5 row 3 (usage disabled): the toggle returning (false, reason) emits
// exactly one warning line and still saves.
func TestRunRouteRefreshUsageDisabledWarning(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return false, "flag" })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })
	produced := []routing.Route{{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared}}
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) { return produced, nil })
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, nil })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, errOut := runRouteRefresh(t, RouteRefreshArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "warning: usage is disabled; refresh uses static sources only\n"
	if errOut.String() != want {
		t.Errorf("stderr = %q, want exactly %q", errOut.String(), want)
	}
	if saves != 1 {
		t.Errorf("save calls = %d, want 1", saves)
	}
}

// F27-T5 row 4 (--auto): a case-insensitive substring match appends a
// user_declared route built from the matched score row after the produced
// routes.
func TestRunRouteRefreshAutoAddsRoute(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })
	produced := []routing.Route{{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceProviderLive}}
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) { return produced, nil })
	setReadScores(t, func(string) ([]ScoreRow, error) {
		return []ScoreRow{
			{Model: "claude-sonnet-4-5", Reasoning: "default", Provider: "claude"},
			{Model: "gpt-5-codex", Reasoning: "default", Provider: "codex"},
		}, nil
	})
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, nil })
	var saved []routing.Table
	setSaveRoutes(t, func(path string, tbl routing.Table) error { saved = append(saved, tbl); return nil })

	err, _, _ := runRouteRefresh(t, RouteRefreshArgs{Auto: "claude-sonnet", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(saved) != 1 || len(saved[0].Routes) != 2 {
		t.Fatalf("saved tables = %d with %d routes, want 1 with 2 routes", len(saved), len(saved[0].Routes))
	}
	if !reflect.DeepEqual(saved[0].Routes[0], produced[0]) {
		t.Errorf("routes[0] = %+v, want produced route first", saved[0].Routes[0])
	}
	added := saved[0].Routes[1]
	if added.Provider != "claude" || added.ModelID != "claude-sonnet-4-5" || added.Model != "claude-sonnet-4-5" {
		t.Errorf("auto route identity = %+v", added)
	}
	if added.Reasoning != "default" {
		t.Errorf("auto reasoning = %q, want default", added.Reasoning)
	}
	if added.Provenance != routing.ProvenanceUserDeclared || string(added.Provenance) != "user_declared" {
		t.Errorf("auto provenance = %q, want user_declared", added.Provenance)
	}
	if len(added.WindowIDs) != 0 {
		t.Errorf("auto window_ids = %v, want empty", added.WindowIDs)
	}
}

// F27-T5 row 5 (--auto errors): zero matches and ambiguous matches are
// UsageErrors naming the rows, and save is never called.
func TestRunRouteRefreshAutoNoMatch(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) {
		return []routing.Route{{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared}}, nil
	})
	setReadScores(t, func(string) ([]ScoreRow, error) {
		return []ScoreRow{{Model: "claude-sonnet-4-5", Reasoning: "default", Provider: "claude"}}, nil
	})
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runRouteRefresh(t, RouteRefreshArgs{Auto: "zzz", ConfigPath: cfg})
	var ue *UsageError
	if !errors.As(err, &ue) || ue.Message != `no score row matching "zzz"` {
		t.Fatalf("err = %v, want *UsageError with no-match message", err)
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}

func TestRunRouteRefreshAutoAmbiguous(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) {
		return []routing.Route{{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared}}, nil
	})
	setReadScores(t, func(string) ([]ScoreRow, error) {
		return []ScoreRow{
			{Model: "claude-sonnet-4-5", Reasoning: "default", Provider: "claude"},
			{Model: "claude-sonnet-4-5-mini", Reasoning: "default", Provider: "claude"},
		}, nil
	})
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runRouteRefresh(t, RouteRefreshArgs{Auto: "claude-sonnet", ConfigPath: cfg})
	var ue *UsageError
	if !errors.As(err, &ue) || ue.Message != `no score row matching "claude-sonnet" (ambiguous: claude-sonnet-4-5, claude-sonnet-4-5-mini)` {
		t.Fatalf("err = %v, want *UsageError with ambiguous message", err)
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}

// F27-T5 row 6 (produce error): a failing producer is a runtime CodedError,
// exit 1, and save is never called.
func TestRunRouteRefreshProduceError(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })
	setProduceRoutes(t, func(*config.Config) ([]routing.Route, error) { return nil, errors.New("boom") })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runRouteRefresh(t, RouteRefreshArgs{ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "runtime" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: runtime}", err, err)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("message = %q, want mention of boom", err.Error())
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}
