//go:build !nousage

package copilot

import (
	"encoding/json"
	"testing"
	"time"
)

// Issue #49: resetTime must normalize every branch to UTC, matching the
// codex/claude adapters — a source offset (+02:00) or the host-local epoch
// zone must not leak into the output.
func TestResetTimeUTCNormalized(t *testing.T) {
	offset := `"2026-09-01T12:00:00+02:00"`
	got := resetTime(json.RawMessage(offset))
	if got == nil {
		t.Fatal("resetTime(offset) = nil")
	}
	want := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("resetTime(offset) = %v in %v, want %v in UTC", got, got.Location(), want)
	}

	epochSec := json.RawMessage("1788000000")
	got = resetTime(epochSec)
	if got == nil {
		t.Fatal("resetTime(epoch) = nil")
	}
	if got.Location() != time.UTC {
		t.Errorf("resetTime(epoch) zone = %v, want UTC", got.Location())
	}
}
