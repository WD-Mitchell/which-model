package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
)

// harnessConfigEvents counts config:changed events emitted so far.
func harnessConfigEvents(rec *emitRecorder) int {
	n := 0
	for _, e := range rec.Events() {
		if e.Event == EventConfigChanged {
			n++
		}
	}
	return n
}

// harnessCfgFile returns the fixture config.toml bytes.
func harnessCfgFile(t *testing.T, svc *Services) string {
	t.Helper()
	data, err := os.ReadFile(svc.paths.UserConfigFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(data)
}

func mustListHarnesses(t *testing.T, svc *Services) []HarnessInfo {
	t.Helper()
	list, err := svc.Harnesses().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return list
}

// providersFixture configures three providers under [providers.*].
const providersFixture = "[providers.claude]\nenabled = true\n" +
	"[providers.codex]\nenabled = true\n" +
	"[providers.cursor]\nenabled = true\n"

// TestHarnessListSeedsBuiltins is fixture §7.1: first List on an empty config
// writes the four builtin seeds (one event), a second List emits nothing, and
// a non-empty [harnesses] section suppresses seeding forever.
func TestHarnessListSeedsBuiltins(t *testing.T) {
	svc, rec := newTestServices(t)
	list := mustListHarnesses(t, svc)
	if len(list) != 4 {
		t.Fatalf("List len = %d, want 4 (seeded)", len(list))
	}
	want := []HarnessInfo{
		{Slug: "claude", Name: "Claude Code", Command: "claude --model {model_id} --reasoning {reasoning}", Builtin: true},
		{Slug: "codex", Name: "Codex CLI", Command: "codex -m {model_id} -c reasoning={reasoning}", Builtin: true},
		{Slug: "copilot", Name: "Copilot CLI", Command: "copilot --model {model_id}", Builtin: true},
		{Slug: "cursor", Name: "Cursor", Command: "cursor --model {model_id}", Builtin: true},
	}
	for i, w := range want {
		g := list[i]
		if g.Slug != w.Slug || g.Name != w.Name || g.Command != w.Command {
			t.Fatalf("seed[%d] = %+v, want %+v", i, g, w)
		}
		if !g.Builtin {
			t.Fatalf("seed[%d].Builtin = false, want true", i)
		}
	}
	if got := harnessConfigEvents(rec); got != 1 {
		t.Fatalf("events after first List = %d, want 1", got)
	}
	cfg := harnessCfgFile(t, svc)
	for _, slug := range []string{"claude", "codex", "copilot", "cursor"} {
		if !strings.Contains(cfg, "[harnesses."+slug+"]") {
			t.Fatalf("config missing [harnesses.%s]\n%s", slug, cfg)
		}
	}
	if strings.Count(cfg, "builtin = true") != 4 {
		t.Fatalf("expected 4 builtin = true, got:\n%s", cfg)
	}

	// Second List emits nothing.
	mustListHarnesses(t, svc)
	if got := harnessConfigEvents(rec); got != 1 {
		t.Fatalf("events after second List = %d, want 1", got)
	}
}

// TestHarnessListNoSeedWhenCustomExists is §7.1: a single custom harness
// suppresses seeding forever.
func TestHarnessListNoSeedWhenCustomExists(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML("[harnesses.myh]\nname = \"My H\"\ncommand = \"tool\"\n"))
	list := mustListHarnesses(t, svc)
	if len(list) != 1 || list[0].Slug != "myh" || list[0].Builtin {
		t.Fatalf("List = %+v, want only custom myh", list)
	}
	if got := harnessConfigEvents(rec); got != 0 {
		t.Fatalf("events = %d, want 0 (no seeding)", got)
	}
}

// TestHarnessBuildCommandSubstitution is §7.2.
func TestHarnessBuildCommandSubstitution(t *testing.T) {
	svc, _ := newTestServices(t)
	mustListHarnesses(t, svc) // seed
	h := svc.Harnesses()

	cases := []struct {
		slug, modelID, reasoning, want string
	}{
		{"claude", "opus-5", "high", "claude --model opus-5 --reasoning high"},
		{"codex", "gpt-5.6", "medium", "codex -m gpt-5.6 -c reasoning=medium"},
		{"copilot", "gpt-5.6", "low", "copilot --model gpt-5.6"},
		{"cursor", "x-1", "max", "cursor --model x-1"},
	}
	for _, c := range cases {
		got, err := h.BuildCommand(c.slug, c.modelID, c.reasoning)
		if err != nil {
			t.Fatalf("BuildCommand(%s): %v", c.slug, err)
		}
		if got != c.want {
			t.Fatalf("BuildCommand(%s) = %q, want %q", c.slug, got, c.want)
		}
	}

	// "default" reasoning substitutes verbatim.
	got, err := h.BuildCommand("claude", "m1", "default")
	if err != nil {
		t.Fatalf("BuildCommand default: %v", err)
	}
	if !strings.Contains(got, "--reasoning default") {
		t.Fatalf("default reasoning not verbatim: %q", got)
	}

	// Unresolved token names the first offending placeholder (§6 #4).
	if err := testSaveCustom(t, svc, HarnessInfo{Slug: "myh", Name: "My H", Command: "x {model_id} {custom_flag}"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, err = h.BuildCommand("myh", "m1", "low")
	if !errors.Is(err, errValidation) {
		t.Fatalf("err = %v, want errValidation", err)
	}
	if err == nil || !strings.Contains(err.Error(), `harness "myh": unresolved template token "{custom_flag}"`) {
		t.Fatalf("unresolved-token message missing, got %v", err)
	}

	// Unknown slug (§6 #6).
	_, err = h.BuildCommand("nope", "m1", "low")
	if !errors.Is(err, errNotFound) {
		t.Fatalf("unknown slug err = %v, want errNotFound", err)
	}
}

func testSaveCustom(t *testing.T, svc *Services, in HarnessInfo) error {
	t.Helper()
	return svc.Harnesses().Save(context.Background(), in)
}

// TestHarnessListInstallDetection is §7.3: argv0 resolved on the live PATH.
func TestHarnessListInstallDetection(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "claude")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", dir)

	svc, _ := newTestServices(t)
	list := mustListHarnesses(t, svc)
	for _, h := range list {
		want := h.Slug == "claude"
		if h.Installed != want {
			t.Fatalf("%s.Installed = %v, want %v", h.Slug, h.Installed, want)
		}
	}

	// Empty / whitespace-only command is never installed (SPEC §2.4).
	if installed("") {
		t.Fatal("installed(\"\") = true, want false")
	}
	if installed("   ") {
		t.Fatal("installed(\"   \") = true, want false")
	}
}

// TestHarnessSaveSemantics is §7.4.
func TestHarnessSaveSemantics(t *testing.T) {
	providers := map[string]bool{"claude": true, "cursor": false}
	svc, rec := newTestServices(t, WithConfigTOML(providersFixture))
	if err := testSaveCustom(t, svc, HarnessInfo{Slug: "myh", Name: "My H", Command: "tool --model {model_id}", Providers: providers}); err != nil {
		t.Fatalf("Save custom: %v", err)
	}
	if got := harnessConfigEvents(rec); got != 1 {
		t.Fatalf("custom Save events = %d, want 1", got)
	}
	got := mustListHarnesses(t, svc)
	var found *HarnessInfo
	for i := range got {
		if got[i].Slug == "myh" {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatal("saved custom myh not listed")
	}
	if found.Builtin || found.Command != "tool --model {model_id}" || !found.Providers["claude"] || found.Providers["cursor"] {
		t.Fatalf("round-trip myh = %+v", *found)
	}
	cfg := harnessCfgFile(t, svc)
	if !strings.Contains(cfg, "builtin = false") {
		t.Fatalf("custom persisted without builtin = false:\n%s", cfg)
	}

	// Reset recorder for the builtin cases (separate Services to isolate).
	svc2, rec2 := newTestServices(t, WithConfigTOML(providersFixture))
	mustListHarnesses(t, svc2) // seed
	rec2s := func() []recordedEvent { return rec2.Events() }
	_ = rec2s

	// Builtin provider-map-only Save succeeds (name/command unchanged).
	if err := testSaveCustom(t, svc2, HarnessInfo{Slug: "claude", Name: "Claude Code", Command: "claude --model {model_id} --reasoning {reasoning}", Providers: map[string]bool{"claude": true}}); err != nil {
		t.Fatalf("builtin provider-only Save: %v", err)
	}

	// Builtin with changed command -> errBuiltinReadonly (§6 #5).
	err := testSaveCustom(t, svc2, HarnessInfo{Slug: "claude", Name: "Claude Code", Command: "claude --model {model_id}", Providers: map[string]bool{}})
	if !errors.Is(err, errBuiltinReadonly) {
		t.Fatalf("err = %v, want errBuiltinReadonly", err)
	}
	if err == nil || !strings.Contains(err.Error(), `harness "claude" is builtin: name and command are read-only`) {
		t.Fatalf("builtin readonly message missing, got %v", err)
	}
	if got := harnessConfigEvents(rec2); got != 2 {
		t.Fatalf("builtin events = %d, want 2 (seed + provider-only save; failure emitted none)", got)
	}
}

// TestHarnessSaveValidationOrder is §7.4: slug grammar is reported before
// an empty name (§6 #1).
func TestHarnessSaveValidationOrder(t *testing.T) {
	svc, rec := newTestServices(t)
	err := testSaveCustom(t, svc, HarnessInfo{Slug: "Bad!", Name: "", Command: ""})
	if !errors.Is(err, errValidation) {
		t.Fatalf("err = %v, want errValidation", err)
	}
	if err == nil || !strings.Contains(err.Error(), `harness slug "Bad!" must match [a-z0-9_]+`) {
		t.Fatalf("validation-order message missing, got %v", err)
	}
	if got := harnessConfigEvents(rec); got != 0 {
		t.Fatalf("validation failure events = %d, want 0", got)
	}

	// Empty name is #2, reported when slug is valid.
	svc2, _ := newTestServices(t)
	err = testSaveCustom(t, svc2, HarnessInfo{Slug: "myh", Name: "", Command: "x"})
	if err == nil || !strings.Contains(err.Error(), "harness name must not be empty") {
		t.Fatalf("empty-name message missing, got %v", err)
	}
}

// TestHarnessDeleteAny is §7.5.
func TestHarnessDeleteAny(t *testing.T) {
	svc, rec := newTestServices(t)
	mustListHarnesses(t, svc) // seed
	if err := svc.Harnesses().Delete(context.Background(), "claude"); err != nil {
		t.Fatalf("Delete claude: %v", err)
	}
	if got := harnessConfigEvents(rec); got != 2 { // seed + delete
		t.Fatalf("events = %d, want 2", got)
	}

	// Deleted builtin does not re-seed (section still non-empty, §2.2).
	list := mustListHarnesses(t, svc)
	if len(list) != 3 {
		t.Fatalf("after delete List len = %d, want 3 (no re-seed)", len(list))
	}
	for _, h := range list {
		if h.Slug == "claude" {
			t.Fatal("deleted claude reappeared")
		}
	}
	if got := harnessConfigEvents(rec); got != 2 {
		t.Fatalf("re-list emitted: events = %d, want 2", got)
	}

	// Unknown slug -> errNotFound.
	err := svc.Harnesses().Delete(context.Background(), "nope")
	if !errors.Is(err, errNotFound) {
		t.Fatalf("Delete unknown err = %v, want errNotFound", err)
	}
	if got := harnessConfigEvents(rec); got != 2 {
		t.Fatalf("failed delete emitted: events = %d, want 2", got)
	}
}

// TestHarnessSetProviderAll is §7.6.
func TestHarnessSetProviderAll(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(providersFixture))
	mustListHarnesses(t, svc) // seed
	h := svc.Harnesses()

	// Toggle on: adds cursor (idempotent; sorted list).
	if err := h.SetProvider(context.Background(), "claude", "cursor", true); err != nil {
		t.Fatalf("SetProvider on: %v", err)
	}
	before := harnessCfgFile(t, svc)
	if got := harnessConfigEvents(rec); got != 2 { // seed + toggle
		t.Fatalf("events after toggle-on = %d, want 2", got)
	}

	// Idempotence: repeat produces identical config bytes, still one event.
	if err := h.SetProvider(context.Background(), "claude", "cursor", true); err != nil {
		t.Fatalf("SetProvider repeat: %v", err)
	}
	if after := harnessCfgFile(t, svc); after != before {
		t.Fatalf("idempotent SetProvider changed config bytes\nbefore: %s\nafter: %s", before, after)
	}

	// Verify claude now allows cursor.
	list := mustListHarnesses(t, svc)
	for _, hh := range list {
		if hh.Slug == "claude" && !hh.Providers["cursor"] {
			t.Fatalf("claude should allow cursor: %+v", hh)
		}
	}

	// Toggle off: removes cursor.
	if err := h.SetProvider(context.Background(), "claude", "cursor", false); err != nil {
		t.Fatalf("SetProvider off: %v", err)
	}
	list = mustListHarnesses(t, svc)
	for _, hh := range list {
		if hh.Slug == "claude" && hh.Providers["cursor"] {
			t.Fatalf("claude should not allow cursor: %+v", hh)
		}
	}

	// Unknown provider -> validation_failed (#7), no event.
	err := h.SetProvider(context.Background(), "claude", "nope", true)
	if !errors.Is(err, errValidation) {
		t.Fatalf("unknown provider err = %v, want errValidation", err)
	}
	if err == nil || !strings.Contains(err.Error(), `unknown provider "nope"`) {
		t.Fatalf("unknown-provider message missing, got %v", err)
	}

	// Unknown harness -> not_found.
	err = h.SetProvider(context.Background(), "nope", "claude", true)
	if !errors.Is(err, errNotFound) {
		t.Fatalf("unknown harness err = %v, want errNotFound", err)
	}

	// SetAllProviders(true): list = every configured provider.
	if err := h.SetAllProviders(context.Background(), "codex", true); err != nil {
		t.Fatalf("SetAllProviders on: %v", err)
	}
	list = mustListHarnesses(t, svc)
	for _, hh := range list {
		if hh.Slug == "codex" {
			for _, p := range []string{"claude", "codex", "cursor"} {
				if !hh.Providers[p] {
					t.Fatalf("codex all-on missing %s: %+v", p, hh)
				}
			}
		}
	}

	// SetAllProviders(false): empty allow-list.
	if err := h.SetAllProviders(context.Background(), "codex", false); err != nil {
		t.Fatalf("SetAllProviders off: %v", err)
	}
	list = mustListHarnesses(t, svc)
	for _, hh := range list {
		if hh.Slug == "codex" {
			for _, on := range hh.Providers {
				if on {
					t.Fatalf("codex all-off still has enabled provider: %+v", hh)
				}
			}
		}
	}
}

// TestHarnessLaunchCopyMode is §7.7.
func TestHarnessLaunchCopyMode(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML("[gui]\ncopy_command_instead = true\n"))
	mustListHarnesses(t, svc) // seed
	calls := []string{}
	svc.recordPick = func(_ context.Context, profile, route string) error {
		calls = append(calls, profile+"|"+route)
		return nil
	}

	res, err := svc.Harnesses().Launch(context.Background(), "claude", "claude/opus-5@high", "profileA")
	if err != nil {
		t.Fatalf("Launch copy: %v", err)
	}
	if !res.Copied || res.Command != "claude --model opus-5 --reasoning high" {
		t.Fatalf("copy result = %+v", res)
	}
	if _, statErr := os.Stat(filepath.Join(svc.paths.StateDir, "launch.log")); !os.IsNotExist(statErr) {
		t.Fatalf("copy mode must not spawn: launch.log exists (statErr=%v)", statErr)
	}
	if len(calls) != 1 || calls[0] != "profileA|claude/opus-5@high" {
		t.Fatalf("recordPick calls = %v, want [profileA|claude/opus-5@high]", calls)
	}
	// Copy-mode launch emits nothing directly (pick:recorded arrives via
	// RecordPick, SPEC §2.11); only the seed event exists.
	if got := harnessConfigEvents(rec); got != 1 {
		t.Fatalf("events = %d, want 1 (seed only)", got)
	}
}

// TestHarnessLaunchBadRouteKey is §2.9.1: invalid grammar -> errValidation,
// nothing spawned, no pick recorded.
func TestHarnessLaunchBadRouteKey(t *testing.T) {
	svc, _ := newTestServices(t)
	mustListHarnesses(t, svc)
	called := false
	svc.recordPick = func(_ context.Context, _, _ string) error { called = true; return nil }
	_, err := svc.Harnesses().Launch(context.Background(), "claude", "no-slash", "p")
	if !errors.Is(err, errValidation) {
		t.Fatalf("bad route key err = %v, want errValidation", err)
	}
	if called {
		t.Fatal("recordPick called for invalid route key")
	}
}

// TestHarnessLaunchSpawnFailure is §7.8 (unix + windows): a shell that cannot
// start maps to launch_failed, recordPick is not called, no event emitted.
func TestHarnessLaunchSpawnFailure(t *testing.T) {
	svc, rec := newTestServices(t)
	mustListHarnesses(t, svc) // seed
	t.Setenv("SHELL", "/nonexistent/shell-xyz")
	called := false
	svc.recordPick = func(_ context.Context, _, _ string) error { called = true; return nil }

	_, err := svc.Harnesses().Launch(context.Background(), "claude", "claude/opus-5@high", "p")
	if err == nil || toErrorDTO(err).Code != "launch_failed" {
		t.Fatalf("err = %v (dto=%+v), want launch_failed", err, toErrorDTO(err))
	}
	if called {
		t.Fatal("recordPick called on spawn failure")
	}
	if got := harnessConfigEvents(rec); got != 1 {
		t.Fatalf("events = %d, want 1 (seed only; spawn failure emits none)", got)
	}
}

// TestHarnessLaunchSpawnSuccess is §7.9 (unix-only): sh -lc "echo ok" writes
// to launch.log and records the pick.
func TestHarnessLaunchSpawnSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix special-case")
	}
	svc, _ := newTestServices(t)
	h := svc.Harnesses()
	if err := h.Save(context.Background(), HarnessInfo{Slug: "echo_harness", Name: "Echo", Command: "echo ok"}); err != nil {
		t.Fatalf("Save echo harness: %v", err)
	}
	t.Setenv("SHELL", "/bin/sh")
	calls := []string{}
	svc.recordPick = func(_ context.Context, profile, route string) error {
		calls = append(calls, profile+"|"+route)
		return nil
	}

	res, err := h.Launch(context.Background(), "echo_harness", "p/abc@low", "profileA")
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if res.Copied || res.Command != "echo ok" {
		t.Fatalf("result = %+v", res)
	}
	if len(calls) != 1 || calls[0] != "profileA|p/abc@low" {
		t.Fatalf("recordPick calls = %v, want [profileA|p/abc@low]", calls)
	}

	// launch.log eventually contains "ok" (spawn is detached/async).
	logPath := filepath.Join(svc.paths.StateDir, "launch.log")
	var contents string
	for i := 0; i < 200; i++ {
		data, err := os.ReadFile(logPath)
		if err == nil {
			contents = string(data)
			if strings.Contains(contents, "ok") {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(contents, "ok") {
		t.Fatalf("launch.log did not contain ok: %q", contents)
	}
}

// TestHarnessBuildCommandUnknownSlug ensures Launch also surfaces not_found
// for a harness that does not exist.
func TestHarnessBuildCommandUnknownSlug(t *testing.T) {
	svc, _ := newTestServices(t)
	mustListHarnesses(t, svc)
	_, err := svc.Harnesses().BuildCommand("nope", "m", "low")
	if !errors.Is(err, errNotFound) {
		t.Fatalf("err = %v, want errNotFound", err)
	}
}

// TestHarnessConfigRoundTrip ensures a seeded config reloads without error
// (integration sanity through config.Load).
func TestHarnessConfigRoundTrip(t *testing.T) {
	svc, _ := newTestServices(t)
	mustListHarnesses(t, svc)
	data := harnessCfgFile(t, svc)
	tmp := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(tmp, []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := config.LoadFile(tmp); err != nil {
		t.Fatalf("seeded config fails to reload: %v", err)
	}
}
