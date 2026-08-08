//go:build !nousage

package fetch

import (
	"context"
	"reflect"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// T7: disabled short-circuit and sorted results (SPEC §2, §8, D7).

func TestDisabledShortCircuit(t *testing.T) {
	// case 1: fake-never requested but NOT enabled → skipped entirely;
	// its Fetch panics, so no panic proves the gate short-circuits before
	// anything (cache/credential/fetch)
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-ok": true}
	snaps, _, err := FetchAll(context.Background(), []string{"fake-ok", "fake-never"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("short-circuit: got %d snapshots, err=%v; want 1, nil", len(snaps), err)
	}
	if snaps[0].Provider != "fake-ok" {
		t.Fatalf("provider = %q, want fake-ok", snaps[0].Provider)
	}
}

func ids(snaps []usage.Snapshot) []string {
	out := make([]string, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, s.Provider)
	}
	return out
}

func TestSortedResults(t *testing.T) {
	ctx := context.Background()

	// case 2: unsorted request → output sorted by Provider ID ascending
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-slow2": true, "fake-ok": true, "fake-slow": true}
	snaps, _, err := FetchAll(ctx, []string{"fake-slow2", "fake-ok", "fake-slow"}, opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"fake-ok", "fake-slow", "fake-slow2"}
	if got := ids(snaps); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}

	// case 3: unknown providers participate in the sort
	opts = newOptions(t)
	opts.Enabled = map[string]bool{"zz-unknown": true, "fake-ok": true}
	snaps, _, err = FetchAll(ctx, []string{"zz-unknown", "fake-ok"}, opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want = []string{"fake-ok", "zz-unknown"}
	if got := ids(snaps); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	if snaps[1].Failure == nil || snaps[1].Failure.Code != "provider_status" {
		t.Fatalf("zz-unknown failure = %+v, want provider_status", snaps[1].Failure)
	}

	// case 4: identical ID sequence across repeated calls under
	// MaxParallel 1 (determinism under serialization)
	opts = newOptions(t)
	opts.MaxParallel = 1
	opts.Enabled = map[string]bool{"fake-slow2": true, "fake-ok": true, "fake-slow": true}
	first, _, err := FetchAll(ctx, []string{"fake-slow2", "fake-ok", "fake-slow"}, opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	second, _, err := FetchAll(ctx, []string{"fake-slow2", "fake-ok", "fake-slow"}, opts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(ids(first), ids(second)) {
		t.Fatalf("non-deterministic ordering: %v vs %v", ids(first), ids(second))
	}

	// case 5: fake-ok + fake-fail + fake-slow under MaxParallel 1 → all
	// present, sorted, err == nil
	opts = newOptions(t)
	opts.MaxParallel = 1
	opts.Enabled = map[string]bool{"fake-ok": true, "fake-fail": true, "fake-slow": true}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok", "fake-fail", "fake-slow"}, opts)
	if err != nil || len(snaps) != 3 {
		t.Fatalf("got %d snapshots, err=%v; want 3, nil", len(snaps), err)
	}
	want = []string{"fake-fail", "fake-ok", "fake-slow"}
	if got := ids(snaps); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}
