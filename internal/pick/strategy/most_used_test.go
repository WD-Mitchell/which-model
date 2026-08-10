package strategy

import (
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

func TestMostUsed(t *testing.T) {
	t.Run("highest provider pressure wins", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(95))
		b := newCandidate("codex", "b", "max", score(80))
		st := &State{UsageEnabled: true, PressureByProvider: map[string]float64{"claude": 30, "codex": 80}}
		got, excluded, err := (MostUsed{}).Pick([]pick.Candidate{a, b}, st)
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
		st := &State{UsageEnabled: true, PressureByProvider: map[string]float64{"claude": 50, "codex": 50}}
		got, _, err := (MostUsed{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatal(err)
		}
		if RouteKey(got) != RouteKey(b) {
			t.Fatalf("got = %s, want %s", RouteKey(got), RouteKey(b))
		}
	})

	t.Run("usage disabled is refused", func(t *testing.T) {
		_, _, err := (MostUsed{}).Pick([]pick.Candidate{newCandidate("claude", "a", "max", score(90))}, &State{UsageDisabledReason: "config"})
		var target *ErrMostUsedRequiresUsage
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrMostUsedRequiresUsage", err)
		}
	})

	t.Run("missing pressure is an error", func(t *testing.T) {
		_, _, err := (MostUsed{}).Pick([]pick.Candidate{newCandidate("claude", "a", "max", score(90))}, &State{UsageEnabled: true})
		var target *ErrMissingPressure
		if !errors.As(err, &target) || target.Provider != "claude" {
			t.Fatalf("err = %#v, want missing pressure for claude", err)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, _, err := (MostUsed{}).Pick(nil, &State{UsageEnabled: true})
		if !errors.Is(err, ErrNoCandidates) {
			t.Fatalf("err = %v, want ErrNoCandidates", err)
		}
	})

	if (MostUsed{}).Name() != pick.StrategyMostUsed {
		t.Fatalf("Name() = %q", (MostUsed{}).Name())
	}
}
