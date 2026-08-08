package aa

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	sdecimal "github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// aaLog records what the aaServer handler saw, per request. Tests reset it
// before each scenario; aa tests run sequentially so a package-level log is
// safe.
var aaLog struct {
	mu   sync.Mutex
	keys []string
	urls []string
}

func resetAALog() {
	aaLog.mu.Lock()
	defer aaLog.mu.Unlock()
	aaLog.keys = nil
	aaLog.urls = nil
}

func loggedKeys() []string {
	aaLog.mu.Lock()
	defer aaLog.mu.Unlock()
	return append([]string(nil), aaLog.keys...)
}

func loggedURLs() []string {
	aaLog.mu.Lock()
	defer aaLog.mu.Unlock()
	return append([]string(nil), aaLog.urls...)
}

func logRequest(r *http.Request) {
	aaLog.mu.Lock()
	defer aaLog.mu.Unlock()
	aaLog.keys = append(aaLog.keys, r.Header.Get("x-api-key"))
	aaLog.urls = append(aaLog.urls, r.URL.String())
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

// aaServer serves pages[i] with statuses[i] for requests with ?page=i+1.
// Requests beyond the table repeat the last page/status (handy for
// endless-has_more fixtures). Every request is logged in aaLog.
func aaServer(t *testing.T, pages []string, statuses []int) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		idx := -1
		if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil {
			idx = n - 1
		}
		if idx < 0 || idx >= len(pages) {
			idx = len(pages) - 1
		}
		status := http.StatusNotFound
		body := ""
		if idx >= 0 {
			status = statuses[idx]
			body = pages[idx]
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	installTestTransport(t, srv)
	t.Cleanup(srv.Close)
	return srv
}

// envelope renders an AA v2 envelope with the given page metadata and items.
func envelope(page int, hasMore bool, totalPages int, items ...string) string {
	data := "[]"
	if len(items) > 0 {
		data = "[" + strings.Join(items, ",") + "]"
	}
	return fmt.Sprintf(`{"data":%s,"pagination":{"page":%d,"has_more":%v,"total_pages":%d}}`,
		data, page, hasMore, totalPages)
}

func itemJSON(slug string, evaluations map[string]any, performance map[string]any, cost any) string {
	parts := []string{fmt.Sprintf(`"slug":%q`, slug)}
	if evaluations != nil {
		evals, err := json.Marshal(evaluations)
		if err != nil {
			panic(err)
		}
		parts = append(parts, `"evaluations":`+string(evals))
	}
	if performance != nil {
		perf, err := json.Marshal(performance)
		if err != nil {
			panic(err)
		}
		parts = append(parts, `"performance":`+string(perf))
	}
	if cost != nil {
		c, err := json.Marshal(cost)
		if err != nil {
			panic(err)
		}
		parts = append(parts, `"artificial_analysis_intelligence_index_cost":`+string(c))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func TestAABenchmarkFields(t *testing.T) {
	if len(AABenchmarkFields) < 10 {
		t.Fatalf("len(AABenchmarkFields) = %d, want >= 10", len(AABenchmarkFields))
	}
	seenFields := map[string]bool{}
	for _, f := range AABenchmarkFields {
		if f.Field == "" {
			t.Error("AABenchmarkField with empty Field")
		}
		if !strings.HasPrefix(f.Column, "benchmark:") {
			t.Errorf("Column %q does not start with benchmark:", f.Column)
		}
		if seenFields[f.Field] {
			t.Errorf("duplicate Field %q", f.Field)
		}
		seenFields[f.Field] = true
	}
}

func TestFetchAAv2FromPagination(t *testing.T) {
	resetAALog()
	m1 := itemJSON("m1", map[string]any{"artificial_analysis_intelligence_index": 0.5}, nil, nil)
	m2 := itemJSON("m2", map[string]any{"artificial_analysis_intelligence_index": 0.6}, nil, nil)
	pages := []string{
		envelope(1, true, 2, m1),
		envelope(2, false, 2, m2),
	}
	srv := aaServer(t, pages, []int{http.StatusOK, http.StatusOK})
	got, err := FetchAAv2From(httpkit.NewClient(), "test-key-1", srv.URL, srv.URL)
	if err != nil {
		t.Fatalf("FetchAAv2From: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d models, want 2 (both pages)", len(got))
	}
	if got[0].Slug != "m1" || got[1].Slug != "m2" {
		t.Errorf("slugs = [%s %s], want [m1 m2]", got[0].Slug, got[1].Slug)
	}
	urls := loggedURLs()
	if len(urls) != 2 {
		t.Fatalf("handler saw %d requests, want 2", len(urls))
	}
	if !strings.Contains(urls[0], "page=1") || !strings.Contains(urls[1], "page=2") {
		t.Errorf("request URLs = %v, want ?page=1 and ?page=2", urls)
	}
	for i, k := range loggedKeys() {
		if k != "test-key-1" {
			t.Errorf("request %d x-api-key = %q, want test-key-1", i, k)
		}
	}
}

func TestFetchAAv2FromItemMapping(t *testing.T) {
	evals := map[string]any{
		"artificial_analysis_intelligence_index": 0.8735,
		"artificial_analysis_coding_index":       0.7000,
		"artificial_analysis_agentic_index":      0.6000,
		"tau_banking":                            0.0300,
		"tau3_banking":                           0.0500,
		"tau2_banking":                           0.0400,
		"terminalbench_v2_1":                     0.1200,
		"terminalbench_hard":                     0.0800,
		"scicode":                                0.2500,
		"ifbench":                                0.3100,
		"ifeval":                                 0.4100,
		"hle":                                    0.0700,
		"gpqa_diamond":                           0.9100,
		"mmmu_pro":                               0.8300,
		"gdpval_aa_normalized":                   0.6400,
	}
	perf := map[string]any{"median_end_to_end_response_time_seconds": 12.5}
	cost := map[string]any{"cost_per_task": map[string]any{"total_cost": 0.42}}
	item := itemJSON("test-model", evals, perf, cost)
	srv := aaServer(t, []string{envelope(1, false, 1, item)}, []int{http.StatusOK})
	got, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
	if err != nil {
		t.Fatalf("FetchAAv2From: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d models, want 1", len(got))
	}
	m := got[0]
	if m.Slug != "test-model" {
		t.Errorf("Slug = %q, want test-model", m.Slug)
	}
	want := map[string]string{
		"IntelligenceIndex":     "87.35", // 0.8735 x 100, Round(2)
		"CodingIndex":           "70",
		"AgenticIndex":          "60",
		"MedianResponseSeconds": "12.5",
		"CostPerTaskUSD":        "0.42",
	}
	fields := map[string]*sdecimal.Decimal{
		"IntelligenceIndex":     m.IntelligenceIndex,
		"CodingIndex":           m.CodingIndex,
		"AgenticIndex":          m.AgenticIndex,
		"MedianResponseSeconds": m.MedianResponseSeconds,
		"CostPerTaskUSD":        m.CostPerTaskUSD,
	}
	for name, w := range want {
		if fields[name] == nil {
			t.Errorf("%s = nil, want %s", name, w)
			continue
		}
		if !fields[name].Equal(decimalFromStr(t, w)) {
			t.Errorf("%s = %s, want %s", name, fields[name], w)
		}
	}
	// Every present evaluation lands in Benchmarks[Column]; the three
	// tau*_banking variants collapse into benchmark:τ3 Banking (max wins).
	if len(m.Benchmarks) != 12 {
		t.Fatalf("len(Benchmarks) = %d, want 12 distinct columns", len(m.Benchmarks))
	}
	colWants := map[string]string{
		"benchmark:Artificial Analysis Coding Index":     "70",
		"benchmark:Artificial Analysis Coding Agent Index": "60",
		"benchmark:τ3 Banking":                           "5",   // max(0.03, 0.05, 0.04) x 100
		"benchmark:Terminal-Bench":                       "12",  // 0.12 x 100
		"benchmark:Terminal-Bench Hard":                  "8",   // 0.08 x 100
		"benchmark:SciCode":                              "25",  // 0.25 x 100
		"benchmark:IFBench":                              "31",  // 0.31 x 100
		"benchmark:IFEval":                               "41",  // 0.41 x 100
		"benchmark:Humanity's Last Exam":                 "7",   // 0.07 x 100
		"benchmark:GPQA Diamond":                         "91",  // 0.91 x 100
		"benchmark:MMMU Pro":                             "83",  // 0.83 x 100
		"benchmark:GDPval-AA":                            "64",  // 0.64 x 100
	}
	for col, w := range colWants {
		v, ok := m.Benchmarks[col]
		if !ok {
			t.Errorf("Benchmarks[%q] missing", col)
			continue
		}
		if !v.Equal(decimalFromStr(t, w)) {
			t.Errorf("Benchmarks[%q] = %s, want %s", col, v, w)
		}
	}
}

func decimalFromStr(t *testing.T, s string) sdecimal.Decimal {
	t.Helper()
	d, err := sdecimal.NewFromString(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func TestFetchAAv2FromMissingOptional(t *testing.T) {
	item := itemJSON("sparse-model",
		map[string]any{
			"artificial_analysis_intelligence_index": 0.5,
			"artificial_analysis_coding_index":       0.7,
		},
		nil, nil)
	srv := aaServer(t, []string{envelope(1, false, 1, item)}, []int{http.StatusOK})
	got, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
	if err != nil {
		t.Fatalf("FetchAAv2From: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d models, want 1", len(got))
	}
	m := got[0]
	if m.AgenticIndex != nil {
		t.Errorf("AgenticIndex = %v, want nil (evaluation absent)", m.AgenticIndex)
	}
	if m.MedianResponseSeconds != nil {
		t.Errorf("MedianResponseSeconds = %v, want nil (performance absent)", m.MedianResponseSeconds)
	}
	if m.CostPerTaskUSD != nil {
		t.Errorf("CostPerTaskUSD = %v, want nil (cost absent)", m.CostPerTaskUSD)
	}
	if _, ok := m.Benchmarks["benchmark:Artificial Analysis Coding Agent Index"]; ok {
		t.Error("Benchmarks has absent column")
	}
	if _, ok := m.Benchmarks["benchmark:Artificial Analysis Coding Index"]; !ok {
		t.Error("Benchmarks missing present column")
	}
}

func TestFetchAAv2FromNegativeAndVariants(t *testing.T) {
	t.Run("negative evaluation -> unsupported_response", func(t *testing.T) {
		item := itemJSON("bad-model", map[string]any{"artificial_analysis_intelligence_index": -0.1}, nil, nil)
		srv := aaServer(t, []string{envelope(1, false, 1, item)}, []int{http.StatusOK})
		_, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})

	t.Run("tau variant dedup highest wins", func(t *testing.T) {
		base := itemJSON("model-x", map[string]any{"artificial_analysis_intelligence_index": 0.5}, nil, nil)
		tau := itemJSON("model-x-tau1", map[string]any{"artificial_analysis_intelligence_index": 0.9}, nil, nil)
		srv := aaServer(t, []string{envelope(1, false, 1, base, tau)}, []int{http.StatusOK})
		got, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
		if err != nil {
			t.Fatalf("FetchAAv2From: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d models, want 1 (variant records deduped)", len(got))
		}
		if got[0].Slug != "model-x" {
			t.Errorf("Slug = %q, want model-x (root slug)", got[0].Slug)
		}
		if got[0].IntelligenceIndex == nil || !got[0].IntelligenceIndex.Equal(decimalFromStr(t, "90")) {
			t.Errorf("IntelligenceIndex = %v, want 90 (highest wins)", got[0].IntelligenceIndex)
		}
	})
}

func TestFetchAAv2FromPaginationInvariants(t *testing.T) {
	t.Run("page mismatch -> unsupported_response", func(t *testing.T) {
		// Requesting page 2 gets a body claiming page 1.
		item := itemJSON("m1", map[string]any{"artificial_analysis_intelligence_index": 0.5}, nil, nil)
		pages := []string{envelope(1, true, 2, item), envelope(1, false, 2, item)}
		srv := aaServer(t, pages, []int{http.StatusOK, http.StatusOK})
		_, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})

	t.Run("endless has_more -> error after MaxPages requests", func(t *testing.T) {
		resetAALog()
		var count int
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count++
			logRequest(r)
			n, _ := strconv.Atoi(r.URL.Query().Get("page"))
			body := envelope(n, true, 200)
			_, _ = w.Write([]byte(body))
		}))
		installTestTransport(t, srv)
		t.Cleanup(srv.Close)

		_, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
		if count != MaxPages {
			t.Errorf("handler saw %d requests, want exactly %d", count, MaxPages)
		}
	})
}

func TestFetchAAv2FromStatusMapping(t *testing.T) {
	t.Run("primary 401 -> unauthorized, zero free requests", func(t *testing.T) {
		var freeHits int
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			if strings.HasSuffix(r.URL.Path, "/free") {
				freeHits++
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(envelope(1, false, 1)))
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		}))
		installTestTransport(t, srv)
		t.Cleanup(srv.Close)

		_, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL+"/primary", srv.URL+"/free")
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unauthorized" {
			t.Errorf("Code = %q, want unauthorized", fe.Code)
		}
		if freeHits != 0 {
			t.Errorf("freeURL saw %d requests, want 0 (401 never falls back)", freeHits)
		}
	})

	t.Run("primary 403 -> one full fallback pagination on freeURL", func(t *testing.T) {
		freeModel1 := itemJSON("free-m1", map[string]any{"artificial_analysis_intelligence_index": 0.5}, nil, nil)
		freeModel2 := itemJSON("free-m2", map[string]any{"artificial_analysis_intelligence_index": 0.6}, nil, nil)
		var freePages []int
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			if strings.HasSuffix(r.URL.Path, "/free") {
				freePages = append(freePages, mustPage(r))
				if mustPage(r) == 1 {
					_, _ = w.Write([]byte(envelope(1, true, 2, freeModel1)))
				} else {
					_, _ = w.Write([]byte(envelope(2, false, 2, freeModel2)))
				}
				return
			}
			w.WriteHeader(http.StatusForbidden)
		}))
		installTestTransport(t, srv)
		t.Cleanup(srv.Close)

		got, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL+"/primary", srv.URL+"/free")
		if err != nil {
			t.Fatalf("FetchAAv2From: %v", err)
		}
		if len(got) != 2 || got[0].Slug != "free-m1" || got[1].Slug != "free-m2" {
			t.Fatalf("got %+v, want the two freeURL models", got)
		}
		if len(freePages) != 2 || freePages[0] != 1 || freePages[1] != 2 {
			t.Errorf("freeURL pages fetched = %v, want exactly [1 2] (full pagination once)", freePages)
		}
	})

	t.Run("primary+free 403 -> access_denied", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			w.WriteHeader(http.StatusForbidden)
		}))
		installTestTransport(t, srv)
		t.Cleanup(srv.Close)

		_, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL+"/primary", srv.URL+"/free")
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "access_denied" {
			t.Errorf("Code = %q, want access_denied", fe.Code)
		}
	})

	t.Run("primary 429 -> rate_limited, exactly one primary request", func(t *testing.T) {
		var primaryHits int
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			primaryHits++
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		installTestTransport(t, srv)
		t.Cleanup(srv.Close)

		_, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "rate_limited" {
			t.Errorf("Code = %q, want rate_limited", fe.Code)
		}
		if primaryHits != 1 {
			t.Errorf("primary saw %d requests, want exactly 1 (4xx never retried)", primaryHits)
		}
	})

	t.Run("primary 500 -> provider_status", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		installTestTransport(t, srv)
		t.Cleanup(srv.Close)

		_, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "provider_status" {
			t.Errorf("Code = %q, want provider_status", fe.Code)
		}
	})
}

func mustPage(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		panic(err)
	}
	return n
}

func TestFetchAAv2FromCanary(t *testing.T) {
	resetAALog()
	const key = "canary-key-abc123"
	// Primary 403 triggers the fallback; free 500 surfaces provider_status.
	// Every assertion below is a canary: the key must appear only in the
	// x-api-key header, never in URLs or error text.
	var freePath, freeQuery string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		if strings.Contains(r.URL.String(), key) {
			t.Errorf("request URL %q leaks the key", r.URL.String())
		}
		if strings.HasSuffix(r.URL.Path, "/free") {
			freePath = r.URL.Path
			freeQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	installTestTransport(t, srv)
	t.Cleanup(srv.Close)

	_, err := FetchAAv2From(httpkit.NewClient(), key, srv.URL+"/primary", srv.URL+"/free")
	if err == nil {
		t.Fatal("FetchAAv2From: nil error, want provider_status")
	}
	if msg := err.Error(); strings.Contains(msg, key) {
		t.Errorf("error %q leaks the key", msg)
	}
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "provider_status" {
		t.Errorf("Code = %q, want provider_status (free 500)", fe.Code)
	}
	// The fallback request targets freeURL itself, carrying only the page
	// query — never the key.
	if freePath != "/free" {
		t.Errorf("free fallback path = %q, want /free", freePath)
	}
	if strings.Contains(freeQuery, key) {
		t.Errorf("free fallback query %q leaks the key", freeQuery)
	}
	for _, u := range loggedURLs() {
		if strings.Contains(u, key) {
			t.Errorf("request URL %q leaks the key", u)
		}
	}
	for _, k := range loggedKeys() {
		if k != key {
			t.Errorf("x-api-key = %q, want %q on every request", k, key)
		}
	}
}

func TestFetchAAv2AllOrNothing(t *testing.T) {
	// Page 1 succeeds, page 2 fails with 500: the collector must return a
	// nil slice and the error — never partial page-1 models.
	m1 := itemJSON("m1", map[string]any{"artificial_analysis_intelligence_index": 0.5}, nil, nil)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		if mustPage(r) == 1 {
			_, _ = w.Write([]byte(envelope(1, true, 2, m1)))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	installTestTransport(t, srv)
	t.Cleanup(srv.Close)

	got, err := FetchAAv2From(httpkit.NewClient(), "k", srv.URL, srv.URL)
	if err == nil {
		t.Fatal("FetchAAv2From: nil error, want provider_status")
	}
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "provider_status" {
		t.Errorf("Code = %q, want provider_status", fe.Code)
	}
	if got != nil {
		t.Errorf("got %d models, want nil (all-or-nothing)", len(got))
	}
}

func TestFetchAAv2WrapperUsesConstants(t *testing.T) {
	// The wrapper must hit the pinned PrimaryURL/FreeURL; the transport
	// rewrites artificialanalysis.ai to the local server so the allow-list
	// (https, exact match) still validates.
	primaryModel := itemJSON("primary-model", map[string]any{"artificial_analysis_intelligence_index": 0.5}, nil, nil)
	freeModel := itemJSON("free-model", map[string]any{"artificial_analysis_intelligence_index": 0.6}, nil, nil)
	var paths []string
	mode := "primary-ok"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		paths = append(paths, r.URL.Path)
		switch {
		case r.URL.Path == "/api/v2/language/models/free":
			if mode != "primary-403" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(envelope(1, false, 1, freeModel)))
		case r.URL.Path == "/api/v2/language/models":
			if mode != "primary-403" {
				_, _ = w.Write([]byte(envelope(1, false, 1, primaryModel)))
				return
			}
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	installTestTransport(t, srv, "artificialanalysis.ai")
	t.Cleanup(srv.Close)

	t.Run("primary serves the list", func(t *testing.T) {
		paths = nil
		got, err := FetchAAv2(httpkit.NewClient(), "k")
		if err != nil {
			t.Fatalf("FetchAAv2: %v", err)
		}
		if len(got) != 1 || got[0].Slug != "primary-model" {
			t.Fatalf("got %+v, want the primary model", got)
		}
		if len(paths) != 1 || !strings.HasSuffix(paths[0], "/api/v2/language/models") {
			t.Errorf("request paths = %v, want exactly one ending /api/v2/language/models", paths)
		}
	})

	t.Run("primary 403 falls back to free", func(t *testing.T) {
		mode = "primary-403"
		paths = nil
		got, err := FetchAAv2(httpkit.NewClient(), "k")
		if err != nil {
			t.Fatalf("FetchAAv2: %v", err)
		}
		if len(got) != 1 || got[0].Slug != "free-model" {
			t.Fatalf("got %+v, want the free model", got)
		}
		if len(paths) != 2 {
			t.Fatalf("request paths = %v, want primary then free", paths)
		}
		if !strings.HasSuffix(paths[0], "/api/v2/language/models") {
			t.Errorf("first path = %q, want /api/v2/language/models", paths[0])
		}
		if !strings.HasSuffix(paths[1], "/api/v2/language/models/free") {
			t.Errorf("second path = %q, want /api/v2/language/models/free", paths[1])
		}
	})
}

func TestFetchAAv2FallbackHeaderCanary(t *testing.T) {
	const key = "fallback-canary-987"
	var freeKey, freeURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		if strings.HasSuffix(r.URL.Path, "/free") {
			freeKey = r.Header.Get("x-api-key")
			freeURL = r.URL.String()
			_, _ = w.Write([]byte(envelope(1, false, 1, itemJSON("free-model", map[string]any{"artificial_analysis_intelligence_index": 0.5}, nil, nil))))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	installTestTransport(t, srv)
	t.Cleanup(srv.Close)

	got, err := FetchAAv2From(httpkit.NewClient(), key, srv.URL+"/primary", srv.URL+"/free")
	if err != nil {
		t.Fatalf("FetchAAv2From: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d models, want 1", len(got))
	}
	if freeKey != key {
		t.Errorf("free request x-api-key = %q, want %q (key travels with the fallback)", freeKey, key)
	}
	if strings.Contains(freeURL, key) {
		t.Errorf("free request URL %q leaks the key", freeURL)
	}
}

func TestFetchAAv2RedirectMapped(t *testing.T) {
	const key = "redirect-canary-456"
	// If the client ever followed the redirect, the evil server's handler
	// fails the test. httpkit refuses 3xx before any dial, so it stays at 0.
	evilHits := 0
	evil := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evilHits++
		t.Errorf("redirect Location %q was contacted (request %s)", r.Host, r.URL)
	}))
	installTestTransport(t, evil, "evil.example")
	t.Cleanup(evil.Close)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		w.Header().Set("Location", "https://evil.example/steal")
		w.WriteHeader(http.StatusFound)
	}))
	installTestTransport(t, srv)
	t.Cleanup(srv.Close)

	_, err := FetchAAv2From(httpkit.NewClient(), key, srv.URL, srv.URL)
	if err == nil {
		t.Fatal("FetchAAv2From: nil error, want redirect_refused")
	}
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "redirect_refused" {
		t.Errorf("Code = %q, want redirect_refused", fe.Code)
	}
	if msg := err.Error(); strings.Contains(msg, key) {
		t.Errorf("error %q leaks the key", msg)
	}
	if evilHits != 0 {
		t.Errorf("evil server saw %d requests, want 0", evilHits)
	}
}
