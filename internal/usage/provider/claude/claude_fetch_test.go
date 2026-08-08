//go:build !nousage

package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// stubTransport fakes the network: RoundTrip returns a canned response and
// records every request for assertions.
type stubTransport struct {
	mu            sync.Mutex
	status        int
	body          string
	contentLength int64
	err           error
	location      string
	requests      []*http.Request
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req.Clone(req.Context()))
	if s.err != nil {
		return nil, s.err
	}
	status := s.status
	if status == 0 {
		status = 200
	}
	cl := s.contentLength
	if cl == 0 {
		cl = int64(len(s.body))
	}
	resp := &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(s.body)),
		ContentLength: cl,
		Request:       req,
	}
	if s.location != "" {
		resp.Header.Set("Location", s.location)
	}
	return resp, nil
}

func (s *stubTransport) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *stubTransport) lastRequest() *http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return nil
	}
	return s.requests[len(s.requests)-1]
}

const oauthBasicBody = `{"five_hour":{"utilization":25,"resets_at":"2030-01-01T00:00:00Z"}}`

func TestFetchSuccess(t *testing.T) {
	stub := &stubTransport{body: oauthBasicBody}
	client := &http.Client{Transport: stub}
	cred := usage.Credential{Token: canaryToken}

	snap, err := Fetch(context.Background(), cred, client)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure != nil {
		t.Fatalf("Failure = %+v, want nil", snap.Failure)
	}
	if snap.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", snap.Provider, "claude")
	}
	if snap.Source != usage.SourceOAuth {
		t.Errorf("Source = %q, want %q", snap.Source, usage.SourceOAuth)
	}
	if snap.Confidence != "live" {
		t.Errorf("Confidence = %q, want %q", snap.Confidence, "live")
	}
	if len(snap.Windows) != 1 {
		t.Fatalf("Windows = %d entries, want 1", len(snap.Windows))
	}
	assertWindow(t, snap.Windows[0], usage.Window{
		ID: "5h", Label: "five hour", Unit: usage.UnitPercent,
		UsedPercent: fp(25), WindowMinutes: ip(300),
		ResetsAt: tm("2030-01-01T00:00:00Z"), UsageKnown: true,
	})

	req := stub.lastRequest()
	if req == nil {
		t.Fatal("no request issued")
	}
	if req.Method != http.MethodGet {
		t.Errorf("Method = %q, want GET", req.Method)
	}
	if req.URL.String() != UsageURL {
		t.Errorf("URL = %q, want %q", req.URL.String(), UsageURL)
	}
	wantHeaders := map[string]string{
		"Accept":          "application/json",
		"Authorization":   "Bearer " + canaryToken,
		"Content-Type":    "application/json",
		"anthropic-beta":  "oauth-2025-04-20",
		"User-Agent":      "claude-code/2.1.0",
	}
	if len(req.Header) != len(wantHeaders) {
		t.Errorf("header count = %d, want exactly %d: %v", len(req.Header), len(wantHeaders), req.Header)
	}
	for k, v := range wantHeaders {
		if got := req.Header.Get(k); got != v {
			t.Errorf("header %q = %q, want %q", k, got, v)
		}
	}
}

func TestFetchStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantMsg    string
	}{
		{name: "401 unauthorized", status: 401, body: `{"message":"` + canaryToken + `"}`, wantCode: "unauthorized", wantMsg: "Claude rejected the credential."},
		{name: "403 unauthorized", status: 403, wantCode: "unauthorized", wantMsg: "Claude rejected the credential."},
		{name: "429 rate limited", status: 429, wantCode: "rate_limited", wantMsg: "Claude rate-limited the usage request."},
		{name: "500 provider status", status: 500, wantCode: "provider_status", wantMsg: "Claude usage is unavailable (HTTP 500)."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubTransport{status: tt.status, body: tt.body}
			snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if snap.Failure == nil {
				t.Fatal("Failure = nil, want set")
			}
			if snap.Failure.Code != tt.wantCode {
				t.Errorf("Failure.Code = %q, want %q", snap.Failure.Code, tt.wantCode)
			}
			if snap.Failure.Message != tt.wantMsg {
				t.Errorf("Failure.Message = %q, want %q", snap.Failure.Message, tt.wantMsg)
			}
			if strings.Contains(snap.Failure.Message, canaryToken) {
				t.Errorf("Failure.Message leaks credential: %q", snap.Failure.Message)
			}
		})
	}
}

func TestFetchRedirectRefused(t *testing.T) {
	stub := &stubTransport{status: 302, location: "https://evil.example/"}
	snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "redirect_refused" {
		t.Fatalf("Failure = %+v, want redirect_refused", snap.Failure)
	}
	if snap.Failure.Message != "The provider attempted an unsafe redirect." {
		t.Errorf("Message = %q, want %q", snap.Failure.Message, "The provider attempted an unsafe redirect.")
	}
}

func TestFetchResponseTooLarge(t *testing.T) {
	// The stub claims a 300 KiB Content-Length (over the F05 256 KiB budget)
	// with a 300-byte body; the Content-Length claim trips the first bounded
	// check in security.ReadResponseBounded.
	stub := &stubTransport{status: 200, contentLength: 300 * 1024, body: strings.Repeat("x", 300)}
	snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "response_too_large" {
		t.Fatalf("Failure = %+v, want response_too_large", snap.Failure)
	}
	if snap.Failure.Message != "The provider response exceeded the safe size limit." {
		t.Errorf("Message = %q, want %q", snap.Failure.Message, "The provider response exceeded the safe size limit.")
	}
}

func TestFetchResponseJSON(t *testing.T) {
	stub := &stubTransport{status: 200, body: `{bad`}
	snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "response_json" {
		t.Fatalf("Failure = %+v, want response_json", snap.Failure)
	}
	if snap.Failure.Message != "The provider returned unsupported JSON." {
		t.Errorf("Message = %q, want %q", snap.Failure.Message, "The provider returned unsupported JSON.")
	}
}

func TestFetchNetworkError(t *testing.T) {
	stub := &stubTransport{err: fmt.Errorf("boom %s", canaryToken)}
	snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "network" {
		t.Fatalf("Failure = %+v, want network", snap.Failure)
	}
	if snap.Failure.Message != "The provider request failed." {
		t.Errorf("Message = %q, want %q", snap.Failure.Message, "The provider request failed.")
	}
	if strings.Contains(snap.Failure.Message, canaryToken) {
		t.Errorf("Failure.Message leaks credential: %q", snap.Failure.Message)
	}
}

func TestFetchEmptyToken(t *testing.T) {
	stub := &stubTransport{body: oauthBasicBody}
	snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "credential_file" {
		t.Fatalf("Failure = %+v, want credential_file", snap.Failure)
	}
	wantMsg := "Claude credentials were not found; sign in with Claude Code first."
	if snap.Failure.Message != wantMsg {
		t.Errorf("Message = %q, want %q", snap.Failure.Message, wantMsg)
	}
	if snap.Provider != "claude" || snap.Source != usage.SourceOAuth || snap.Confidence != "live" {
		t.Errorf("Snapshot = provider %q source %q confidence %q, want claude/oauth/live", snap.Provider, snap.Source, snap.Confidence)
	}
	if got := stub.requestCount(); got != 0 {
		t.Errorf("HTTP requests issued = %d, want 0", got)
	}
}

func TestFetchFileLegBroadPermissions(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	writeCred(t, home, ".claude/.credentials.json", 0o644,
		`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":`+itoa(now.Add(60*time.Second).UnixMilli())+`}}`)
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	stub := &stubTransport{body: oauthBasicBody}
	cred := usage.Credential{Token: canaryToken, Source: usage.AuthFile, Mode: 0o644}
	snap, err := Fetch(context.Background(), cred, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure != nil {
		t.Fatalf("Failure = %+v, want nil", snap.Failure)
	}
	wantWarning := "Warning: Claude credential permissions are broader than 0600; review them before continuing."
	if !strings.Contains(buf.String(), wantWarning) {
		t.Errorf("stderr = %q, want it to contain %q", buf.String(), wantWarning)
	}
	if got := stub.requestCount(); got != 1 {
		t.Errorf("HTTP requests issued = %d, want 1 (fetch proceeds)", got)
	}
}

func TestFetchFileLegExpired(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	writeCred(t, home, ".claude/.credentials.json", 0o600,
		`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":`+itoa(now.Add(-1000*time.Millisecond).UnixMilli())+`}}`)
	t.Setenv("HOME", home)

	stub := &stubTransport{body: oauthBasicBody}
	cred := usage.Credential{Token: canaryToken, Source: usage.AuthFile, Mode: 0o600}
	snap, err := Fetch(context.Background(), cred, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "expired_credential" {
		t.Fatalf("Failure = %+v, want expired_credential", snap.Failure)
	}
	if snap.Failure.Message != "The Claude access token is expired." {
		t.Errorf("Message = %q, want %q", snap.Failure.Message, "The Claude access token is expired.")
	}
	if got := stub.requestCount(); got != 0 {
		t.Errorf("HTTP requests issued = %d, want 0 (expiry fails before any request)", got)
	}
}

// TestFetchFileLegFallbackToChainToken covers the D2 fallback: the file leg
// finds nothing, so Fetch proceeds with the chain-resolved cred.Token.
func TestFetchFileLegFallbackToChainToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no .claude files
	stub := &stubTransport{body: oauthBasicBody}
	cred := usage.Credential{Token: canaryToken, Source: usage.AuthFile}
	snap, err := Fetch(context.Background(), cred, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure != nil {
		t.Fatalf("Failure = %+v, want nil", snap.Failure)
	}
	req := stub.lastRequest()
	if req == nil {
		t.Fatal("no request issued")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer "+canaryToken {
		t.Errorf("Authorization = %q, want Bearer %s", got, canaryToken)
	}
}

// TestRequestJSONDirect exercises the private request helper's enforcement
// invariants directly (SPEC §2.5): endpoint allow-list, JSON object check,
// and context-deadline mapping.
func TestRequestJSONDirect(t *testing.T) {
	t.Run("endpoint refused", func(t *testing.T) {
		_, _, err := requestJSON(context.Background(), &http.Client{Transport: &stubTransport{}}, "http://evil.example/", []string{UsageURL}, nil)
		var e *Error
		if !errors.As(err, &e) || e.Code != "endpoint_refused" {
			t.Fatalf("error = %v, want endpoint_refused", err)
		}
		if e.Message != "The provider endpoint was refused." {
			t.Errorf("Message = %q, want %q", e.Message, "The provider endpoint was refused.")
		}
	})

	t.Run("non-object 2xx body", func(t *testing.T) {
		stub := &stubTransport{status: 200, body: `[1,2]`}
		_, _, err := requestJSON(context.Background(), &http.Client{Transport: stub}, UsageURL, []string{UsageURL}, nil)
		var e *Error
		if !errors.As(err, &e) || e.Code != "response_json" {
			t.Fatalf("error = %v, want response_json", err)
		}
	})

	t.Run("empty 2xx body", func(t *testing.T) {
		stub := &stubTransport{status: 200, body: ``}
		_, _, err := requestJSON(context.Background(), &http.Client{Transport: stub}, UsageURL, []string{UsageURL}, nil)
		var e *Error
		if !errors.As(err, &e) || e.Code != "response_json" {
			t.Fatalf("error = %v, want response_json", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		blocked := &blockingTransport{started: make(chan struct{})}
		_, _, err := requestJSON(ctx, &http.Client{Transport: blocked}, UsageURL, []string{UsageURL}, nil)
		var e *Error
		if !errors.As(err, &e) || e.Code != "timeout" {
			t.Fatalf("error = %v, want timeout", err)
		}
		if e.Message != "The provider request timed out." {
			t.Errorf("Message = %q, want %q", e.Message, "The provider request timed out.")
		}
	})

	t.Run("non-2xx returns status only", func(t *testing.T) {
		stub := &stubTransport{status: 503, body: "oops"}
		status, body, err := requestJSON(context.Background(), &http.Client{Transport: stub}, UsageURL, []string{UsageURL}, nil)
		if err != nil {
			t.Fatalf("requestJSON: %v", err)
		}
		if status != 503 {
			t.Errorf("status = %d, want 503", status)
		}
		if body != nil {
			t.Errorf("body = %q, want nil for non-2xx", body)
		}
	})
}

// blockingTransport stalls until the request context is done, then returns
// the context error — reliably producing a DeadlineExceeded transport error.
type blockingTransport struct {
	started chan struct{}
}

func (b *blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	close(b.started)
	<-req.Context().Done()
	return nil, req.Context().Err()
}
