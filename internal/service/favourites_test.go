package service

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

func ctxFav() context.Context { return context.Background() }

// mustList returns List' output, fataling on error.
func mustList(t *testing.T, f *FavouriteService) []Favourite {
	t.Helper()
	got, err := f.List(ctxFav())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return got
}

// assertFavEvent asserts recorder has exactly one config:changed with payload
// {"section":"favourites"} appended since the given baseline.
func assertFavEvent(t *testing.T, rec *emitRecorder, baseline int) {
	t.Helper()
	ev := rec.Events()
	if len(ev) != baseline+1 {
		t.Fatalf("events = %d, want %d (one new)", len(ev), baseline+1)
	}
	last := ev[len(ev)-1]
	if last.Event != EventConfigChanged {
		t.Errorf("event = %q, want %q", last.Event, EventConfigChanged)
	}
	payload, ok := last.Payload.(map[string]string)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]string", last.Payload)
	}
	if payload["section"] != "favourites" {
		t.Errorf("payload section = %q, want favourites", payload["section"])
	}
}

// TestFavourites_ListOrderAndResolution is CONTRACTS §5.1: stored order is
// preserved (not re-sorted); ModelName resolves from the routes table; the
// route label is "<provider> · <reasoning>".
func TestFavourites_ListOrderAndResolution(t *testing.T) {
	// Pins deliberately non-alphabetical (codex before claude).
	svc, rec := newTestServices(t, WithConfigTOML(`
[favourites]
pins = ["codex/gpt-5.6@high", "claude/claude-opus-5@max"]

[providers.claude]
enabled = true

[providers.codex]
enabled = true
`))
	f := svc.Favourites()

	got := mustList(t, f)
	if len(got) != 2 {
		t.Fatalf("List = %d entries, want 2", len(got))
	}
	// Stored order, not alphabetical (alphabetically claude < codex).
	if got[0].RouteKey != "codex/gpt-5.6@high" {
		t.Errorf("entry[0] RouteKey = %q, want codex/gpt-5.6@high", got[0].RouteKey)
	}
	if got[1].RouteKey != "claude/claude-opus-5@max" {
		t.Errorf("entry[1] RouteKey = %q, want claude/claude-opus-5@max", got[1].RouteKey)
	}
	// ModelName equals the fixture Route.Model for each.
	if got[0].ModelName != "GPT-5.6 Sol" {
		t.Errorf("entry[0] ModelName = %q, want GPT-5.6 Sol", got[0].ModelName)
	}
	if got[1].ModelName != "Claude Opus 5" {
		t.Errorf("entry[1] ModelName = %q, want Claude Opus 5", got[1].ModelName)
	}
	// Route label style "<provider> · <reasoning>" with in-range pins.
	if got[0].RouteLabel != "codex \u00b7 high" {
		t.Errorf("entry[0] RouteLabel = %q, want codex · high", got[0].RouteLabel)
	}
	if got[1].RouteLabel != "claude \u00b7 max" {
		t.Errorf("entry[1] RouteLabel = %q, want claude · max", got[1].RouteLabel)
	}
	if !got[0].InRange || !got[1].InRange {
		t.Errorf("InRange = %v/%v, want true/true", got[0].InRange, got[1].InRange)
	}
	if len(rec.Events()) != 0 {
		t.Errorf("List emitted %d events, want 0", len(rec.Events()))
	}
}

// TestFavourites_ModelNameFallback is CONTRACTS §5.7-adjacent: a pin whose
// exact reasoning is missing falls back to any route sharing the model_id.
func TestFavourites_ModelNameFallback(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[favourites]
pins = ["codex/gpt-5.6@low"]

[providers.codex]
enabled = true
`))
	f := svc.Favourites()

	got := mustList(t, f)
	if len(got) != 1 {
		t.Fatalf("List = %d entries, want 1", len(got))
	}
	// "low" is not a fixture route for gpt-5.6; fall back to the same model_id.
	if got[0].ModelName != "GPT-5.6 Sol" {
		t.Errorf("ModelName = %q, want GPT-5.6 Sol (model_id fallback)", got[0].ModelName)
	}
	// No exact route in the availability set → out of range.
	if got[0].InRange {
		t.Error("InRange = true, want false (no exact route for @low)")
	}
	if got[0].RouteLabel != "no provider \u00b7 low" {
		t.Errorf("RouteLabel = %q, want no provider · low", got[0].RouteLabel)
	}
}

// TestFavourites_InRangeTransitions is CONTRACTS §5.2: a pinned in-range route
// turns out of range when its provider is disabled or its route is listed
// under [routes.disabled], and the label drops the provider.
func TestFavourites_InRangeTransitions(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[favourites]
pins = ["claude/claude-opus-5@max"]

[providers.claude]
enabled = true
`))
	f := svc.Favourites()

	got := mustList(t, f)
	if len(got) != 1 || !got[0].InRange {
		t.Fatalf("initial InRange = %v, want true", got[0].InRange)
	}
	if got[0].RouteLabel != "claude \u00b7 max" {
		t.Errorf("initial RouteLabel = %q, want claude · max", got[0].RouteLabel)
	}

	// (A) Provider disabled → out of range.
	svc.cfg.Providers["claude"] = config.ProviderConfig{Enabled: false}
	got = mustList(t, f)
	if got[0].InRange {
		t.Error("InRange true after provider disabled, want false")
	}
	if !strings.HasPrefix(got[0].RouteLabel, "no provider \u00b7 ") {
		t.Errorf("RouteLabel = %q, want no provider · …", got[0].RouteLabel)
	}

	// (B) Re-enable provider, then disable the exact route.
	svc.cfg.Providers["claude"] = config.ProviderConfig{Enabled: true}
	if err := svc.cfg.SetRoutesDisabled(config.RoutesDisabledTOML{
		"claude": {"claude-opus-5@max"},
	}); err != nil {
		t.Fatalf("SetRoutesDisabled: %v", err)
	}
	got = mustList(t, f)
	if got[0].InRange {
		t.Error("InRange true after route disabled, want false")
	}
	if !strings.HasPrefix(got[0].RouteLabel, "no provider \u00b7 ") {
		t.Errorf("RouteLabel = %q, want no provider · …", got[0].RouteLabel)
	}
}

// TestFavourites_PinRoundTrip is CONTRACTS §5.3: Pin persists the key under
// [favourites].pins on disk and emits exactly one config:changed.
func TestFavourites_PinRoundTrip(t *testing.T) {
	svc, rec := newTestServices(t)
	f := svc.Favourites()
	key := "claude/claude-opus-5@max"

	if err := f.Pin(ctxFav(), key); err != nil {
		t.Fatalf("Pin: %v", err)
	}
	assertFavEvent(t, rec, 0)

	// The key is persisted under [favourites].pins.
	data, err := os.ReadFile(svc.paths.UserConfigFile)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	if !bytes.Contains(data, []byte("favourites")) {
		t.Errorf("config.toml missing [favourites] section:\n%s", data)
	}
	if !bytes.Contains(data, []byte(key)) {
		t.Errorf("config.toml missing pin %q:\n%s", key, data)
	}

	// List reflects the pinned route (in-range requires provider enabled).
	got := mustList(t, f)
	if len(got) != 1 || got[0].RouteKey != key {
		t.Errorf("List = %v, want one pinned entry", got)
	}
}

// TestFavourites_Idempotent is CONTRACTS §5.4: re-Pinning an already-pinned
// key and Unpinning an absent key succeed with zero events and no file change.
func TestFavourites_Idempotent(t *testing.T) {
	svc, rec := newTestServices(t)
	f := svc.Favourites()
	key := "claude/claude-opus-5@max"

	if err := f.Pin(ctxFav(), key); err != nil {
		t.Fatalf("Pin: %v", err)
	}
	assertFavEvent(t, rec, 0)

	filePath := svc.paths.UserConfigFile
	dataBefore, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	statBefore, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat config.toml: %v", err)
	}

	// Re-Pin the same key: success, no event, no write.
	if err := f.Pin(ctxFav(), key); err != nil {
		t.Fatalf("re-Pin: %v", err)
	}
	if len(rec.Events()) != 1 {
		t.Errorf("events after re-Pin = %d, want 1 (unchanged)", len(rec.Events()))
	}
	assertFileUnchanged(t, filePath, dataBefore, statBefore)

	// Unpin an absent key: success, no event, no write.
	if err := f.Unpin(ctxFav(), "codex/gpt-5.6@high"); err != nil {
		t.Fatalf("Unpin absent: %v", err)
	}
	if len(rec.Events()) != 1 {
		t.Errorf("events after absent Unpin = %d, want 1 (unchanged)", len(rec.Events()))
	}
	assertFileUnchanged(t, filePath, dataBefore, statBefore)
}

func assertFileUnchanged(t *testing.T, path string, data []byte, stat os.FileInfo) {
	t.Helper()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read config.toml: %v", err)
	}
	if !bytes.Equal(after, data) {
		t.Error("config.toml content changed on idempotent no-op")
	}
	statAfter, err := os.Stat(path)
	if err != nil {
		t.Fatalf("re-stat config.toml: %v", err)
	}
	if !statAfter.ModTime().Equal(stat.ModTime()) {
		t.Error("config.toml mtime changed on idempotent no-op")
	}
}

// TestFavourites_GrammarRejected is CONTRACTS §5.5: an ill-formed key maps to
// validation_failed with the §4 message prefix and zero events.
func TestFavourites_GrammarRejected(t *testing.T) {
	svc, rec := newTestServices(t)
	f := svc.Favourites()

	for _, method := range []string{"Pin", "Unpin"} {
		var err error
		if method == "Pin" {
			err = f.Pin(ctxFav(), "not a key")
		} else {
			err = f.Unpin(ctxFav(), "not a key")
		}
		dto := toErrorDTO(err)
		if dto.Code != "validation_failed" {
			t.Errorf("%s: code = %q, want validation_failed", method, dto.Code)
		}
		if !strings.HasPrefix(dto.Message, `favourites: invalid route key "not a key"`) {
			t.Errorf("%s: message = %q, want §4 prefix", method, dto.Message)
		}
		if len(rec.Events()) != 0 {
			t.Errorf("%s: events = %d, want 0", method, len(rec.Events()))
		}
	}
}

// TestFavourites_PinOutOfRangeAccepted is SPEC §2.5: pinning is not
// availability-gated; a well-formed key currently out of range is accepted and
// reported as OutOfRange by List.
func TestFavourites_PinOutOfRangeAccepted(t *testing.T) {
	svc, _ := newTestServices(t)
	f := svc.Favourites()
	// "nonexistent-model" has no route and no provider enabled — still accepted.
	if err := f.Pin(ctxFav(), "codex/nonexistent-model@high"); err != nil {
		t.Fatalf("Pin out-of-range key: %v", err)
	}
	got := mustList(t, f)
	if len(got) != 1 || got[0].RouteKey != "codex/nonexistent-model@high" {
		t.Fatalf("List = %v, want the pinned out-of-range key", got)
	}
	if got[0].InRange {
		t.Error("InRange = true, want false for out-of-range pin")
	}
	if got[0].RouteLabel != "no provider \u00b7 high" {
		t.Errorf("RouteLabel = %q, want no provider · high", got[0].RouteLabel)
	}
}

// TestFavourites_UnknownModel is CONTRACTS §5.7: a pin whose model_id is
// absent from the routes table → ModelName is the model_id verbatim and
// InRange is false.
func TestFavourites_UnknownModel(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[favourites]
pins = ["claude/nonexistent-model@high"]

[providers.claude]
enabled = true
`))
	f := svc.Favourites()

	got := mustList(t, f)
	if len(got) != 1 {
		t.Fatalf("List = %d entries, want 1", len(got))
	}
	if got[0].ModelName != "nonexistent-model" {
		t.Errorf("ModelName = %q, want model_id verbatim", got[0].ModelName)
	}
	if got[0].InRange {
		t.Error("InRange = true, want false (model not in routes table)")
	}
	if got[0].RouteLabel != "no provider \u00b7 high" {
		t.Errorf("RouteLabel = %q, want no provider · high", got[0].RouteLabel)
	}
}

// TestFavourites_CorruptStoredPin is CONTRACTS §5.6: a hand-written corrupt
// pin is surfaced by List (never dropped, never errors) with an out-of-range
// "no provider · default" label.
func TestFavourites_CorruptStoredPin(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[favourites]
pins = ["garbage"]
`))
	f := svc.Favourites()

	got, err := f.List(ctxFav())
	if err != nil {
		t.Fatalf("List must not fail on corrupt pin: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List = %d entries, want 1 (corrupt pin surfaced)", len(got))
	}
	e := got[0]
	if e.RouteKey != "garbage" {
		t.Errorf("RouteKey = %q, want garbage", e.RouteKey)
	}
	if e.ModelName != "garbage" {
		t.Errorf("ModelName = %q, want garbage", e.ModelName)
	}
	if e.RouteLabel != "no provider \u00b7 default" {
		t.Errorf("RouteLabel = %q, want no provider · default", e.RouteLabel)
	}
	if e.InRange {
		t.Error("InRange = true, want false")
	}
	if len(rec.Events()) != 0 {
		t.Errorf("List emitted %d events, want 0", len(rec.Events()))
	}
}

// TestFavourites_FailedWriteLeavesStateAndNoEvent is CONTRACTS §4 discipline:
// a persist forced to fail at AtomicWriteFile leaves config bytes and
// in-memory pins unchanged and the recorder empty.
func TestFavourites_FailedWriteLeavesStateAndNoEvent(t *testing.T) {
	svc, rec := newTestServices(t)
	f := svc.Favourites()
	key := "claude/claude-opus-5@max"

	// First Pin succeeds so there is a baseline to preserve.
	if err := f.Pin(ctxFav(), key); err != nil {
		t.Fatalf("Pin: %v", err)
	}
	assertFavEvent(t, rec, 0)
	filePath := svc.paths.UserConfigFile
	dataBefore, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}

	// Make the config directory unwritable so AtomicWriteFile fails.
	dir := svc.paths.ConfigDir
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := f.Pin(ctxFav(), "codex/gpt-5.6@high"); err == nil {
		t.Fatal("Pin with unwritable config dir: error = nil, want io_error")
	}

	// In-memory pins unchanged (second pin absent).
	got := mustList(t, f)
	if len(got) != 1 || got[0].RouteKey != key {
		t.Errorf("in-memory pins after failed write = %v, want only %q", got, key)
	}
	// Disk unchanged.
	after, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("re-read config.toml: %v", err)
	}
	if !bytes.Equal(after, dataBefore) {
		t.Error("config.toml changed despite failed AtomicWriteFile")
	}
	// No event emitted for the failed write.
	if len(rec.Events()) != 1 {
		t.Errorf("events = %d, want 1 (only the first success)", len(rec.Events()))
	}
}