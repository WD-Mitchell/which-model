//go:build !nousage

package copilot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
)

// stubTransport is a scripted http.RoundTripper: each RoundTrip consumes the
// next queued response; when the queue is empty the last response repeats.
// Requests (and POST bodies) are recorded for assertions.
type stubTransport struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	queue    []stubResponse
	last     *stubResponse
}

type stubResponse struct {
	status int
	body   string
	err    error
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		s.bodies = append(s.bodies, string(b))
	} else {
		s.bodies = append(s.bodies, "")
	}
	var r stubResponse
	if len(s.queue) > 0 {
		r = s.queue[0]
		s.queue = s.queue[1:]
		s.last = &r
	} else if s.last != nil {
		r = *s.last
	} else {
		r = stubResponse{status: http.StatusOK, body: "{}"}
	}
	if r.err != nil {
		return nil, r.err
	}
	return &http.Response{
		StatusCode: r.status,
		Status:     fmt.Sprintf("%d %s", r.status, http.StatusText(r.status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Request:    req,
	}, nil
}

const canaryToken = "canary-secret-token-123"

func TestValidateIdentity(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		body      string
		err       error
		wantLogin string
		wantCode  string
		wantMsg   string
	}{
		{
			name:      "case 1: valid login",
			status:    http.StatusOK,
			body:      `{"login":"synthetic-user","id":42}`,
			wantLogin: "synthetic-user",
		},
		{
			name:     "case 2: 401 body echoes canary",
			status:   http.StatusUnauthorized,
			body:     `{"message":"canary-secret-token-123"}`,
			wantCode: "unauthorized",
			wantMsg:  "GitHub identity rejected the credential.",
		},
		{
			name:     "case 3: 403",
			status:   http.StatusForbidden,
			wantCode: "unauthorized",
			wantMsg:  "GitHub identity rejected the credential.",
		},
		{
			name:     "case 4: 429",
			status:   http.StatusTooManyRequests,
			wantCode: "rate_limited",
			wantMsg:  "GitHub identity rate-limited the usage request.",
		},
		{
			name:     "case 5: 500",
			status:   http.StatusInternalServerError,
			wantCode: "provider_status",
			wantMsg:  "GitHub identity usage is unavailable (HTTP 500).",
		},
		{
			name:     "case 6: invalid login characters",
			status:   http.StatusOK,
			body:     `{"login":"bad login!"}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported identity response.",
		},
		{
			name:     "case 7: no login field",
			status:   http.StatusOK,
			body:     `{"id":42}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported identity response.",
		},
		{
			name:      "case 8: 1-char login",
			status:    http.StatusOK,
			body:      `{"login":"a"}`,
			wantLogin: "a",
		},
		{
			name:     "case 9: non-string login",
			status:   http.StatusOK,
			body:     `{"login":123}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported identity response.",
		},
		{
			name:     "case 10: 302 redirect",
			status:   http.StatusFound,
			wantCode: "redirect_refused",
			wantMsg:  "The provider attempted an unsafe redirect.",
		},
		{
			name:     "case 11: transport error with canary",
			err:      errors.New("boom canary-secret-token-123"),
			wantCode: "network",
			wantMsg:  "The provider request failed.",
		},
		{
			name:     "case 12: malformed JSON",
			status:   http.StatusOK,
			body:     `{bad`,
			wantCode: "response_json",
			wantMsg:  "The provider returned unsupported JSON.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubTransport{}
			if tc.err != nil {
				stub.queue = append(stub.queue, stubResponse{err: tc.err})
			} else {
				stub.queue = append(stub.queue, stubResponse{status: tc.status, body: tc.body})
			}
			client := &http.Client{Transport: stub}

			login, err := ValidateIdentity(context.Background(), canaryToken, client)

			if tc.wantLogin != "" {
				if err != nil {
					t.Fatalf("ValidateIdentity = %q, %v; want login %q, nil error", login, err, tc.wantLogin)
				}
				if login != tc.wantLogin {
					t.Errorf("login = %q, want %q", login, tc.wantLogin)
				}
			} else {
				if err == nil {
					t.Fatalf("ValidateIdentity = %q, nil; want error code %q", login, tc.wantCode)
				}
				pe, ok := err.(*Error)
				if !ok {
					t.Fatalf("error type = %T, want *copilot.Error", err)
				}
				if pe.Code != tc.wantCode {
					t.Errorf("code = %q, want %q", pe.Code, tc.wantCode)
				}
				if pe.Message != tc.wantMsg {
					t.Errorf("message = %q, want %q", pe.Message, tc.wantMsg)
				}
				if strings.Contains(err.Error(), canaryToken) {
					t.Errorf("error %q leaks the canary token", err)
				}
			}

			// Request assertions: exactly one call to GitHubUserURL.
			stub.mu.Lock()
			defer stub.mu.Unlock()
			if len(stub.requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(stub.requests))
			}
			req := stub.requests[0]
			if req.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", req.Method)
			}
			if req.URL.String() != GitHubUserURL {
				t.Errorf("URL = %s, want %s", req.URL.String(), GitHubUserURL)
			}
			if req.Body != nil {
				t.Errorf("identity request must not carry a body")
			}

			// Exactly three headers (copilot.mjs githubIdentityHeaders,
			// sorted-key-tested); User-Agent per SPEC D4.
			keys := make([]string, 0, len(req.Header))
			for k := range req.Header {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			wantKeys := []string{"Accept", "Authorization", "User-Agent"}
			if len(keys) != len(wantKeys) {
				t.Errorf("header keys = %v, want exactly %v", keys, wantKeys)
			} else {
				for i := range wantKeys {
					if keys[i] != wantKeys[i] {
						t.Errorf("header keys = %v, want exactly %v", keys, wantKeys)
						break
					}
				}
			}
			if got := req.Header.Get("Accept"); got != "application/vnd.github+json" {
				t.Errorf("Accept = %q, want %q", got, "application/vnd.github+json")
			}
			if got := req.Header.Get("Authorization"); got != "Bearer "+canaryToken {
				t.Errorf("Authorization = %q, want %q", got, "Bearer "+canaryToken)
			}
			if got := req.Header.Get("User-Agent"); got != IdentityUserAgent {
				t.Errorf("User-Agent = %q, want %q", got, IdentityUserAgent)
			}
			// No editor/api-version keys on the identity check (copilot.mjs test 15).
			for _, k := range []string{"Editor-Version", "Editor-Plugin-Version", "X-GitHub-Api-Version"} {
				if req.Header.Get(k) != "" {
					t.Errorf("identity request carries unexpected header %s", k)
				}
			}
		})
	}
}
