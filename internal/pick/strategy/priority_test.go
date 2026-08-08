package strategy

import (
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

func TestPriorityPick(t *testing.T) {
	t.Run("case 1: first listed provider wins", func(t *testing.T) {
		a := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		b := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		st := &State{ProviderPriority: []string{"codex", "claude"}}
		got, _, err := (Priority{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if got.Route.Provider != "codex" {
			t.Errorf("Pick() provider = %q, want codex", got.Route.Provider)
		}
	})

	t.Run("case 2: priority beats score", func(t *testing.T) {
		a := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		b := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		st := &State{ProviderPriority: []string{"claude", "codex"}}
		got, _, err := (Priority{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if got.Route.Provider != "claude" {
			t.Errorf("Pick() provider = %q, want claude", got.Route.Provider)
		}
	})

	t.Run("case 3: listed provider absent falls to unlisted asc", func(t *testing.T) {
		a := newCandidate("copilot", "gpt-5.6-sol", "max", score(88.4))
		b := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		st := &State{ProviderPriority: []string{"codex"}}
		got, _, err := (Priority{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if got.Route.Provider != "claude" {
			t.Errorf("Pick() provider = %q, want claude", got.Route.Provider)
		}
	})

	t.Run("case 4: max FinalScore within winner", func(t *testing.T) {
		a := newCandidate("codex", "b", "max", score(88.4))
		b := newCandidate("codex", "a", "max", score(75.14))
		st := &State{ProviderPriority: []string{"codex"}}
		got, _, err := (Priority{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/b/max" {
			t.Errorf("Pick() = %+v, want codex/b/max", got)
		}
	})

	t.Run("case 5: tie route key asc within winner", func(t *testing.T) {
		a := newCandidate("codex", "b", "max", score(80))
		b := newCandidate("codex", "a", "max", score(80))
		st := &State{ProviderPriority: []string{"codex"}}
		got, _, err := (Priority{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/a/max" {
			t.Errorf("Pick() = %+v, want codex/a/max", got)
		}
	})

	t.Run("case 6: empty priority sorts provider ID asc", func(t *testing.T) {
		a := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		b := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		c := newCandidate("copilot", "gpt-5.6-sol", "high", score(79.2))
		st := &State{}
		got, _, err := (Priority{}).Pick([]pick.Candidate{a, b, c}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if got.Route.Provider != "claude" {
			t.Errorf("Pick() provider = %q, want claude", got.Route.Provider)
		}
	})

	t.Run("case 7: excluded is the other candidates", func(t *testing.T) {
		a := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		b := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		st := &State{ProviderPriority: []string{"codex", "claude"}}
		_, excluded, err := (Priority{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if len(excluded) != 1 || RouteKey(excluded[0]) != "claude/claude-opus-4-8-20260115/max" {
			t.Errorf("excluded = %+v, want the claude candidate", excluded)
		}
	})

	t.Run("case 8: empty slice", func(t *testing.T) {
		_, _, err := (Priority{}).Pick(nil, &State{})
		if !errors.Is(err, ErrNoCandidates) {
			t.Errorf("Pick() error = %v, want ErrNoCandidates", err)
		}
	})

	t.Run("Name()", func(t *testing.T) {
		if (Priority{}).Name() != pick.StrategyPriority {
			t.Errorf("Name() = %v, want StrategyPriority", (Priority{}).Name())
		}
	})
}
