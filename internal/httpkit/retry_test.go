package httpkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newRetryServer() (*httptest.Server, *atomic.Int32) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		switch r.URL.Path {
		case "/flaky500":
			if n == 1 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				_, _ = w.Write([]byte("ok"))
			}
		case "/always500":
			w.WriteHeader(http.StatusInternalServerError)
		case "/once404":
			w.WriteHeader(http.StatusNotFound)
		case "/once429":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/netflaky":
			if n == 1 {
				hj, ok := w.(http.Hijacker)
				if !ok {
					return
				}
				conn, _, err := hj.Hijack()
				if err == nil {
					_ = conn.Close()
				}
				return
			}
			_, _ = w.Write([]byte("ok"))
		case "/loc301":
			http.Redirect(w, r, "/target", http.StatusMovedPermanently)
		case "/stream":
			for i := range 20 {
				_, _ = w.Write([]byte{byte(i)})
				w.(http.Flusher).Flush()
			}
		}
	}))
	return srv, &hits
}

func retryCode(t *testing.T, err error, want string) {
	t.Helper()
	e, ok := AsError(err)
	if !ok || e.Code != want {
		t.Fatalf("err = %v, want code %q", err, want)
	}
}

func TestRetryFlaky500(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/flaky500", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := NewClient().Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do err = %v, want nil", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestRetryAlways500(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/always500", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient().Do(context.Background(), req)
	e, ok := AsError(err)
	if !ok || e.Code != "provider_status" || e.StatusCode != http.StatusInternalServerError {
		t.Fatalf("Do err = %v, want provider_status/500", err)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestRetryNever4xx(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()
	c := NewClient()

	for _, tc := range []struct {
		path string
		code string
	}{
		{"/once404", "provider_status"},
		{"/once429", "rate_limited"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			before := hits.Load()
			req, err := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Do(context.Background(), req)
			retryCode(t, err, tc.code)
			if got := hits.Load(); got != before+1 {
				t.Errorf("attempts = %d, want %d (4xx never retried)", got, before+1)
			}
		})
	}
}

func TestRetryNetworkError(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/netflaky", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := NewClient().Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do err = %v, want nil", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestRetryZeroRetries(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/always500", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient(WithRetries(0)).Do(context.Background(), req)
	retryCode(t, err, "provider_status")
	if got := hits.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestRetryNotForRedirect(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/loc301", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient().Do(context.Background(), req)
	retryCode(t, err, "redirect_refused")
	if got := hits.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestRetryNotForTooLarge(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient(WithMaxBytes(10)).Do(context.Background(), req)
	retryCode(t, err, "response_too_large")
	if got := hits.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestRetryUnreplayable(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/always500", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	req.GetBody = nil
	_, err = NewClient().Do(context.Background(), req)
	retryCode(t, err, "provider_status")
	if got := hits.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1 (unreplayable request not retried)", got)
	}
}

func TestRetryCanceledContext(t *testing.T) {
	srv, hits := newRetryServer()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/flaky500", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient().Do(ctx, req)
	retryCode(t, err, "network")
	if got := hits.Load(); got != 0 {
		t.Errorf("attempts = %d, want 0 (no attempt reaches the server)", got)
	}
}

func TestRetryBackoffApplied(t *testing.T) {
	srv, _ := newRetryServer()
	defer srv.Close()

	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/flaky500", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient().Do(context.Background(), req); err != nil {
		t.Fatalf("Do err = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed < 400*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 400ms (backoff applied)", elapsed)
	}
}
