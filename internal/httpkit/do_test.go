package httpkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDoAllowListTLS(t *testing.T) {
	var hits atomic.Int32
	var sawUA atomic.Value
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sawUA.Store(r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.hc = srv.Client()
	c.SetAllowList([]string{srv.URL})
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := c.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do = %v, want nil", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
	if ua, _ := sawUA.Load().(string); ua != "which-model/dev" {
		t.Errorf("server saw User-Agent %q, want %q", ua, "which-model/dev")
	}
	if hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1", hits.Load())
	}
}

func TestDoAllowListExactMatch(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	c := NewClient()
	c.hc = srv.Client()
	c.SetAllowList([]string{srv.URL})
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Do(context.Background(), req)
	e, ok := AsError(err)
	if !ok || e.Code != "endpoint_refused" {
		t.Fatalf("Do err = %v, want endpoint_refused", err)
	}
	if hits.Load() != 0 {
		t.Errorf("server hits = %d, want 0 (refused before network I/O)", hits.Load())
	}
}

func TestDoAllowListRejectsPlainHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	c := NewClient()
	c.SetAllowList([]string{srv.URL})
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Do(context.Background(), req)
	e, ok := AsError(err)
	if !ok || e.Code != "endpoint_refused" {
		t.Fatalf("Do err = %v, want endpoint_refused", err)
	}
}

func TestDoAllowListRejectsUserinfoAndFragment(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"userinfo", "https://user@example.com/"},
		{"fragment", "https://example.com/#frag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient()
			c.SetAllowList([]string{"https://example.com/"})
			req, err := http.NewRequest(http.MethodGet, tc.raw, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Do(context.Background(), req)
			e, ok := AsError(err)
			if !ok || e.Code != "endpoint_refused" {
				t.Fatalf("Do err = %v, want endpoint_refused", err)
			}
		})
	}
}

func TestDoAllowListParseFailureMessage(t *testing.T) {
	c := NewClient()
	c.SetAllowList([]string{"https://example.com/"})
	req, err := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.URL = &url.URL{Scheme: "https", Host: "exa mple.com", Path: "/"}
	_, err = c.Do(context.Background(), req)
	e, ok := AsError(err)
	if !ok || e.Code != "endpoint_refused" {
		t.Fatalf("Do err = %v, want endpoint_refused", err)
	}
	const want = "endpoint_refused: the provider endpoint is not a valid URL"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDoNoAllowListEnforced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := c.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do err = %v, want nil", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
}

func TestDoUserAgentClientWins(t *testing.T) {
	saw := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw <- r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	c := NewClient()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "sneaky")
	if _, err := c.Do(context.Background(), req); err != nil {
		t.Fatalf("Do err = %v, want nil", err)
	}
	if ua := <-saw; ua != "which-model/dev" {
		t.Errorf("server saw User-Agent %q, want which-model/dev", ua)
	}
}

func TestDoUserAgentOption(t *testing.T) {
	saw := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw <- r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	c := NewClient(WithUserAgent("wm/9"))
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Do(context.Background(), req); err != nil {
		t.Fatalf("Do err = %v, want nil", err)
	}
	if ua := <-saw; ua != "wm/9" {
		t.Errorf("server saw User-Agent %q, want wm/9", ua)
	}
}

func TestDoStatusMapping(t *testing.T) {
	const canary = "CANARY_TOP_SECRET"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/404":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("nf"))
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("err"))
		case "/401":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(canary))
		case "/403":
			w.WriteHeader(http.StatusForbidden)
		case "/429":
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()

	cases := []struct {
		path   string
		code   string
		status int
	}{
		{"/404", "provider_status", http.StatusNotFound},
		{"/500", "provider_status", http.StatusInternalServerError},
		{"/401", "unauthorized", http.StatusUnauthorized},
		{"/403", "unauthorized", http.StatusForbidden},
		{"/429", "rate_limited", http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := NewClient().Do(context.Background(), req)
			e, ok := AsError(err)
			if !ok || e.Code != tc.code {
				t.Fatalf("Do err = %v, want %s", err, tc.code)
			}
			if e.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", e.StatusCode, tc.status)
			}
			if body != nil {
				t.Errorf("body = %q, want nil (non-2xx body discarded)", body)
			}
			if strings.Contains(err.Error(), canary) {
				t.Errorf("Error() %q leaks the response body", err.Error())
			}
		})
	}
}

func TestDoNetworkError(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient().Do(context.Background(), req)
	e, ok := AsError(err)
	if !ok || e.Code != "network" {
		t.Fatalf("Do err = %v, want network", err)
	}
	if e.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0", e.StatusCode)
	}
}
