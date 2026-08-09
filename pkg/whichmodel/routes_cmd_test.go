package whichmodel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

// requireUsageRegistry skips tests that depend on the live provider registry
// (usage.Get). The registry is empty by construction under -tags nousage, so
// provider-validation cases cannot run there; the routes command itself is
// registered in all builds (F27 SPEC §2.6).
func requireUsageRegistry(t *testing.T) {
	t.Helper()
	if !toggle.Compiled {
		t.Skip("provider registry unavailable in nousage build")
	}
}

// routesTestConfig writes an empty config.toml and returns its path.
func routesTestConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}
	return path
}

// routesTable builds the F18 table envelope for a set of routes.
func routesTable(routes ...routing.Route) routing.Table {
	return routing.Table{SchemaVersion: routing.TableSchemaVersion, Routes: routes}
}

// trimTrailingSpaces removes per-line trailing whitespace (tabwriter pads the
// final cell; the golden is asserted without trailing padding).
func trimTrailingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// Route-seam setters: swap a package seam for the duration of one test and
// restore it on cleanup. Names and shapes per F27 TASKS.md T2/T5/T6.
func setLoadRoutes(t *testing.T, fn func(path string) (routing.Table, error)) {
	t.Helper()
	old := loadRoutesFunc
	loadRoutesFunc = fn
	t.Cleanup(func() { loadRoutesFunc = old })
}

func setSaveRoutes(t *testing.T, fn func(path string, tbl routing.Table) error) {
	t.Helper()
	old := saveRoutesFunc
	saveRoutesFunc = fn
	t.Cleanup(func() { saveRoutesFunc = old })
}

func setProduceRoutes(t *testing.T, fn func(cfg *config.Config) ([]routing.Route, error)) {
	t.Helper()
	old := produceRoutesFunc
	produceRoutesFunc = fn
	t.Cleanup(func() { produceRoutesFunc = old })
}

func setRoutesPath(t *testing.T, fn func(cfg *config.Config) (string, error)) {
	t.Helper()
	old := routesPathFunc
	routesPathFunc = fn
	t.Cleanup(func() { routesPathFunc = old })
}

func setReadScores(t *testing.T, fn func(path string) ([]ScoreRow, error)) {
	t.Helper()
	old := readScoresFunc
	readScoresFunc = fn
	t.Cleanup(func() { readScoresFunc = old })
}

func setScoresSHA256(t *testing.T, fn func(cfg *config.Config) (string, error)) {
	t.Helper()
	old := scoresSHA256Func
	scoresSHA256Func = fn
	t.Cleanup(func() { scoresSHA256Func = old })
}

func setToggleResolve(t *testing.T, fn func(flagNoUsage bool, cfg *config.Config) (bool, string)) {
	t.Helper()
	old := toggleResolveFunc
	toggleResolveFunc = fn
	t.Cleanup(func() { toggleResolveFunc = old })
}

func routesCheckFlag(t *testing.T, cmd *cobra.Command, name, wantType, wantDefault string) {
	t.Helper()
	f := cmd.Flags().Lookup(name)
	if f == nil {
		t.Fatalf("flag --%s missing on %q", name, cmd.Name())
	}
	if f.Value.Type() != wantType {
		t.Errorf("--%s type = %q, want %q", name, f.Value.Type(), wantType)
	}
	if f.DefValue != wantDefault {
		t.Errorf("--%s default = %q, want %q", name, f.DefValue, wantDefault)
	}
}

// F27-T1 row 1: registeredCommands() contains routes.
func TestRoutesCommandRegistered(t *testing.T) {
	for _, cmd := range registeredCommands() {
		if cmd.Name() == "routes" {
			return
		}
	}
	t.Fatal("registeredCommands() does not contain routes")
}

// F27-T1 row 2: subcommand names exactly [list add remove refresh verify] in
// that order.
func TestRoutesSubcommandOrder(t *testing.T) {
	cmd := NewRoutesCmd()
	if cmd.Name() != "routes" || cmd.Use != "routes list|add|remove|refresh|verify" {
		t.Fatalf("routes shape = %q %q", cmd.Name(), cmd.Use)
	}
	var names []string
	for _, child := range cmd.Commands() {
		names = append(names, child.Name())
	}
	want := []string{"list", "add", "remove", "refresh", "verify"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("subcommands = %v, want %v", names, want)
	}
}

// F27-T1 row 3: add flags — --provider "", --model-id "", --model "",
// --reasoning "default", --window string slice [].
func TestRoutesAddFlags(t *testing.T) {
	add, _, err := NewRoutesCmd().Find([]string{"add"})
	if err != nil {
		t.Fatal(err)
	}
	routesCheckFlag(t, add, "provider", "string", "")
	routesCheckFlag(t, add, "model-id", "string", "")
	routesCheckFlag(t, add, "model", "string", "")
	routesCheckFlag(t, add, "reasoning", "string", "default")
	routesCheckFlag(t, add, "window", "stringSlice", "[]")
}

// F27-T1 row 4: remove --provider/--model-id ""; refresh --auto "";
// list --provider "".
func TestRoutesSubcommandFlagDefaults(t *testing.T) {
	remove, _, err := NewRoutesCmd().Find([]string{"remove"})
	if err != nil {
		t.Fatal(err)
	}
	routesCheckFlag(t, remove, "provider", "string", "")
	routesCheckFlag(t, remove, "model-id", "string", "")

	refresh, _, err := NewRoutesCmd().Find([]string{"refresh"})
	if err != nil {
		t.Fatal(err)
	}
	routesCheckFlag(t, refresh, "auto", "string", "")

	list, _, err := NewRoutesCmd().Find([]string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	routesCheckFlag(t, list, "provider", "string", "")
}

// F27-T1 row 5: RunE wiring — `routes add` and `routes remove` with no flags
// must route to the subcommand's Run* and fail as *UsageError naming
// --provider (exit 2).
func TestRoutesRunEArgValidation(t *testing.T) {
	for _, sub := range []string{"add", "remove"} {
		cmd := NewRoutesCmd()
		cmd.SetArgs([]string{sub})
		err := cmd.Execute()
		var ue *UsageError
		if !errors.As(err, &ue) {
			t.Fatalf("routes %s: err = %v (%T), want *UsageError", sub, err, err)
		}
		if !strings.Contains(ue.Message, "--provider") {
			t.Errorf("routes %s: message = %q, want mention of --provider", sub, ue.Message)
		}
		if ExitCodeFor(err) != 2 {
			t.Errorf("routes %s: exit = %d, want 2", sub, ExitCodeFor(err))
		}
	}
}
