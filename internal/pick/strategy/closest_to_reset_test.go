package strategy

import (
	"errors"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

func TestClosestToReset(t *testing.T) {
	base := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	t.Run("earliest provider reset wins", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(95))
		b := newCandidate("codex", "b", "max", score(80))
		st := &State{UsageEnabled: true, ResetAtByProvider: map[string]time.Time{"claude": base.Add(2 * time.Hour), "codex": base.Add(time.Hour)}}
		got, excluded, err := (ClosestToReset{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatal(err)
		}
		if RouteKey(got) != RouteKey(b) || len(excluded) != 1 || RouteKey(excluded[0]) != RouteKey(a) {
			t.Fatalf("got = %s, excluded = %#v", RouteKey(got), excluded)
		}
	})

	t.Run("ties use score then route key", func(t *testing.T) {
		a := newCandidate("claude", "z", "max", score(80))
		b := newCandidate("codex", "b", "max", score(90))
		st := &State{UsageEnabled: true, ResetAtByProvider: map[string]time.Time{"claude": base, "codex": base}}
		got, _, err := (ClosestToReset{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatal(err)
		}
		if RouteKey(got) != RouteKey(b) {
			t.Fatalf("got = %s, want %s", RouteKey(got), RouteKey(b))
		}
	})

	t.Run("usage disabled is refused", func(t *testing.T) {
		_, _, err := (ClosestToReset{}).Pick([]pick.Candidate{newCandidate("claude", "a", "max", score(90))}, &State{UsageDisabledReason: "config"})
		var target *ErrClosestToResetRequiresUsage
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrClosestToResetRequiresUsage", err)
		}
	})

	t.Run("missing reset is an error", func(t *testing.T) {
		_, _, err := (ClosestToReset{}).Pick([]pick.Candidate{newCandidate("claude", "a", "max", score(90))}, &State{UsageEnabled: true})
		var target *ErrMissingReset
		if !errors.As(err, &target) || target.Provider != "claude" {
			t.Fatalf("err = %#v, want missing reset for claude", err)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, _, err := (ClosestToReset{}).Pick(nil, &State{UsageEnabled: true})
		if !errors.Is(err, ErrNoCandidates) {
			t.Fatalf("err = %v, want ErrNoCandidates", err)
		}
	})

	if (ClosestToReset{}).Name() != pick.StrategyClosestToReset {
		t.Fatalf("Name() = %q", (ClosestToReset{}).Name())
	}
}
