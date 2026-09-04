// F29-T7: hooks CLI tests (specs/features/F29-agent-hooks/TASKS.md task
// F29-T7, CLI half).
package whichmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/skills"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// hooksRepo makes a repo-shaped temp dir and resets the skills repo-dir
// seam so cwd-based discovery is authoritative.
func hooksRepo(t *testing.T) string {
	t.Helper()
	skills.SetRepoDir("")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// Test 1: `hooks install --repo <tmp>` → exit 0; settings.json and the
// manifest exist. Variant is the real resolved state (auto), which is
// deterministic per machine config; the files must exist either way.
func TestHooksInstallRepo(t *testing.T) {
	repo := hooksRepo(t)
	code, stdout, stderr := captureExecuteFresh(t, []string{"hooks", "install", "--repo", repo})
	if code != 0 {
		t.Fatalf("hooks install: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "installed") {
		t.Errorf("stdout = %q, want install summary lines", stdout)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "settings.json")); err != nil {
		t.Errorf("settings.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "which-model-hooks.json")); err != nil {
		t.Errorf("manifest missing: %v", err)
	}
}

// Test 2: `hooks install --repo <tmp> --no-usage` → exit 0; manifest has 2
// entries; settings.json lacks usage-refresh/quota-guard.
func TestHooksInstallNoUsage(t *testing.T) {
	repo := hooksRepo(t)
	code, _, stderr := captureExecuteFresh(t, []string{"hooks", "install", "--repo", repo, "--no-usage"})
	if code != 0 {
		t.Fatalf("hooks install --no-usage: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(repo, ".claude", "which-model-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `"usage-refresh"`) || strings.Contains(s, `"quota-guard"`) {
		t.Errorf("manifest = %s, want NO usage hooks", s)
	}
	settings, err := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(settings), "which-model hooks run usage-refresh") ||
		strings.Contains(string(settings), "which-model hooks run quota-guard") {
		t.Errorf("settings.json contains usage hooks under --no-usage:\n%s", settings)
	}
	if !strings.Contains(string(settings), "which-model hooks run spawn-gate --no-usage --profile balanced_implementation --quiet") {
		t.Errorf("settings.json lacks the variant-B spawn-gate command:\n%s", settings)
	}
	if !strings.Contains(string(settings), "which-model hooks run model-audit --last") {
		t.Errorf("settings.json lacks the variant-B model-audit command:\n%s", settings)
	}
}

// Test 3: --usage + --no-usage together → exit 2.
func TestHooksInstallUsageConflict(t *testing.T) {
	repo := hooksRepo(t)
	code, _, stderr := captureExecuteFresh(t, []string{"hooks", "install", "--repo", repo, "--usage", "--no-usage"})
	if code != 2 {
		t.Errorf("exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "--usage and --no-usage are mutually exclusive") {
		t.Errorf("stderr = %q, want mutual-exclusion message", stderr)
	}
}

// Test 4: unknown target → exit 2.
func TestHooksInstallUnknownTarget(t *testing.T) {
	repo := hooksRepo(t)
	code, _, stderr := captureExecuteFresh(t, []string{"hooks", "install", "--repo", repo, "--target", "codex"})
	if code != 2 {
		t.Errorf("exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "unknown target") {
		t.Errorf("stderr = %q, want unknown target", stderr)
	}
}

// Test 5: install with no repo root and no --repo → exit 1.
func TestHooksInstallNoRepoRoot(t *testing.T) {
	skills.SetRepoDir("")
	t.Chdir(t.TempDir())
	code, _, stderr := captureExecuteFresh(t, []string{"hooks", "install"})
	if code != 1 {
		t.Errorf("exit = %d, want 1 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "repository root") {
		t.Errorf("stderr = %q, want repo-root error", stderr)
	}
}

// Test 6: install then `hooks remove --repo` → exit 0; no trace left.
func TestHooksRemoveNoTrace(t *testing.T) {
	repo := hooksRepo(t)
	if code, _, stderr := captureExecuteFresh(t, []string{"hooks", "install", "--repo", repo, "--usage"}); code != 0 {
		t.Fatalf("setup install: exit = %d (stderr: %s)", code, stderr)
	}
	code, stdout, stderr := captureExecuteFresh(t, []string{"hooks", "remove", "--repo", repo})
	if code != 0 {
		t.Fatalf("hooks remove: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("stdout = %q, want remove summary", stdout)
	}
	for _, rel := range []string{".claude/settings.json", ".claude/which-model-hooks.json", "agents/hooks.toml"} {
		if _, err := os.Stat(filepath.Join(repo, rel)); !os.IsNotExist(err) {
			t.Errorf("%s still exists after remove: %v", rel, err)
		}
	}
	// Second remove is a no-op success.
	code, stdout, stderr = captureExecuteFresh(t, []string{"hooks", "remove", "--repo", repo})
	if code != 0 {
		t.Errorf("second remove: exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "nothing to remove") {
		t.Errorf("second remove stdout = %q, want nothing to remove", stdout)
	}
}

// captureExecuteWithStdin runs the CLI with a given stdin reader, mirroring
// captureExecute but injecting stdin via the root command.
func captureExecuteWithStdin(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	resetRegistryBuildCache()
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	prevOut, prevErr := Stdout, Stderr
	Stdout, Stderr = outBuf, errBuf
	defer func() { Stdout, Stderr = prevOut, prevErr }()
	root := NewRootCmd()
	root.SetArgs(args)
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	found, err := root.ExecuteC()
	if err != nil {
		renderError(found, err)
		return ExitCodeFor(err), outBuf.String(), errBuf.String()
	}
	return 0, outBuf.String(), errBuf.String()
}

// Test 8: `hooks run nonsense` → exit 2; stderr names the unknown hook.
func TestHooksRunUnknownHook(t *testing.T) {
	skills.SetRepoDir("")
	code, _, stderr := captureExecuteFresh(t, []string{"hooks", "run", "nonsense"})
	if code != 2 {
		t.Errorf("exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "unknown hook") {
		t.Errorf("stderr = %q, want unknown hook", stderr)
	}
	if !strings.Contains(stderr, "usage-refresh, quota-guard, spawn-gate, model-audit") {
		t.Errorf("stderr = %q, want the known-hook list", stderr)
	}
}

// Test 9: `hooks run usage-refresh` with non-JSON stdin → exit 2; stderr
// mentions the invalid JSON.
func TestHooksRunBadStdin(t *testing.T) {
	skills.SetRepoDir("")
	code, _, stderr := captureExecuteWithStdin(t, []string{"hooks", "run", "usage-refresh"}, "not json")
	if code != 2 {
		t.Errorf("exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "not valid JSON") {
		t.Errorf("stderr = %q, want not valid JSON", stderr)
	}
}

// Test 10: hooks command is registered in the tree (F22 commandOrder).
func TestHooksCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range registeredCommands() {
		if cmd.Name() == "hooks" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("registeredCommands() does not contain hooks")
	}
}

// Test 11: ExecuteCommand runs an in-process subcommand and captures its
// streams (the Runner default for hooks run).
func TestExecuteCommandInProcess(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := ExecuteCommand([]string{"version"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("ExecuteCommand(version) = %d, want 0", code)
	}
	if !strings.HasPrefix(out.String(), "which-model ") {
		t.Errorf("stdout = %q, want version line", out.String())
	}
	// Streams are restored afterwards.
	_ = code
}

// The two SessionStart hooks execute end to end against the actual command
// tree. This catches registry argv that only looks plausible but parses as an
// unknown provider, subcommand, or flag.
func TestSessionStartHooksExecuteUsageCommand(t *testing.T) {
	oldFetch := fetchAllFunc
	calls := 0
	t.Cleanup(func() {
		fetchAllFunc = oldFetch
		Global = GlobalFlags{}
	})
	used := 80.0
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		calls++
		return &FetchResult{Snapshots: []usage.Snapshot{{
			Provider: "claude",
			Windows: []usage.Window{{
				ID:          "5h",
				UsedPercent: &used,
				UsageKnown:  true,
			}},
			UsageKnown: true,
		}}}, nil
	}
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte("[usage]\nbackend = \"native\"\n[providers.claude]\nenabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantOutput := map[string]string{
		"usage-refresh": `"reason":"usage cache refreshed"`,
		"quota-guard":   `"critical_providers":["claude"]`,
	}
	for id, want := range wantOutput {
		t.Run(id, func(t *testing.T) {
			Global = GlobalFlags{}
			resetRegistryBuildCache()
			before := calls
			code, out, stderr := captureExecuteWithStdin(t, []string{"hooks", "run", id, "--config", cfg}, `{"hook_event_name":"SessionStart","secret":"CANARY_HOST"}`)
			if code != 0 || calls != before+1 {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls-before, stderr)
			}
			if strings.Contains(out, "CANARY_HOST") {
				t.Fatal("host context leaked")
			}
			if !strings.Contains(string(out), want) {
				t.Fatalf("%s output = %q, want %s", id, out, want)
			}
		})
	}
}

func TestHookInheritsOuterGlobalFlags(t *testing.T) {
	oldFetch := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = oldFetch; Global = GlobalFlags{} })
	cfg := pickTestConfig(t, t.TempDir(), "[usage]\nbackend=\"native\"\n[providers.claude]\nenabled=true\n")
	for _, override := range []bool{false, true} {
		calls := 0
		fetchAllFunc = func(_ context.Context, opts FetchAllOptions) (*FetchResult, error) {
			calls++
			want := 2 * time.Second
			if override {
				want = time.Second
			}
			if !opts.Offline || opts.Timeout != want || !opts.ForceRefresh {
				t.Fatalf("lost outer/passthrough flags: %+v", opts)
			}
			return &FetchResult{Snapshots: []usage.Snapshot{}}, nil
		}
		args := []string{"--offline", "--config", cfg, "--timeout", "2s", "hooks", "run", "usage-refresh"}
		if override {
			args = append(args, "--timeout", "1s")
		}
		code, out, stderr := captureExecuteWithStdin(t, args, `{}`)
		if code != 0 || calls != 1 || !strings.Contains(out, "usage cache refreshed") {
			t.Fatalf("code=%d calls=%d out=%s stderr=%s", code, calls, out, stderr)
		}
	}
}

func TestModelAuditConsumesRealExplainOutput(t *testing.T) {
	repo := hooksRepo(t)
	t.Chdir(repo)
	state := t.TempDir()
	setStateDir(t, func() string { return state })
	cfg := pickTestConfig(t, t.TempDir(), "")
	const firstID = "01K20000000000000000000000"
	seedExplainHistory(t, state,
		explainHistoryEntry(firstID, "claude:model:revision", 90, f26FullEvidence()),
		explainHistoryEntry("01K20000000000000000000001", "codex:latest", 80, f26FullEvidence()))
	t.Setenv("WHICH_MODEL_CANDIDATE_ID", "claude:model:revision")
	t.Setenv("WHICH_MODEL_DISPATCHED_MODEL", "model:revision")
	t.Cleanup(func() { Global = GlobalFlags{} })
	code, out, stderr := captureExecuteWithStdin(t,
		[]string{"hooks", "run", "model-audit", "--pick-id", firstID, "--config", cfg},
		`{"tool_name":"Task","secret":"CANARY_HOST"}`)
	if code != 0 || !strings.Contains(out, `"mismatch":false`) {
		t.Fatalf("code=%d out=%s stderr=%s", code, out, stderr)
	}
	path := filepath.Join(repo, ".which-model", "evidence.jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "\n") != 1 || strings.Contains(string(b), "CANARY_HOST") {
		t.Fatalf("invalid JSONL: %s", b)
	}
	var doc ExplainResult
	if err := json.Unmarshal(b, &doc); err != nil || doc.Candidate != "claude:model:revision" || !reflect.DeepEqual(doc.Evidence, f26FullEvidence()) {
		t.Fatalf("lost explain fields: %+v error=%v", doc, err)
	}
	// The latest record belongs to a different dispatch: do not log it as
	// evidence for WHICH_MODEL_CANDIDATE_ID.
	code, out, stderr = captureExecuteWithStdin(t, []string{"hooks", "run", "model-audit", "--config", cfg}, `{}`)
	if code != 0 || !strings.Contains(out, "fail-open:") {
		t.Fatalf("code=%d out=%s stderr=%s", code, out, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(b, after) {
		t.Fatalf("uncorrelated evidence was appended: %s error=%v", after, err)
	}
}
