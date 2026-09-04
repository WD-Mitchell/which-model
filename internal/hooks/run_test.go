// F29-T2/T3/T4/T5: run-core tests — usage-refresh (T2), quota-guard (T3),
// spawn-gate (T4), model-audit (T5)
// (specs/features/F29-agent-hooks/TASKS.md tasks F29-T2..F29-T5).
package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeRunner returns a Runner that writes out and returns code, recording
// nothing (see capturingRunner for argv capture).
func fakeRunner(code int, out string) Runner {
	return func(args []string, stdout, stderr io.Writer) int {
		stdout.Write([]byte(out))
		return code
	}
}

// capturingRunner records every argv slice handed to it and feeds back out.
func capturingRunner(out string, code int) (Runner, *[][]string) {
	var got [][]string
	r := func(args []string, stdout, stderr io.Writer) int {
		got = append(got, args)
		stdout.Write([]byte(out))
		return code
	}
	return r, &got
}

const usageRefreshGolden = "{\"decision\":\"approve\",\"reason\":\"usage cache refreshed\",\"hookSpecificOutput\":{}}\n"

const quotaFixture = `{"schema_version":"2.0","usage_enabled":true,"snapshots":[{"provider":"claude","confidence":"live","windows":[]},{"provider":"codex","confidence":"live","windows":[]}]}`

const quotaGolden = "{\"decision\":\"block\",\"reason\":\"quota guard: 1 provider(s) at or above critical band\",\"hookSpecificOutput\":{\"critical_providers\":[\"claude\"]}}\n"

const pickFixture = `{"candidates":[{"candidate_id":"cand-1","route":{"provider":"claude","model_id":"m1"}},{"candidate_id":"cand-2","route":{"provider":"codex","model_id":"m2"}}]}`

const pickFirstCandidate = `{"candidate_id":"cand-1","route":{"provider":"claude","model_id":"m1"}}`

const pickExit4Fixture = `{"excluded_candidates":[{"route":{"provider":"claude"},"reason_code":"band_gated","reason":"at gate"},{"route":{"provider":"codex"},"reason_code":"auth_required","reason":"login"}]}`

const explainFixture = `{"schema_version":"2.0","candidate":"claude:claude-sonnet-4-5","evidence":{"profile":"balanced_implementation","score_inputs":{},"route_provenance":"provider_live","excluded_candidates":[]}}`

// envelopeBytes decodes a Run output into the envelope shape, keeping
// hookSpecificOutput raw for verbatim comparisons.
func envelopeBytes(t *testing.T, out []byte) (string, string, map[string]json.RawMessage) {
	t.Helper()
	var e struct {
		Decision           string                     `json:"decision"`
		Reason             string                     `json:"reason"`
		HookSpecificOutput map[string]json.RawMessage `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &e); err != nil {
		t.Fatalf("output is not an envelope: %v\n%s", err, out)
	}
	return e.Decision, e.Reason, e.HookSpecificOutput
}

// ---- F29-T2: usage-refresh ----

// Test 1: runner exit 0 → exact approve envelope bytes.
func TestRunUsageRefreshSuccess(t *testing.T) {
	out, err := Run("usage-refresh", nil, Options{
		Runner:   fakeRunner(0, `{"schema_version":"2.0","usage_enabled":true,"snapshots":[]}`),
		RepoRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if string(out) != usageRefreshGolden {
		t.Errorf("output = %q, want %q", out, usageRefreshGolden)
	}
}

// Test 2/3: underlying non-zero exit → empty output, nil error (fail-open
// silence), whatever stdout carried.
func TestRunUsageRefreshFailOpen(t *testing.T) {
	for _, tc := range []struct {
		code int
		out  string
	}{
		{2, ""},
		{5, "auth needed"},
	} {
		out, err := Run("usage-refresh", nil, Options{Runner: fakeRunner(tc.code, tc.out)})
		if err != nil {
			t.Fatalf("code %d: Run() error = %v", tc.code, err)
		}
		if len(out) != 0 {
			t.Errorf("code %d: output = %q, want empty", tc.code, out)
		}
	}
}

// Test 4: unknown hook name → errUnknownHook sentinel.
func TestRunUnknownHook(t *testing.T) {
	_, err := Run("nonsense", nil, Options{})
	if !errors.Is(err, errUnknownHook) {
		t.Errorf("error = %v, want errUnknownHook", err)
	}
}

// Test 5: non-empty stdin that is not valid JSON → errBadStdin sentinel.
func TestRunBadStdin(t *testing.T) {
	_, err := Run("usage-refresh", nil, Options{Stdin: []byte("not json")})
	if !errors.Is(err, errBadStdin) {
		t.Errorf("error = %v, want errBadStdin", err)
	}
}

// Host context never substitutes for a command result.
func TestRunHostStdinExecutesRunner(t *testing.T) {
	for _, hook := range []string{"usage-refresh", "quota-guard", "spawn-gate", "model-audit"} {
		t.Run(hook, func(t *testing.T) {
			runner, calls := capturingRunner(`{"snapshots":[]}`, 2)
			out, err := Run(hook, []string{"--quiet"}, Options{Runner: runner, Stdin: []byte(`{"tool_name":"Task","secret":"CANARY_HOST"}`)})
			if err != nil || len(*calls) != 1 {
				t.Fatalf("Run error=%v calls=%v", err, *calls)
			}
			if bytes.Contains(out, []byte("CANARY_HOST")) {
				t.Fatalf("host context leaked: %s", out)
			}
			if hook == "usage-refresh" || hook == "quota-guard" {
				if len(out) != 0 {
					t.Fatalf("failed session hook must be silent: %s", out)
				}
			} else if !bytes.Contains(out, []byte("fail-open:")) {
				t.Fatalf("missing fail-open: %s", out)
			}
		})
	}
}

func TestRunRejectsNonObjectHostInput(t *testing.T) {
	for _, input := range []string{"null", "[]", "1", `"hello"`, "true", "{} {}"} {
		runner, calls := capturingRunner("", 0)
		_, err := Run("usage-refresh", nil, Options{Runner: runner, Stdin: []byte(input)})
		if !errors.Is(err, errBadStdin) || len(*calls) != 0 {
			t.Errorf("input %s: error=%v calls=%v", input, err, *calls)
		}
	}
}

// ---- F29-T3: quota-guard ----

// Test 1: two providers in order → exact block envelope.
func TestQuotaGuardBlocks(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(0, quotaFixture)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "{\"decision\":\"block\",\"reason\":\"quota guard: 2 provider(s) at or above critical band\",\"hookSpecificOutput\":{\"critical_providers\":[\"claude\",\"codex\"]}}\n"
	if string(out) != want {
		t.Errorf("output = %s, want %s", out, want)
	}
}

// Test 2: empty snapshots array → approve envelope.
func TestQuotaGuardNoProviders(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(0, `{"snapshots":[]}`)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "{\"decision\":\"approve\",\"reason\":\"no providers at or above critical band\",\"hookSpecificOutput\":{}}\n"
	if string(out) != want {
		t.Errorf("output = %s, want %s", out, want)
	}
}

// Test 3: duplicate providers are de-duplicated in first-seen order.
func TestQuotaGuardDeduplicates(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(0, `{"snapshots":[{"provider":"codex"},{"provider":"claude"},{"provider":"codex"}]}`)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	_, _, hso := envelopeBytes(t, out)
	var providers []string
	if err := json.Unmarshal(hso["critical_providers"], &providers); err != nil {
		t.Fatalf("critical_providers not an array: %v", err)
	}
	if !reflect.DeepEqual(providers, []string{"codex", "claude"}) {
		t.Errorf("critical_providers = %v, want [codex claude]", providers)
	}
}

// Test 4: missing snapshots key → empty output, nil error (fail-open).
func TestQuotaGuardMissingSnapshots(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(0, `{"schema_version":"2.0"}`)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(out) != 0 {
		t.Errorf("output = %q, want empty", out)
	}
}

// Test 5: underlying exit 1 with valid stdout → empty output (fail-open).
func TestQuotaGuardRunnerFail(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(1, quotaFixture)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(out) != 0 {
		t.Errorf("output = %q, want empty", out)
	}
}

// Test 6: exit 0 with non-JSON stdout → empty output (unparseable).
func TestQuotaGuardUnparseable(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(0, "not json")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(out) != 0 {
		t.Errorf("output = %q, want empty", out)
	}
}

// Test 7 (canary): canary in an unrelated fixture field never reaches the
// envelope.
func TestQuotaGuardCanary(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(0, `{"snapshots":[{"provider":"claude"}],"secret":"CANARY_QUOTA"}`)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if bytes.Contains(out, []byte("CANARY_QUOTA")) {
		t.Errorf("envelope leaks canary: %s", out)
	}
}

// Test 8: golden block envelope is byte-exact.
func TestQuotaGuardGolden(t *testing.T) {
	out, err := Run("quota-guard", nil, Options{Runner: fakeRunner(0, `{"snapshots":[{"provider":"claude"}]}`)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if string(out) != quotaGolden {
		t.Errorf("output = %s, want %s", out, quotaGolden)
	}
}

// ---- F29-T4: spawn-gate ----

// Test 1: exit 0 with a 2-candidate fixture → approve; candidate verbatim;
// reason names the first candidate.
func TestSpawnGateApprove(t *testing.T) {
	out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(0, pickFixture)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, hso := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if reason != "dispatch approved: cand-1" {
		t.Errorf("reason = %q, want %q", reason, "dispatch approved: cand-1")
	}
	if got := string(hso["candidate"]); got != pickFirstCandidate {
		t.Errorf("candidate = %s, want %s (verbatim)", got, pickFirstCandidate)
	}
}

// Test 2: exit 0 with an empty candidates array → fail-open approve naming
// exit 0.
func TestSpawnGateEmptyCandidates(t *testing.T) {
	out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(0, `{"candidates":[]}`)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, hso := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if !strings.Contains(reason, "exited 0") {
		t.Errorf("reason = %q, want it to contain %q", reason, "exited 0")
	}
	if len(hso) != 0 {
		t.Errorf("hookSpecificOutput = %v, want empty", hso)
	}
}

// Test 3: exit 4 with mixed exclusions → block naming band-gated providers;
// excluded_candidates verbatim.
func TestSpawnGateBlock(t *testing.T) {
	out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(4, pickExit4Fixture)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, hso := envelopeBytes(t, out)
	if decision != "block" {
		t.Errorf("decision = %q, want block", decision)
	}
	if reason != "all eligible providers band-gated: claude" {
		t.Errorf("reason = %q, want %q", reason, "all eligible providers band-gated: claude")
	}
	if got := string(hso["excluded_candidates"]); got != `[{"route":{"provider":"claude"},"reason_code":"band_gated","reason":"at gate"},{"route":{"provider":"codex"},"reason_code":"auth_required","reason":"login"}]` {
		t.Errorf("excluded_candidates = %s, want verbatim array", got)
	}
}

// Test 4: exit 4 with no band-gated entries → fail-open approve.
func TestSpawnGateExit4NoBandGated(t *testing.T) {
	fixture := `{"excluded_candidates":[{"route":{"provider":"codex"},"reason_code":"auth_required","reason":"login"}]}`
	out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(4, fixture)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, hso := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if !strings.Contains(reason, "exited 4") {
		t.Errorf("reason = %q, want it to contain %q", reason, "exited 4")
	}
	if len(hso) != 0 {
		t.Errorf("hookSpecificOutput = %v, want empty", hso)
	}
}

// Test 5: exit 4 with non-JSON stdout → fail-open approve.
func TestSpawnGateExit4Unparseable(t *testing.T) {
	out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(4, "not json")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, _ := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if !strings.Contains(reason, "exited 4") {
		t.Errorf("reason = %q, want it to contain %q", reason, "exited 4")
	}
}

// Test 6: exits 1/2/3/5 → fail-open approve naming the exit; payload {}.
func TestSpawnGateFailOpenCodes(t *testing.T) {
	for _, code := range []int{1, 2, 3, 5} {
		out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(code, "")})
		if err != nil {
			t.Fatalf("code %d: Run() error = %v", code, err)
		}
		decision, reason, hso := envelopeBytes(t, out)
		if decision != "approve" {
			t.Errorf("code %d: decision = %q, want approve", code, decision)
		}
		if !strings.Contains(reason, "exited "+string(rune('0'+code))) {
			t.Errorf("code %d: reason = %q, want it to contain %q", code, reason, "exited "+string(rune('0'+code)))
		}
		if len(hso) != 0 {
			t.Errorf("code %d: hookSpecificOutput = %v, want empty", code, hso)
		}
	}
}

// Test 7: exit 0 with non-JSON stdout → fail-open approve.
func TestSpawnGateExit0Unparseable(t *testing.T) {
	out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(0, "not json")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, _ := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if !strings.Contains(reason, "exited 0") {
		t.Errorf("reason = %q, want it to contain %q", reason, "exited 0")
	}
}

// Test 8: WHICH_MODEL_TASK_PROFILE plumbed into the underlying argv.
func TestSpawnGateProfileEnv(t *testing.T) {
	runner, got := capturingRunner(pickFixture, 0)
	if _, err := Run("spawn-gate", nil, Options{Runner: runner, Env: map[string]string{"WHICH_MODEL_TASK_PROFILE": "research"}}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("runner called %d times, want 1", len(*got))
	}
	argv := (*got)[0]
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "--profile" && argv[i+1] != "research" {
			t.Errorf("argv = %v, want --profile research", argv)
		}
	}
	if !reflect.DeepEqual(argv, []string{"pick", "--profile", "research", "--strategy", "priority", "--json"}) {
		t.Errorf("argv = %v, want exact underlying command", argv)
	}
}

// Test 9 (canary): only candidates[0] is echoed; extra fields never leak.
func TestSpawnGateCanary(t *testing.T) {
	out, err := Run("spawn-gate", nil, Options{Runner: fakeRunner(0, `{"candidates":[{"candidate_id":"cand-1"}],"secret":"CANARY_PICK"}`)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if bytes.Contains(out, []byte("CANARY_PICK")) {
		t.Errorf("envelope leaks canary: %s", out)
	}
}

// ---- F29-T5: model-audit ----

// Test 1: exit 0 with a valid explain fixture → approve; evidence_logged
// path; one line == fixture; mode 0600.
func TestModelAuditRecordsEvidence(t *testing.T) {
	root := t.TempDir()
	out, err := Run("model-audit", nil, Options{Runner: fakeRunner(0, explainFixture), RepoRoot: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, hso := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if reason != "dispatch evidence recorded" {
		t.Errorf("reason = %q, want %q", reason, "dispatch evidence recorded")
	}
	var logged string
	if err := json.Unmarshal(hso["evidence_logged"], &logged); err != nil {
		t.Fatalf("evidence_logged not a string: %v", err)
	}
	if !strings.HasSuffix(logged, "/.which-model/evidence.jsonl") {
		t.Errorf("evidence_logged = %q, want suffix %q", logged, "/.which-model/evidence.jsonl")
	}
	if string(hso["mismatch"]) != "false" {
		t.Errorf("mismatch = %s, want false", hso["mismatch"])
	}
	path := filepath.Join(root, ".which-model", "evidence.jsonl")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("evidence file missing: %v", err)
	}
	if string(content) != explainFixture+"\n" {
		t.Errorf("evidence content = %q, want fixture + newline", content)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("evidence file mode = %o, want 600", perm)
	}
}

// Test 2: env dispatched model matches route model_id → no mismatch file,
// mismatch=false.
func TestModelAuditNoMismatch(t *testing.T) {
	root := t.TempDir()
	out, err := Run("model-audit", nil, Options{
		Runner:   fakeRunner(0, explainFixture),
		RepoRoot: root,
		Env:      map[string]string{"WHICH_MODEL_DISPATCHED_MODEL": "claude-sonnet-4-5"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	_, _, hso := envelopeBytes(t, out)
	if string(hso["mismatch"]) != "false" {
		t.Errorf("mismatch = %s, want false", hso["mismatch"])
	}
	if _, err := os.Stat(filepath.Join(root, ".which-model", "audit-mismatches.jsonl")); !os.IsNotExist(err) {
		t.Errorf("audit-mismatches.jsonl exists or stat failed: %v", err)
	}
}

// Test 3: env dispatched model differs → mismatch=true and a mismatches
// line with the documented fields.
func TestModelAuditMismatch(t *testing.T) {
	root := t.TempDir()
	out, err := Run("model-audit", nil, Options{
		Runner:   fakeRunner(0, explainFixture),
		RepoRoot: root,
		Env:      map[string]string{"WHICH_MODEL_DISPATCHED_MODEL": "gpt-5"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	_, _, hso := envelopeBytes(t, out)
	if string(hso["mismatch"]) != "true" {
		t.Errorf("mismatch = %s, want true", hso["mismatch"])
	}
	content, err := os.ReadFile(filepath.Join(root, ".which-model", "audit-mismatches.jsonl"))
	if err != nil {
		t.Fatalf("audit-mismatches.jsonl missing: %v", err)
	}
	var rec map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(content), &rec); err != nil {
		t.Fatalf("mismatch line not JSON: %v (%s)", err, content)
	}
	if got := string(rec["dispatched_model"]); got != `"gpt-5"` {
		t.Errorf("dispatched_model = %s, want gpt-5", got)
	}
	if got := string(rec["route_model_id"]); got != `"claude-sonnet-4-5"` {
		t.Errorf("route_model_id = %s, want claude-sonnet-4-5", got)
	}
	if rec["ts"] == nil {
		t.Error("mismatch line missing ts")
	}
	if rec["evidence"] == nil {
		t.Error("mismatch line missing evidence")
	}
	if got := string(rec["evidence"]); got != explainFixture {
		t.Errorf("evidence = %s, want full explain object", got)
	}
}

// Test 4: a second run appends a second line; both lines intact.
func TestModelAuditAppends(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		if _, err := Run("model-audit", nil, Options{Runner: fakeRunner(0, explainFixture), RepoRoot: root}); err != nil {
			t.Fatalf("run %d: Run() error = %v", i, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, ".which-model", "evidence.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	for i, line := range lines {
		if line != explainFixture {
			t.Errorf("line %d = %q, want the fixture", i, line)
		}
	}
}

// Test 5: underlying exit 1 → fail-open approve; no evidence file.
func TestModelAuditFailOpen(t *testing.T) {
	root := t.TempDir()
	out, err := Run("model-audit", nil, Options{Runner: fakeRunner(1, ""), RepoRoot: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, hso := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if !strings.Contains(reason, "exited 1") {
		t.Errorf("reason = %q, want it to contain %q", reason, "exited 1")
	}
	if len(hso) != 0 {
		t.Errorf("hookSpecificOutput = %v, want empty", hso)
	}
	if _, err := os.Stat(filepath.Join(root, ".which-model", "evidence.jsonl")); !os.IsNotExist(err) {
		t.Errorf("evidence file created on failure: %v", err)
	}
}

// Test 6: exit 0 fixture without a candidate key → fail-open; no file.
func TestModelAuditNoCandidate(t *testing.T) {
	root := t.TempDir()
	out, err := Run("model-audit", nil, Options{Runner: fakeRunner(0, `{"schema_version":"2.0"}`), RepoRoot: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, _, _ := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if _, err := os.Stat(filepath.Join(root, ".which-model", "evidence.jsonl")); !os.IsNotExist(err) {
		t.Errorf("evidence file created without candidate: %v", err)
	}
}

// Test 7: exit 0 fixture whose candidate lacks route → fail-open; no file.
func TestModelAuditCandidateNoRoute(t *testing.T) {
	root := t.TempDir()
	out, err := Run("model-audit", nil, Options{Runner: fakeRunner(0, `{"candidate":{"candidate_id":"c-1"}}`), RepoRoot: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, _, _ := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if _, err := os.Stat(filepath.Join(root, ".which-model", "evidence.jsonl")); !os.IsNotExist(err) {
		t.Errorf("evidence file created without route: %v", err)
	}
}

// Test 8 (canary): canary in evidence.score_inputs never reaches the
// envelope (nor the evidence file).
func TestModelAuditCanary(t *testing.T) {
	root := t.TempDir()
	fixture := `{"schema_version":"2.0","candidate":"claude:m1","secret":"CANARY_EVIDENCE","evidence":{"profile":"balanced_implementation","score_inputs":{},"secret":"CANARY_EVIDENCE","route_provenance":"provider_live","excluded_candidates":[]}}`
	out, err := Run("model-audit", nil, Options{Runner: fakeRunner(0, fixture), RepoRoot: root})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if bytes.Contains(out, []byte("CANARY_EVIDENCE")) {
		t.Errorf("envelope leaks canary: %s", out)
	}
	content, err := os.ReadFile(filepath.Join(root, ".which-model", "evidence.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte("CANARY_EVIDENCE")) {
		t.Fatalf("evidence leaks canary: %s", content)
	}
}

// Test 9: empty RepoRoot → fail-open approve, no error.
func TestModelAuditNoRepoRoot(t *testing.T) {
	out, err := Run("model-audit", nil, Options{Runner: fakeRunner(0, explainFixture)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	decision, reason, _ := envelopeBytes(t, out)
	if decision != "approve" {
		t.Errorf("decision = %q, want approve", decision)
	}
	if !strings.Contains(reason, "exited 0") {
		t.Errorf("reason = %q, want it to contain %q", reason, "exited 0")
	}
}

// Test 10: passthrough --last plus env candidate id → argv is
// ["explain","--last","--json","--last"] (env id is correlation only).
func TestModelAuditPassthrough(t *testing.T) {
	runner, got := capturingRunner(explainFixture, 0)
	if _, err := Run("model-audit", []string{"--last"}, Options{
		Runner:   runner,
		Env:      map[string]string{"WHICH_MODEL_CANDIDATE_ID": "c-9"},
		RepoRoot: t.TempDir(),
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("runner called %d times, want 1", len(*got))
	}
	want := []string{"explain", "--last", "--json", "--last"}
	if !reflect.DeepEqual((*got)[0], want) {
		t.Errorf("argv = %v, want %v", (*got)[0], want)
	}
}

func TestAuditRejectsIncompleteEvidence(t *testing.T) {
	for _, evidence := range []string{`{}`, `{"profile":"p","score_inputs":{},"route_provenance":"invalid","excluded_candidates":[]}`, `{"profile":"p","score_inputs":{},"route_provenance":"provider_live"}`} {
		t.Run(evidence, func(t *testing.T) {
			root := t.TempDir()
			out, err := Run("model-audit", nil, Options{Runner: fakeRunner(0, `{"schema_version":"2.0","candidate":"claude:m1","evidence":`+evidence+`}`), RepoRoot: root})
			if err != nil || !strings.Contains(string(out), "fail-open") {
				t.Fatalf("got %s, %v", out, err)
			}
			if _, err := os.Stat(filepath.Join(root, ".which-model")); !os.IsNotExist(err) {
				t.Fatalf("incomplete evidence created audit directory: %v", err)
			}
		})
	}
}
