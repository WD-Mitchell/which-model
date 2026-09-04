package strategy

import (
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

func TestRoundRobinPick(t *testing.T) {
	t.Run("case 1: first pick is sorted[0], cursor advances", func(t *testing.T) {
		dir := t.TempDir()
		a := newCandidate("codex", "b", "max", score(80))
		b := newCandidate("claude", "a", "max", score(80))
		st := &State{Profile: "balanced_implementation", DataDir: dir}
		got, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a/max" {
			t.Errorf("Pick() = %+v, want sorted[0] claude/a/max", got)
		}
		sorted := sortByRouteKey([]pick.Candidate{a, b})
		keys := []string{RouteKey(sorted[0]), RouteKey(sorted[1])}
		key := scopeKey(st.Profile, keys)
		idx, err := loadCursor(dir, key)
		if err != nil || idx != 1 {
			t.Errorf("loadCursor() = (%d, %v), want (1, nil)", idx, err)
		}
	})

	t.Run("case 2-3: rotation persists and wraps", func(t *testing.T) {
		dir := t.TempDir()
		a := newCandidate("codex", "b", "max", score(80))
		b := newCandidate("claude", "a", "max", score(80))
		st := &State{Profile: "balanced_implementation", DataDir: dir}

		got1, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() 1 error = %v", err)
		}
		got2, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() 2 error = %v", err)
		}
		if RouteKey(got1) == RouteKey(got2) {
			t.Errorf("Pick() 2 = %+v, want different from Pick() 1", got2)
		}
		got3, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() 3 error = %v", err)
		}
		if RouteKey(got3) != RouteKey(got1) {
			t.Errorf("Pick() 3 = %+v, want wrap back to %+v", got3, got1)
		}
	})

	t.Run("case 4: dry run never advances", func(t *testing.T) {
		dir := t.TempDir()
		a := newCandidate("codex", "c", "max", score(80))
		b := newCandidate("claude", "a", "max", score(80))
		c := newCandidate("copilot", "b", "max", score(80))
		st := &State{Profile: "p", DataDir: dir, DryRun: true}
		sorted := sortByRouteKey([]pick.Candidate{a, b, c})
		for i := 0; i < 4; i++ {
			got, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b, c}, st)
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}
			if RouteKey(got) != RouteKey(sorted[0]) {
				t.Errorf("Pick() iteration %d = %+v, want sorted[0]", i, got)
			}
		}
		if _, err := os.Stat(stateFilePath(dir)); err == nil {
			data, _ := os.ReadFile(stateFilePath(dir))
			if len(data) > 0 {
				raw, rerr := readRawState(t, dir)
				if rerr == nil && len(raw) != 0 {
					t.Errorf("state file has entries %v, want none written under DryRun", raw)
				}
			}
		}
	})

	t.Run("case 5: corrupt state file starts from index 0", func(t *testing.T) {
		dir := t.TempDir()
		if err := writeRawState(t, dir, "not json"); err != nil {
			t.Fatalf("setup error = %v", err)
		}
		a := newCandidate("codex", "b", "max", score(80))
		b := newCandidate("claude", "a", "max", score(80))
		st := &State{Profile: "p", DataDir: dir}
		got, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		sorted := sortByRouteKey([]pick.Candidate{a, b})
		if RouteKey(got) != RouteKey(sorted[0]) {
			t.Errorf("Pick() = %+v, want sorted[0]", got)
		}
	})

	t.Run("case 6: concurrent picks each advance under the lock", func(t *testing.T) {
		dir := t.TempDir()
		a := newCandidate("codex", "b", "max", score(80))
		b := newCandidate("claude", "a", "max", score(80))
		st := &State{Profile: "p", DataDir: dir}

		ready := make(chan struct{})
		results := make(chan pick.Candidate, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		for i := 0; i < 2; i++ {
			go func() {
				defer wg.Done()
				<-ready
				got, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b}, st)
				if err != nil {
					t.Errorf("goroutine Pick() error = %v", err)
					return
				}
				results <- got
			}()
		}
		close(ready)
		wg.Wait()
		close(results)

		var got []pick.Candidate
		for c := range results {
			got = append(got, c)
		}
		if len(got) != 2 {
			t.Fatalf("got %d results, want 2", len(got))
		}
		if RouteKey(got[0]) == RouteKey(got[1]) {
			t.Errorf("both goroutine picks were %+v, want them to differ", got[0])
		}
		sorted := sortByRouteKey([]pick.Candidate{a, b})
		seen := map[string]bool{RouteKey(got[0]): true, RouteKey(got[1]): true}
		if !seen[RouteKey(sorted[0])] || !seen[RouteKey(sorted[1])] {
			t.Errorf("goroutine picks = %v, want union to cover both candidates", seen)
		}

		got3, _, err := (RoundRobin{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() 3 error = %v", err)
		}
		if RouteKey(got3) != RouteKey(sorted[0]) {
			t.Errorf("Pick() 3 = %+v, want sorted[0] (both advances landed under the lock)", got3)
		}
	})

	t.Run("case 7: empty slice", func(t *testing.T) {
		_, _, err := (RoundRobin{}).Pick(nil, &State{DataDir: t.TempDir()})
		if !errors.Is(err, ErrNoCandidates) {
			t.Errorf("Pick() error = %v, want ErrNoCandidates", err)
		}
	})

	t.Run("Name()", func(t *testing.T) {
		if (RoundRobin{}).Name() != pick.StrategyRoundRobin {
			t.Errorf("Name() = %v, want StrategyRoundRobin", (RoundRobin{}).Name())
		}
	})
}

func TestRoundRobinInvalidStateRecovery(t *testing.T) {
	candidates := []pick.Candidate{newCandidate("claude", "a", "max", score(80)), newCandidate("codex", "b", "max", score(80))}
	key := scopeKey("p", []string{RouteKey(candidates[0]), RouteKey(candidates[1])})
	for _, raw := range []string{"null", "[]", "not json", `{"` + key + `":{"index":-1}}`, `{"` + key + `":{"index":` + strconv.Itoa(int(^uint(0)>>1)) + `}}`, `{"` + key + `":{"index":1},"bad":{"index":"broken"}}`} {
		t.Run(raw, func(t *testing.T) {
			dir := t.TempDir()
			if err := writeRawState(t, dir, raw); err != nil {
				t.Fatal(err)
			}
			st := &State{Profile: "p", DataDir: dir, DryRun: true}
			got, _, err := (RoundRobin{}).Pick(candidates, st)
			if err != nil || RouteKey(got) != RouteKey(candidates[0]) {
				t.Fatalf("dry run: %+v, %v", got, err)
			}
			bytes, err := os.ReadFile(stateFilePath(dir))
			if err != nil || string(bytes) != raw {
				t.Fatalf("dry run changed file: %s %v", bytes, err)
			}
			st.DryRun = false
			for i := 0; i < 2; i++ {
				got, _, err := (RoundRobin{}).Pick(candidates, st)
				if err != nil || RouteKey(got) != RouteKey(candidates[i]) {
					t.Fatalf("pick %d: %+v, %v", i, got, err)
				}
			}
		})
	}
}
