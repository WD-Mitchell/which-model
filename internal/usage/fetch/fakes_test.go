//go:build !nousage

package fetch

import (
	"context"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Fake provider descriptors. IDs are unique to this feature (fake-*) — the
// global registry (F11) is shared with F11's tests. Registered once in
// TestMain so registration order is explicit and no duplicate-ID panic can
// occur at test start.

func TestMain(m *testing.M) {
	registerFakes()
	os.Exit(m.Run())
}

func registerFakes() {
	usage.Register(usage.Descriptor{
		ID:          "fake-ok",
		DisplayName: "Fake OK",
		Kind:        usage.KindSubscription,
		CacheTTL:    300 * time.Second,
		Timeout:     5 * time.Second,
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			recordCall("fake-ok")
			return usage.Snapshot{
				Provider: "fake-ok",
				Account:  "acct",
				Plan:     "pro",
				Windows: []usage.Window{{
					ID:    "w1",
					Label: "Window 1",
					Unit:  usage.UnitPercent,
				}},
			}, nil
		},
	})

	usage.Register(usage.Descriptor{
		ID:          "fake-env",
		DisplayName: "Fake Env",
		Kind:        usage.KindSubscription,
		CacheTTL:    0,
		Auth:        []usage.AuthSource{{Kind: usage.AuthEnvVar, EnvVar: "WHICH_MODEL_TEST_TOKEN"}},
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			recordCall("fake-env")
			return usage.Snapshot{Provider: "fake-env"}, nil
		},
	})

	usage.Register(usage.Descriptor{
		ID:          "fake-fail",
		DisplayName: "Fake Fail",
		Kind:        usage.KindSubscription,
		CacheTTL:    0,
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			return usage.Snapshot{}, &usage.FailureError{Failure: usage.Failure{Code: "provider_status", Message: "boom"}}
		},
	})

	usage.Register(usage.Descriptor{
		ID:          "fake-slow",
		DisplayName: "Fake Slow",
		Kind:        usage.KindSubscription,
		CacheTTL:    0,
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			slowEnter()
			defer slowExit()
			time.Sleep(300 * time.Millisecond)
			return usage.Snapshot{Provider: "fake-slow"}, nil
		},
	})

	usage.Register(usage.Descriptor{
		ID:          "fake-block",
		DisplayName: "Fake Block",
		Kind:        usage.KindSubscription,
		CacheTTL:    0,
		Timeout:     0,
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			<-ctx.Done()
			return usage.Snapshot{}, ctx.Err()
		},
	})

	usage.Register(usage.Descriptor{
		ID:          "fake-never",
		DisplayName: "Fake Never",
		Kind:        usage.KindSubscription,
		CacheTTL:    300 * time.Second,
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			panic("fake-never Fetch must never be invoked")
		},
	})

	usage.Register(usage.Descriptor{
		ID:          "fake-nocache",
		DisplayName: "Fake No Cache",
		Kind:        usage.KindSubscription,
		CacheTTL:    0,
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			recordCall("fake-nocache")
			return usage.Snapshot{Provider: "fake-nocache"}, nil
		},
	})

	usage.Register(usage.Descriptor{
		ID:          "fake-leak",
		DisplayName: "Fake Leak",
		Kind:        usage.KindSubscription,
		CacheTTL:    0,
		Auth:        []usage.AuthSource{{Kind: usage.AuthEnvVar, EnvVar: "WHICH_MODEL_TEST_TOKEN"}},
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			return usage.Snapshot{}, &usage.FailureError{Failure: usage.Failure{Code: "provider_status", Message: "provider said " + cred.Token}}
		},
	})

	// Slow twins for concurrency-cap tests (same Fetch as fake-slow).
	for _, id := range []string{"fake-slow2", "fake-slow3", "fake-slow4"} {
		usage.Register(usage.Descriptor{
			ID:          id,
			DisplayName: "Fake Slow Twin",
			Kind:        usage.KindSubscription,
			CacheTTL:    0,
			Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
				slowEnter()
				defer slowExit()
				time.Sleep(300 * time.Millisecond)
				return usage.Snapshot{Provider: id}, nil
			},
		})
	}

	// Second failing cache writer: needed by T5's warning-ordering case
	// (only fake-ok and fake-never have CacheTTL > 0, and fake-never's
	// Fetch panics; fake-warn2 completes a fetch and fails its write).
	usage.Register(usage.Descriptor{
		ID:          "fake-warn2",
		DisplayName: "Fake Warn 2",
		Kind:        usage.KindSubscription,
		CacheTTL:    300 * time.Second,
		Fetch: func(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
			return usage.Snapshot{Provider: "fake-warn2"}, nil
		},
	})
}

// recordCall appends a timestamp per fetch invocation; callCount returns
// how many times a fake's Fetch ran.
var (
	callMu  sync.Mutex
	callLog = map[string][]time.Time{}
)

func recordCall(id string) {
	callMu.Lock()
	defer callMu.Unlock()
	callLog[id] = append(callLog[id], time.Now())
}

func callCount(id string) int {
	callMu.Lock()
	defer callMu.Unlock()
	return len(callLog[id])
}

// resetCalls clears the fetch log so count assertions measure only the
// calls made during the current test.
func resetCalls() {
	callMu.Lock()
	defer callMu.Unlock()
	callLog = map[string][]time.Time{}
}

// slowActive/slowMax track the concurrent-fetch high-water mark for the
// slow fakes (T5 concurrency-cap tests). Reset slowMax with Store(0) at
// the start of each measurement.
var (
	slowActive atomic.Int64
	slowMax    atomic.Int64
)

func slowEnter() {
	n := slowActive.Add(1)
	for {
		cur := slowMax.Load()
		if n <= cur || slowMax.CompareAndSwap(cur, n) {
			return
		}
	}
}

func slowExit() {
	slowActive.Add(-1)
}

func slowPeak() int64 { return slowMax.Load() }

// newOptions returns Options with an isolated cache dir (SPEC D11: tests
// never touch the real user cache dir).
func newOptions(t *testing.T) Options {
	t.Helper()
	return Options{CacheDir: t.TempDir()}
}
