package whichmodel

import (
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

func runVerify(t *testing.T, args RouteVerifyArgs) (err error, out, errOut *strings.Builder) {
	t.Helper()
	var o, e strings.Builder
	return RunRouteVerify(args, &o, &e), &o, &e
}

// routesVerifyFixture: one user_declared route (claude) and one
// provider_live route (codex), all resolving in the fake scores.
func routesVerifyFixture() routing.Table {
	return routesTable(
		routing.Route{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
		routing.Route{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceProviderLive},
	)
}

func verifyScoresFixture() []ScoreRow {
	return []ScoreRow{
		{Model: "claude-sonnet-4-5", Reasoning: "default"},
		{Model: "gpt-5-codex", Reasoning: "default"},
	}
}

// F27-T6 row 1 (clean): all routes resolve, hashes agree — nil error, empty
// stdout, and the stderr summary line.
func TestRunRouteVerifyClean(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesVerifyFixture(), nil })
	setReadScores(t, func(string) ([]ScoreRow, error) { return verifyScoresFixture(), nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })

	err, out, errOut := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "routes: 2 total (1 user_declared, 1 provider_live, 0 models_dev)") {
		t.Errorf("stderr = %q, want the summary line", errOut.String())
	}
}

// F27-T6 row 2 (stale): a route whose (model, reasoning) pair is absent from
// the scores is reported on stdout and fails with stale_routes, exit 1.
func TestRunRouteVerifyStale(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(
			routing.Route{Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
			routing.Route{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceProviderLive},
		), nil
	})
	setReadScores(t, func(string) ([]ScoreRow, error) {
		return []ScoreRow{{Model: "claude-sonnet-4-5", Reasoning: "default"}}, nil
	})
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })

	err, out, _ := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	if out.String() != "stale route codex:gpt-5-codex (gpt-5-codex/default)\n" {
		t.Errorf("stdout = %q, want the stale route line", out.String())
	}
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "stale_routes" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: stale_routes}", err, err)
	}
	if ce.Message != "1 stale route(s); run which-model routes refresh" {
		t.Errorf("message = %q", ce.Message)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
}

// F27-T6 row 3 (unrouted): a score row with no covering route is warned about
// but does not fail the verify.
func TestRunRouteVerifyUnrouted(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(
			routing.Route{Provider: "claude", ModelID: "claude-sonnet-4-5", Model: "claude-sonnet-4-5", Reasoning: "default", Provenance: routing.ProvenanceUserDeclared},
		), nil
	})
	setReadScores(t, func(string) ([]ScoreRow, error) { return verifyScoresFixture(), nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })

	err, out, errOut := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	want := "warning: score row gpt-5-codex/default has no route; it cannot be picked\n"
	if !strings.Contains(errOut.String(), want) {
		t.Errorf("stderr = %q, want to contain %q", errOut.String(), want)
	}
}

// F27-T6 row 4 (hash mismatch): the live and stored hashes disagree — the
// changed-CSV warning fires; when either hash is empty there is no warning.
func TestRunRouteVerifyHashMismatch(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) {
		tbl := routesVerifyFixture()
		tbl.ScoresHash = "abc"
		return tbl, nil
	})
	setReadScores(t, func(string) ([]ScoreRow, error) { return verifyScoresFixture(), nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "def", nil })

	err, _, errOut := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "warning: scores CSV changed since routes were produced; run which-model routes refresh\n"
	if !strings.Contains(errOut.String(), want) {
		t.Errorf("stderr = %q, want to contain %q", errOut.String(), want)
	}
}

func TestRunRouteVerifyHashAbsentNoWarning(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) {
		tbl := routesVerifyFixture()
		tbl.ScoresHash = ""
		return tbl, nil
	})
	setReadScores(t, func(string) ([]ScoreRow, error) { return verifyScoresFixture(), nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "abc", nil })

	err, _, errOut := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(errOut.String(), "scores CSV changed") {
		t.Errorf("stderr = %q, want no changed-CSV warning when the stored hash is empty", errOut.String())
	}
}

// F27-T6 row 5 (missing routes file): the load seam returns an empty table —
// exit 0 with the zero summary.
func TestRunRouteVerifyMissingRoutesFile(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, nil })
	setReadScores(t, func(string) ([]ScoreRow, error) { return nil, nil })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })

	err, out, errOut := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "routes: 0 total (0 user_declared, 0 provider_live, 0 models_dev)") {
		t.Errorf("stderr = %q, want the zero summary line", errOut.String())
	}
}

// F27-T6 row 6 (IO error): a failing scores read is a runtime CodedError,
// exit 1; so is a failing routes load.
func TestRunRouteVerifyScoresReadError(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesVerifyFixture(), nil })
	setReadScores(t, func(string) ([]ScoreRow, error) { return nil, errors.New("corrupt") })
	setScoresSHA256(t, func(*config.Config) (string, error) { return "", nil })

	err, _, _ := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
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
}

func TestRunRouteVerifyLoadError(t *testing.T) {
	cfg := routesTestConfig(t, t.TempDir())
	setLoadRoutes(t, func(string) (routing.Table, error) { return routing.Table{}, errors.New("corrupt") })

	err, _, _ := runVerify(t, RouteVerifyArgs{ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "runtime" {
		t.Fatalf("err = %v (%T), want *CodedError{Code: runtime}", err, err)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
}
