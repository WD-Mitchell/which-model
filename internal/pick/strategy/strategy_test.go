package strategy

import (
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/shopspring/decimal"
)

func TestRouteKey(t *testing.T) {
	t.Run("case 1", func(t *testing.T) {
		c := pick.Candidate{Route: routing.Route{Provider: "claude", ModelID: "claude-opus-4-8-20260115", Reasoning: "max"}}
		if got := RouteKey(c); got != "claude/claude-opus-4-8-20260115/max" {
			t.Errorf("RouteKey() = %q, want claude/claude-opus-4-8-20260115/max", got)
		}
	})
	t.Run("case 2: empty reasoning", func(t *testing.T) {
		c := pick.Candidate{Route: routing.Route{Provider: "p", ModelID: "m", Reasoning: ""}}
		if got := RouteKey(c); got != "p/m/" {
			t.Errorf("RouteKey() = %q, want p/m/", got)
		}
	})
}

func TestRouteKeyFromRoute(t *testing.T) {
	r := routing.Route{Provider: "codex", ModelID: "gpt-5.6-sol", Reasoning: "high"}
	if got := RouteKeyFromRoute(r); got != "codex/gpt-5.6-sol/high" {
		t.Errorf("RouteKeyFromRoute() = %q, want codex/gpt-5.6-sol/high", got)
	}
}

func TestConfigResolvedCostMaxScoreDrop(t *testing.T) {
	t.Run("case 4: zero default", func(t *testing.T) {
		got := Config{}.ResolvedCostMaxScoreDrop()
		if !got.Equal(decimal.NewFromInt(5)) {
			t.Errorf("ResolvedCostMaxScoreDrop() = %v, want 5", got)
		}
	})
	t.Run("case 5: explicit override", func(t *testing.T) {
		got := Config{CostMaxScoreDrop: 3}.ResolvedCostMaxScoreDrop()
		if !got.Equal(decimal.NewFromInt(3)) {
			t.Errorf("ResolvedCostMaxScoreDrop() = %v, want 3", got)
		}
	})
}

func TestPriorityOrder(t *testing.T) {
	t.Run("case 6: descending priority", func(t *testing.T) {
		got := PriorityOrder(map[string]int{"claude": 10, "codex": 5})
		want := []string{"claude", "codex"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("PriorityOrder() = %v, want %v", got, want)
		}
	})
	t.Run("case 7: tie ascending key", func(t *testing.T) {
		got := PriorityOrder(map[string]int{"a": 1, "b": 1})
		want := []string{"a", "b"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("PriorityOrder() = %v, want %v", got, want)
		}
	})
	t.Run("case 8: nil input", func(t *testing.T) {
		got := PriorityOrder(nil)
		if len(got) != 0 {
			t.Errorf("PriorityOrder(nil) = %v, want empty", got)
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	t.Run("case 9", func(t *testing.T) {
		if got := ErrNoCandidates.Error(); got != "no candidates to pick from" {
			t.Errorf("ErrNoCandidates.Error() = %q", got)
		}
	})
	t.Run("case 10", func(t *testing.T) {
		if got := ErrSeedRequired.Error(); got != "weighted-random requires --seed for reproducibility" {
			t.Errorf("ErrSeedRequired.Error() = %q", got)
		}
	})
	t.Run("case 11", func(t *testing.T) {
		err := &ErrLeastUsedRequiresUsage{Reason: "flag"}
		want := "least-used requires usage data; usage is disabled by --no-usage"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
	t.Run("case 12: errors.As", func(t *testing.T) {
		var err error = &ErrLeastUsedRequiresUsage{Reason: "config"}
		var target *ErrLeastUsedRequiresUsage
		if !errors.As(err, &target) {
			t.Fatal("errors.As() = false, want true")
		}
		if target.Reason != "config" {
			t.Errorf("Reason = %q, want config", target.Reason)
		}
	})
}
