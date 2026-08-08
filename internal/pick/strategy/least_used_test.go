package strategy

import (
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

func TestLeastUsedPick(t *testing.T) {
	t.Run("case 1: min pressure wins over score", func(t *testing.T) {
		a := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		b := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		st := &State{UsageEnabled: true, PressureByProvider: map[string]float64{"claude": 30, "codex": 80}}
		got, _, err := (LeastUsed{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if got.Route.Provider != "claude" {
			t.Errorf("Pick() provider = %q, want claude", got.Route.Provider)
		}
	})

	t.Run("case 2: pressure tie broken by FinalScore", func(t *testing.T) {
		a := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		b := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		st := &State{UsageEnabled: true, PressureByProvider: map[string]float64{"claude": 80, "codex": 80}}
		got, _, err := (LeastUsed{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if got.Route.Provider != "codex" {
			t.Errorf("Pick() provider = %q, want codex", got.Route.Provider)
		}
	})

	t.Run("case 3: full tie route key asc", func(t *testing.T) {
		a := newCandidate("codex", "b", "max", score(80))
		b := newCandidate("codex", "a", "max", score(80))
		st := &State{UsageEnabled: true, PressureByProvider: map[string]float64{"codex": 50}}
		got, _, err := (LeastUsed{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/a/max" {
			t.Errorf("Pick() = %+v, want codex/a/max", got)
		}
	})

	refusalTests := []struct {
		name   string
		reason string
		want   string
	}{
		{"case 4: flag", "flag", "least-used requires usage data; usage is disabled by --no-usage"},
		{"case 5: config", "config", "least-used requires usage data; usage is disabled by [usage] enabled = false"},
		{"case 6: compiled_out", "compiled_out", "least-used requires usage data; usage is disabled by nousage build"},
		{"case 7: no_providers_enabled", "no_providers_enabled", "least-used requires usage data; usage is disabled by no providers enabled"},
	}
	for _, tc := range refusalTests {
		t.Run(tc.name, func(t *testing.T) {
			a := newCandidate("claude", "x", "max", score(80))
			b := newCandidate("codex", "y", "max", score(80))
			st := &State{UsageEnabled: false, UsageDisabledReason: tc.reason}
			_, _, err := (LeastUsed{}).Pick([]pick.Candidate{a, b}, st)
			var refusal *ErrLeastUsedRequiresUsage
			if !errors.As(err, &refusal) {
				t.Fatalf("Pick() error = %v, want *ErrLeastUsedRequiresUsage", err)
			}
			if refusal.Reason != tc.reason {
				t.Errorf("Reason = %q, want %q", refusal.Reason, tc.reason)
			}
			if got := refusal.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("case 8: missing pressure", func(t *testing.T) {
		a := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		st := &State{UsageEnabled: true, PressureByProvider: map[string]float64{}}
		_, _, err := (LeastUsed{}).Pick([]pick.Candidate{a}, st)
		var missing *ErrMissingPressure
		if !errors.As(err, &missing) {
			t.Fatalf("Pick() error = %v, want *ErrMissingPressure", err)
		}
		if missing.Provider != "codex" {
			t.Errorf("Provider = %q, want codex", missing.Provider)
		}
		want := `no usage pressure data for provider "codex"`
		if got := missing.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("case 9: empty slice", func(t *testing.T) {
		_, _, err := (LeastUsed{}).Pick(nil, &State{UsageEnabled: true})
		if !errors.Is(err, ErrNoCandidates) {
			t.Errorf("Pick() error = %v, want ErrNoCandidates", err)
		}
	})

	t.Run("Name()", func(t *testing.T) {
		if (LeastUsed{}).Name() != pick.StrategyLeastUsed {
			t.Errorf("Name() = %v, want StrategyLeastUsed", (LeastUsed{}).Name())
		}
	})
}
