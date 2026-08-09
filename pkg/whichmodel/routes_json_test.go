package whichmodel

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

func runRouteVerify(t *testing.T, args RouteVerifyArgs) (err error, out, errOut *strings.Builder) {
	t.Helper()
	var o, e strings.Builder
	return RunRouteVerify(args, &o, &e), &o, &e
}

// routesJSONFixture: one resolving user_declared route (claude), one stale
// provider_live route (codex — its model is not in the scores), and an
// unrouted score row; stored hash differs from the live hash.
func routesJSONFixture() (routing.Table, []ScoreRow) {
	tbl := routesTable(
		routing.Route{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
		routing.Route{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex-legacy", Reasoning: "default", Provenance: routing.ProvenanceProviderLive},
	)
	tbl.ScoresHash = "abc"
	return tbl, []ScoreRow{
		{Model: "claude-sonnet-4-5", Reasoning: "default"},
		{Model: "gpt-5-codex", Reasoning: "default"},
	}
}

func verifyReport(t *testing.T, out string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	return doc
}

// F27-T7 row 1 (verify --json golden): the report carries schema_version
// "2.0", stale_routes, unrouted, provenance_counts with all three keys, and
// scores_sha256_matches.
func TestRunRouteVerifyJSONReport(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	tbl, rows := routesJSONFixture()
	setLoadRoutes(t, func(string) (routing.Table, error) { return tbl, nil })
	setReadScores(t, func(string) ([]ScoreRow, error) { return rows, nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "def", nil })

	err, out, _ := runRouteVerify(t, RouteVerifyArgs{JSON: true, ConfigPath: cfg})
	doc := verifyReport(t, out.String())

	if doc["schema_version"] != "2.0" {
		t.Errorf("schema_version = %v, want 2.0", doc["schema_version"])
	}
	stale, ok := doc["stale_routes"].([]any)
	if !ok || len(stale) != 1 || stale[0] != "codex:gpt-5-codex" {
		t.Errorf("stale_routes = %v, want [codex:gpt-5-codex]", doc["stale_routes"])
	}
	unrouted, ok := doc["unrouted"].([]any)
	if !ok || len(unrouted) != 1 {
		t.Fatalf("unrouted = %v, want one entry", doc["unrouted"])
	}
	wantUnrouted := map[string]any{"model": "gpt-5-codex", "reasoning": "default"}
	if u, ok := unrouted[0].(map[string]any); !ok || len(u) != len(wantUnrouted) {
		t.Errorf("unrouted[0] = %v, want %v", unrouted[0], wantUnrouted)
	} else {
		for k, want := range wantUnrouted {
			if u[k] != want {
				t.Errorf("unrouted[0].%s = %v, want %v", k, u[k], want)
			}
		}
	}
	counts, ok := doc["provenance_counts"].(map[string]any)
	if !ok || len(counts) != 3 {
		t.Fatalf("provenance_counts = %v, want all three keys", doc["provenance_counts"])
	}
	if counts["user_declared"] != float64(1) || counts["provider_live"] != float64(1) || counts["models_dev"] != float64(0) {
		t.Errorf("provenance_counts = %v, want {user_declared:1 provider_live:1 models_dev:0}", counts)
	}
	if m, ok := doc["scores_sha256_matches"].(bool); !ok || m {
		t.Errorf("scores_sha256_matches = %v, want false", doc["scores_sha256_matches"])
	}
	// The stale report still exits 1 via the stale_routes code.
	if ExitCodeFor(err) != 1 || CodeFor(err) != "stale_routes" {
		t.Errorf("err = %v (exit %d, code %q), want exit 1 code stale_routes", err, ExitCodeFor(err), CodeFor(err))
	}
}

// F27-T7 row 2 (JSON mode error shape): a stale JSON verify returns
// *ReportedError wrapping *CodedError{Code: stale_routes}, and stdout still
// carries the full report.
func TestRunRouteVerifyJSONStaleReportedError(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	tbl, rows := routesJSONFixture()
	setLoadRoutes(t, func(string) (routing.Table, error) { return tbl, nil })
	setReadScores(t, func(string) ([]ScoreRow, error) { return rows, nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "def", nil })

	err, out, _ := runRouteVerify(t, RouteVerifyArgs{JSON: true, ConfigPath: cfg})
	re, ok := err.(*ReportedError)
	if !ok {
		t.Fatalf("err = %T, want *ReportedError", err)
	}
	inner, ok := re.Unwrap().(*CodedError)
	if !ok || inner.Code != "stale_routes" {
		t.Fatalf("unwrapped = %v (%T), want *CodedError{Code: stale_routes}", re.Unwrap(), re.Unwrap())
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
	doc := verifyReport(t, out.String())
	if stale, ok := doc["stale_routes"].([]any); !ok || len(stale) != 1 {
		t.Errorf("stdout report missing the stale route: %v", doc["stale_routes"])
	}
}

// F27-T7 row 3 (clean JSON): no stale routes — nil error, empty stale_routes
// array, exit 0.
func TestRunRouteVerifyJSONClean(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	tbl := routesVerifyFixture()
	setLoadRoutes(t, func(string) (routing.Table, error) { return tbl, nil })
	setReadScores(t, func(string) ([]ScoreRow, error) { return verifyScoresFixture(), nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })

	err, out, _ := runRouteVerify(t, RouteVerifyArgs{JSON: true, ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ExitCodeFor(err) != 0 {
		t.Errorf("exit = %d, want 0", ExitCodeFor(err))
	}
	doc := verifyReport(t, out.String())
	if stale, ok := doc["stale_routes"].([]any); !ok || len(stale) != 0 {
		t.Errorf("stale_routes = %v, want empty array", doc["stale_routes"])
	}
	if unrouted, ok := doc["unrouted"].([]any); !ok || len(unrouted) != 0 {
		t.Errorf("unrouted = %v, want empty array", doc["unrouted"])
	}
}

// F27-T7 row 4 (remove error shape): a missing route maps to code no_route,
// exit 1.
func TestRunRouteRemoveJSONErrorShape(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesVerifyFixture(), nil })
	saves := 0
	setSaveRoutes(t, func(string, routing.Table) error { saves++; return nil })

	err, _, _ := runRemove(t, RouteRemoveArgs{Provider: "codex", ModelID: "missing", ConfigPath: cfg})
	if CodeFor(err) != "no_route" {
		t.Errorf("code = %q, want no_route", CodeFor(err))
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
	if saves != 0 {
		t.Errorf("save calls = %d, want 0", saves)
	}
}

// F27-T7 row 5 (text mode stale): the text-mode stale exit is also wrapped in
// *ReportedError, with the stale line on stdout.
func TestRunRouteVerifyTextStaleReportedError(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	tbl, rows := routesJSONFixture()
	setLoadRoutes(t, func(string) (routing.Table, error) { return tbl, nil })
	setReadScores(t, func(string) ([]ScoreRow, error) { return rows, nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "def", nil })

	err, out, _ := runRouteVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	if out.String() != "stale route codex:gpt-5-codex (gpt-5-codex-legacy/default)\n" {
		t.Errorf("stdout = %q, want the stale route line", out.String())
	}
	re, ok := err.(*ReportedError)
	if !ok {
		t.Fatalf("err = %T, want *ReportedError", err)
	}
	inner, ok := re.Unwrap().(*CodedError)
	if !ok || inner.Code != "stale_routes" {
		t.Fatalf("unwrapped = %v (%T), want *CodedError{Code: stale_routes}", re.Unwrap(), re.Unwrap())
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
}

// F27-T7 acceptance: errors.As must find the stale_routes CodedError through
// the ReportedError wrap in both modes.
func TestRunRouteVerifyReportedErrorUnwrap(t *testing.T) {
	for _, j := range []bool{false, true} {
		cfg := routesTestConfig(t, t.TempDir())
		tbl, rows := routesJSONFixture()
		setLoadRoutes(t, func(string) (routing.Table, error) { return tbl, nil })
		setReadScores(t, func(string) ([]ScoreRow, error) { return rows, nil })
		setScoresSHA256(t, func(*config.Config) (string, error) { return "def", nil })

		err, _, _ := runRouteVerify(t, RouteVerifyArgs{JSON: j, ConfigPath: cfg})
		var ce *CodedError
		if !errors.As(err, &ce) || ce.Code != "stale_routes" {
			t.Errorf("json=%v: err = %v, errors.As did not find stale_routes CodedError", j, err)
		}
	}
}
