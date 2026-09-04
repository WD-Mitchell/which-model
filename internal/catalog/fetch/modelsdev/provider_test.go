package modelsdev

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// mdRecorded captures each request a test server saw. Tests reset the log
// before each scenario and assert on it afterwards; modelsdev tests run
// sequentially so a package-level log is safe.
var mdRecorded struct {
	mu   sync.Mutex
	reqs []recordedRequest
}

type recordedRequest struct {
	method  string
	headers http.Header
	url     *url.URL
}

func resetRecorded() {
	mdRecorded.mu.Lock()
	defer mdRecorded.mu.Unlock()
	mdRecorded.reqs = nil
}

func recorded() []recordedRequest {
	mdRecorded.mu.Lock()
	defer mdRecorded.mu.Unlock()
	return append([]recordedRequest(nil), mdRecorded.reqs...)
}

// installTestTransport makes the process-wide default transport trust local
// TLS test servers (test-only: InsecureSkipVerify) and optionally rewrite the
// given hosts to the test server, so collectors that pin real https URLs
// (allow-list check included) still reach the fixture. The previous transport
// is restored at test end.
func installTestTransport(t *testing.T, srv *httptest.Server, rewriteHosts ...string) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test-only
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			for _, h := range rewriteHosts {
				if host == h {
					return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
				}
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}
	t.Cleanup(func() { http.DefaultTransport = old })
}

// newTestServer serves payload with status for every request and records the
// requests in mdRecorded.
func newTestServer(t *testing.T, payload string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mdRecorded.mu.Lock()
		mdRecorded.reqs = append(mdRecorded.reqs, recordedRequest{
			method:  r.Method,
			headers: r.Header.Clone(),
			url:     r.URL,
		})
		mdRecorded.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(payload))
	}))
	installTestTransport(t, srv)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchModelsDevProvidersList(t *testing.T) {
	resetRecorded()
	payload := `[
		{"provider":"openai","id":"gpt-5.6","name":"GPT-5.6 Sol","status":"available","base_model":"","reasoning_options":{"values":["low","medium","high"]}},
		{"provider":"anthropic","id":"claude-opus-5","name":"Claude Opus 5 (latest)","status":"available","reasoning_options":{"values":["none"]}},
		{"provider":"kimi","id":"kimi-k2.7","name":"Kimi K2.7 Code","status":"deprecated"}
	]`
	srv := newTestServer(t, payload, http.StatusOK)
	got, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), srv.URL)
	if err != nil {
		t.Fatalf("FetchModelsDevProvidersFrom: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d models, want 2 (deprecated dropped)", len(got))
	}

	first := got[0]
	if first.Provider != "openai" || first.ModelID != "gpt-5.6" {
		t.Errorf("first = %+v, want provider openai / id gpt-5.6", first)
	}
	if first.Name != identity.CleanModelName("GPT-5.6 Sol") {
		t.Errorf("Name = %q, want %q (CleanModelName applied)", first.Name, identity.CleanModelName("GPT-5.6 Sol"))
	}
	if first.Status != "available" || first.BaseModel != "" {
		t.Errorf("Status/BaseModel = %q/%q, want available/empty", first.Status, first.BaseModel)
	}
	if !first.Reasoning {
		t.Error("Reasoning = false, want true")
	}
	if len(first.EffortLevels) != 3 || first.EffortLevels[0] != "high" || first.EffortLevels[1] != "low" || first.EffortLevels[2] != "medium" {
		t.Errorf("EffortLevels = %v, want sorted [high low medium]", first.EffortLevels)
	}

	second := got[1]
	if second.Provider != "anthropic" || second.ModelID != "claude-opus-5" {
		t.Errorf("second = %+v, want provider anthropic / id claude-opus-5", second)
	}
	if second.Name != "Claude Opus 5" {
		t.Errorf("Name = %q, want %q (CleanModelName applied)", second.Name, "Claude Opus 5")
	}
	if !second.Reasoning {
		t.Error("Reasoning = false, want true")
	}
	// "none" is not a valid effort: it normalizes to default -> high.
	if len(second.EffortLevels) != 1 || second.EffortLevels[0] != "high" {
		t.Errorf("EffortLevels = %v, want [high]", second.EffortLevels)
	}

	reqs := recorded()
	if len(reqs) != 1 {
		t.Fatalf("handler saw %d requests, want 1", len(reqs))
	}
	if reqs[0].method != http.MethodGet {
		t.Errorf("method = %s, want GET", reqs[0].method)
	}
	if accept := reqs[0].headers.Get("Accept"); accept != "application/json" {
		t.Errorf("Accept = %q, want application/json", accept)
	}
	if q := reqs[0].url.RawQuery; q != "" {
		t.Errorf("query = %q, want empty", q)
	}
}

func TestFetchModelsDevProvidersMissingReasoningOptions(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"no reasoning_options", `[{"provider":"kimi","id":"kimi-k2.7","name":"Kimi K2.7","status":"available"}]`},
		{"empty values", `[{"provider":"kimi","id":"kimi-k2.7","name":"Kimi K2.7","status":"available","reasoning_options":{"values":[]}}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, tt.payload, http.StatusOK)
			got, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), srv.URL)
			if err != nil {
				t.Fatalf("FetchModelsDevProvidersFrom: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d models, want 1", len(got))
			}
			m := got[0]
			if m.Reasoning {
				t.Error("Reasoning = true, want false")
			}
			if len(m.EffortLevels) != 0 {
				t.Errorf("EffortLevels = %v, want empty", m.EffortLevels)
			}
		})
	}
}

func TestFetchModelsDevProvidersErrors(t *testing.T) {
	t.Run("http 500 -> provider_status", func(t *testing.T) {
		resetRecorded()
		srv := newTestServer(t, `[]`, http.StatusInternalServerError)
		_, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "provider_status" {
			t.Errorf("Code = %q, want provider_status", fe.Code)
		}
		// httpkit retries 5xx once: the handler may see up to 2 requests.
		if n := len(recorded()); n > 2 {
			t.Errorf("handler saw %d requests, want <= 2 (one retry)", n)
		}
	})

	t.Run("not json -> response_json", func(t *testing.T) {
		srv := newTestServer(t, `definitely not json`, http.StatusOK)
		_, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "response_json" {
			t.Errorf("Code = %q, want response_json", fe.Code)
		}
	})

	t.Run("dial failure -> network", func(t *testing.T) {
		// A closed test server's port can be reused before the retry. Inject
		// the transport failure so both attempts exercise the network path.
		dialErr := errors.New("fixture connection refused")
		var attempts atomic.Int32
		old := http.DefaultTransport
		transport := &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				attempts.Add(1)
				return nil, dialErr
			},
		}
		http.DefaultTransport = transport
		t.Cleanup(func() {
			transport.CloseIdleConnections()
			http.DefaultTransport = old
		})
		_, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), ProvidersURL)
		if got := attempts.Load(); got != 2 {
			t.Fatalf("dial attempts = %d, want 2 (one retry)", got)
		}
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "network" {
			t.Errorf("Code = %q, want network", fe.Code)
		}
	})

	t.Run("non-string id -> unsupported_response", func(t *testing.T) {
		srv := newTestServer(t, `[{"provider":"openai","id":42,"name":"X","status":"available"}]`, http.StatusOK)
		_, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})
}

func TestProvidersURLConstant(t *testing.T) {
	if ProvidersURL != "https://models.dev/api.json" {
		t.Errorf("ProvidersURL = %q, want https://models.dev/api.json", ProvidersURL)
	}
}

func TestFetchModelsDevProvidersAllowList(t *testing.T) {
	// The collector pins the exact URL via SetAllowList (F04 enforces https);
	// a request to a different host/path must be refused before any dial.
	resetRecorded()
	payload := `[{"provider":"openai","id":"gpt-5.6","name":"GPT-5.6","status":"available"}]`
	srv := newTestServer(t, payload, http.StatusOK)
	_, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), srv.URL)
	if err != nil {
		t.Fatalf("FetchModelsDevProvidersFrom: %v", err)
	}
	if len(recorded()) != 1 {
		t.Errorf("handler saw %d requests, want 1 (allow-listed URL accepted)", len(recorded()))
	}
}

func TestFetchModelsDevProvidersPayloadShapeIsArray(t *testing.T) {
	// The models.dev providers endpoint is a JSON array of records; a
	// non-array body is a shape violation -> unsupported_response.
	srv := newTestServer(t, `{"models":[]}`, http.StatusOK)
	_, err := FetchModelsDevProvidersFrom(httpkit.NewClient(), srv.URL)
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "unsupported_response" {
		t.Errorf("Code = %q, want unsupported_response", fe.Code)
	}
}

// TestFetchModelsDevProvidersFileIsolation pins the T2/T3 file isolation:
// provider.go must not reference benchmark-file symbols and vice versa
// (source-text check, mirror of the benchmark-side test).
func TestFetchModelsDevProvidersFileIsolation(t *testing.T) {
	providerSrc, err := os.ReadFile("provider.go")
	if err != nil {
		t.Fatalf("read provider.go: %v", err)
	}
	for _, tok := range []string{"FetchModelsDevBenchmarks", "BenchmarksURL", "BenchmarkRecord"} {
		if strings.Contains(string(providerSrc), tok) {
			t.Errorf("provider.go references benchmark-file symbol %q", tok)
		}
	}
}
