package aa

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// markerJSON builds one "currentModel" script-marker fragment in the annex
// shape: a JSON object with slug, intelligenceIndexTimePerTask and
// intelligenceIndexCostPerTask.cost.total. Passing nil omits the field;
// passing a string renders it raw (for malformed-value tests).
func markerJSON(slug string, timeV, costV any) string {
	parts := []string{fmt.Sprintf(`"slug": %q`, slug)}
	if timeV != nil {
		parts = append(parts, fmt.Sprintf(`"intelligenceIndexTimePerTask": %v`, timeV))
	}
	if costV != nil {
		parts = append(parts, fmt.Sprintf(`"intelligenceIndexCostPerTask": {"cost": {"total": %v}}`, costV))
	}
	return fmt.Sprintf(`"currentModel": {%s}`, strings.Join(parts, ", "))
}

// pageBody wraps marker fragments in an HTML page with __NEXT_DATA__-style
// JSON, matching the annex's marker description.
func pageBody(markers ...string) string {
	return `<html><body><script>window.__NEXT_DATA__ = {"props": {"pageProps": {` +
		strings.Join(markers, ",") +
		`}}};</script></body></html>`
}

// pageServer serves body for every request.
func pageServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		_, _ = w.Write([]byte(body))
	}))
	installTestTransport(t, srv)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchAAPageFromFound(t *testing.T) {
	srv := pageServer(t, pageBody(markerJSON("claude-opus-5", 12.5, 0.99)))
	got, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", true, srv.URL)
	if err != nil {
		t.Fatalf("FetchAAPageFrom: %v", err)
	}
	if got == nil {
		t.Fatal("got nil PageMetrics")
	}
	if got.Slug != "Claude-Opus-5" {
		t.Errorf("Slug = %q, want requested slug Claude-Opus-5", got.Slug)
	}
	if got.TimePerIntelligenceTaskSeconds == nil || !got.TimePerIntelligenceTaskSeconds.Equal(decimalFromStr(t, "12.5")) {
		t.Errorf("TimePerIntelligenceTaskSeconds = %v, want 12.5", got.TimePerIntelligenceTaskSeconds)
	}
	if got.FallbackCostUSD == nil || !got.FallbackCostUSD.Equal(decimalFromStr(t, "0.99")) {
		t.Errorf("FallbackCostUSD = %v, want 0.99", got.FallbackCostUSD)
	}
}

func TestFetchAAPageFromNoCost(t *testing.T) {
	// The marker's cost field is deliberately non-numeric garbage: with
	// requireFallbackCost=false the cost must not be probed at all.
	srv := pageServer(t, pageBody(markerJSON("claude-opus-5", 12.5, `"not-a-number"`)))
	got, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", false, srv.URL)
	if err != nil {
		t.Fatalf("FetchAAPageFrom: %v", err)
	}
	if got == nil {
		t.Fatal("got nil PageMetrics")
	}
	if got.FallbackCostUSD != nil {
		t.Errorf("FallbackCostUSD = %v, want nil (cost not required)", got.FallbackCostUSD)
	}
	if got.TimePerIntelligenceTaskSeconds == nil || !got.TimePerIntelligenceTaskSeconds.Equal(decimalFromStr(t, "12.5")) {
		t.Errorf("TimePerIntelligenceTaskSeconds = %v, want 12.5", got.TimePerIntelligenceTaskSeconds)
	}
}

func TestFetchAAPageFromZeroMarkers(t *testing.T) {
	t.Run("no markers at all", func(t *testing.T) {
		srv := pageServer(t, `<html><body><p>nothing here</p></body></html>`)
		got, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", true, srv.URL)
		if err != nil {
			t.Fatalf("FetchAAPageFrom: %v", err)
		}
		if got != nil {
			t.Errorf("got %+v, want (nil, nil)", got)
		}
	})

	t.Run("markers present but none matching", func(t *testing.T) {
		srv := pageServer(t, pageBody(markerJSON("gpt-5", 1.0, 0.5)))
		got, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", true, srv.URL)
		if err != nil {
			t.Fatalf("FetchAAPageFrom: %v", err)
		}
		if got != nil {
			t.Errorf("got %+v, want (nil, nil)", got)
		}
	})
}

func TestFetchAAPageFromAmbiguous(t *testing.T) {
	t.Run("two unequal-slug matches -> unsupported_response", func(t *testing.T) {
		// Both markers match "Claude-Opus-5" case-insensitively once variant
		// suffixes are stripped, but their slugs differ.
		srv := pageServer(t, pageBody(
			markerJSON("claude-opus-5", 12.5, 0.99),
			markerJSON("Claude-Opus-5-tau1", 11.0, 0.95),
		))
		_, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", true, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})

	t.Run("time/cost marker count mismatch -> unsupported_response", func(t *testing.T) {
		// Two matching markers with equal slugs: one carries time+cost, the
		// other only time -> counts disagree.
		srv := pageServer(t, pageBody(
			markerJSON("claude-opus-5", 12.5, 0.99),
			markerJSON("claude-opus-5", 11.0, nil),
		))
		_, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", true, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})

	t.Run("non-numeric marker value -> unsupported_response", func(t *testing.T) {
		srv := pageServer(t, pageBody(markerJSON("claude-opus-5", `"fast"`, 0.99)))
		_, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", true, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})
}

func TestFetchAAPageFromRequestShape(t *testing.T) {
	// Wrapper: the request must go to ModelPageURL(slug) with Go-default
	// headers only — no x-api-key, no Authorization.
	resetAALog()
	var path, host string
	var hadAPIKey, hadAuth bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		path = r.URL.Path
		host = r.Host
		hadAPIKey = r.Header.Get("x-api-key") != ""
		hadAuth = r.Header.Get("Authorization") != ""
		_, _ = w.Write([]byte(pageBody(markerJSON("claude-opus-5", 12.5, 0.99))))
	}))
	installTestTransport(t, srv, "artificialanalysis.ai")
	t.Cleanup(srv.Close)

	got, err := FetchAAPage(httpkit.NewClient(), "claude-opus-5", false)
	if err != nil {
		t.Fatalf("FetchAAPage: %v", err)
	}
	if got == nil {
		t.Fatal("got nil PageMetrics")
	}
	if gotURL := "https://" + host + path; gotURL != ModelPageURL("claude-opus-5") {
		t.Errorf("request URL = %q, want %q (ModelPageURL)", gotURL, ModelPageURL("claude-opus-5"))
	}
	if hadAPIKey {
		t.Error("request carried x-api-key; the public page must not be authenticated")
	}
	if hadAuth {
		t.Error("request carried Authorization; the public page must not be authenticated")
	}
}

func TestFetchAAPageFromTooLarge(t *testing.T) {
	srv := pageServer(t, strings.Repeat("x", 3<<20)) // 3 MiB
	_, err := FetchAAPageFrom(AAPageClient(), "Claude-Opus-5", false, srv.URL)
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "response_too_large" {
		t.Errorf("Code = %q, want response_too_large", fe.Code)
	}
}

func TestFetchAAPageErrors(t *testing.T) {
	t.Run("http 500 -> provider_status", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		installTestTransport(t, srv)
		t.Cleanup(srv.Close)

		_, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", false, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "provider_status" {
			t.Errorf("Code = %q, want provider_status", fe.Code)
		}
	})

	t.Run("malformed marker JSON -> unsupported_response", func(t *testing.T) {
		// An unbalanced currentModel object cannot be extracted.
		srv := pageServer(t, `<script>"currentModel": {"slug": "claude-opus-5", "intelligenceIndexTimePerTask": 12.5</script>`)
		_, err := FetchAAPageFrom(httpkit.NewClient(), "Claude-Opus-5", false, srv.URL)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})
}
