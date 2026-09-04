//go:build !nousage

package fetch

import (
	"context"
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/claude"
	"io"
	"net/http"
	"strings"
	"testing"
)

type knownSnapshotTransport struct {
	t     *testing.T
	calls int
}

func (s *knownSnapshotTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.calls++
	if req.URL.String() != claude.UsageURL {
		s.t.Error("unexpected request target")
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"five_hour":{"utilization":0}}`)), Request: req}, nil
}

func TestNativeKnownSnapshotThroughFetchCacheJSON(t *testing.T) {
	t.Setenv("WHICH_MODEL_CLAUDE_OAUTH_TOKEN", "synthetic-aggregate-token")
	transport := &knownSnapshotTransport{t: t}
	old := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = old })
	opts := Options{Enabled: map[string]bool{"claude": true}, CacheDir: t.TempDir()}
	for _, source := range []usage.Source{usage.SourceAPI, usage.SourceCache} {
		snaps, _, err := FetchAll(context.Background(), []string{"claude"}, opts)
		if err != nil || len(snaps) != 1 {
			t.Fatalf("fetch result count=%d err=%v", len(snaps), err)
		}
		data, err := json.Marshal(snaps[0])
		if err != nil {
			t.Fatal(err)
		}
		var decoded usage.Snapshot
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if !decoded.UsageKnown || decoded.Source != source || decoded.Failure != nil || len(decoded.Windows) != 1 || !decoded.Windows[0].UsageKnown || decoded.Windows[0].UsedPercent == nil || *decoded.Windows[0].UsedPercent != 0 {
			t.Fatalf("JSON lost real zero usage: %s", data)
		}
	}
	if transport.calls != 1 {
		t.Errorf("cache replay performed %d total requests", transport.calls)
	}
}
