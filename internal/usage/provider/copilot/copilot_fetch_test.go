//go:build !nousage

package copilot

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// case15Fixture is copied from usage-allowance.test.mjs case 15.
const case15Fixture = `{"quota_snapshots":{"chat":{"entitlement":300,"remaining":225,"percent_remaining":75}},"quota_reset_date":"2030-01-01"}`

func TestFetchUsage(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		err      error
		wantCode string
		wantMsg  string
	}{
		{
			name:   "case 1: 200 with case-15 fixture",
			status: http.StatusOK,
			body:   case15Fixture,
		},
		{
			name:     "case 2: 401",
			status:   http.StatusUnauthorized,
			body:     `{"message":"canary-secret-token-123"}`,
			wantCode: "unauthorized",
			wantMsg:  "GitHub Copilot rejected the credential.",
		},
		{
			name:     "case 3: 403",
			status:   http.StatusForbidden,
			wantCode: "unauthorized",
			wantMsg:  "GitHub Copilot rejected the credential.",
		},
		{
			name:     "case 4: 429",
			status:   http.StatusTooManyRequests,
			wantCode: "rate_limited",
			wantMsg:  "GitHub Copilot rate-limited the usage request.",
		},
		{
			name:     "case 5: 500",
			status:   http.StatusInternalServerError,
			wantCode: "provider_status",
			wantMsg:  "GitHub Copilot usage is unavailable (HTTP 500).",
		},
		{
			name:     "case 6: 302",
			status:   http.StatusFound,
			wantCode: "redirect_refused",
			wantMsg:  "The provider attempted an unsafe redirect.",
		},
		{
			name:     "case 7: malformed JSON",
			status:   http.StatusOK,
			body:     `{bad`,
			wantCode: "response_json",
			wantMsg:  "The provider returned unsupported JSON.",
		},
		{
			name:     "case 8: empty object",
			status:   http.StatusOK,
			body:     `{}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub Copilot returned an unsupported usage shape.",
		},
		{
			name:     "case 9: transport error with canary",
			err:      errors.New("boom canary-secret-token-123"),
			wantCode: "network",
			wantMsg:  "The provider request failed.",
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

			windows, err := fetchUsage(context.Background(), canaryToken, client)

			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("fetchUsage = %v, nil; want error %q", windows, tc.wantCode)
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
				return
			}
			if err != nil {
				t.Fatalf("fetchUsage = %v", err)
			}
			if len(windows) != 1 {
				t.Fatalf("len(windows) = %d, want 1", len(windows))
			}
			if windows[0].ID != "chat" {
				t.Errorf("windows[0].ID = %q, want %q", windows[0].ID, "chat")
			}

			// Request assertions: exactly one call to CopilotUsageURL.
			stub.mu.Lock()
			defer stub.mu.Unlock()
			if len(stub.requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(stub.requests))
			}
			req := stub.requests[0]
			if req.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", req.Method)
			}
			if req.URL.String() != CopilotUsageURL {
				t.Errorf("URL = %s, want %s", req.URL.String(), CopilotUsageURL)
			}

			// Exactly six headers (copilot.mjs copilotUsageHeaders; .mjs test
			// 15 asserts the set by sorted lowercase key). Go canonicalizes
			// header names on Set (X-GitHub-Api-Version → X-Github-Api-Version),
			// so keys are compared lowercased.
			keys := make([]string, 0, len(req.Header))
			for k := range req.Header {
				keys = append(keys, strings.ToLower(k))
			}
			sort.Strings(keys)
			wantKeys := []string{"accept", "authorization", "editor-plugin-version", "editor-version", "user-agent", "x-github-api-version"}
			if !reflect.DeepEqual(keys, wantKeys) {
				t.Errorf("header keys = %v, want exactly %v", keys, wantKeys)
			}
			wantHeaders := map[string]string{
				"Accept":                "application/vnd.github+json",
				"Authorization":         "Bearer " + canaryToken,
				"Editor-Version":        "vscode/1.96.2",
				"Editor-Plugin-Version": "copilot-chat/0.26.7",
				"User-Agent":            "GitHubCopilotChat/0.26.7",
				"X-GitHub-Api-Version":  "2025-04-01",
			}
			for k, v := range wantHeaders {
				if got := req.Header.Get(k); got != v {
					t.Errorf("header %s = %q, want %q", k, got, v)
				}
			}
		})
	}
}

// TestFetchUsageWindowValues asserts the normalized window data for the
// case-15 fixture (remaining 225, limit 300, used 25).
func TestFetchUsageWindowValues(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{{status: http.StatusOK, body: case15Fixture}}}
	windows, err := fetchUsage(context.Background(), canaryToken, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("fetchUsage = %v", err)
	}
	w := windows[0]
	if w.Remaining == nil || *w.Remaining != 225 {
		t.Errorf("Remaining = %v, want 225", w.Remaining)
	}
	if w.Limit == nil || *w.Limit != 300 {
		t.Errorf("Limit = %v, want 300", w.Limit)
	}
	if w.UsedPercent == nil || *w.UsedPercent != 25 {
		t.Errorf("UsedPercent = %v, want 25", w.UsedPercent)
	}
	if !w.UsageKnown {
		t.Errorf("UsageKnown = false, want true")
	}
}

// TestFetchUsageCancelledContext covers case 10: a done context maps to
// timeout before any request is issued.
func TestFetchUsageCancelledContext(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{{status: http.StatusOK, body: case15Fixture}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fetchUsage(ctx, canaryToken, &http.Client{Transport: stub})
	if err == nil {
		t.Fatalf("fetchUsage = nil error, want timeout")
	}
	pe, ok := err.(*Error)
	if !ok || pe.Code != "timeout" || pe.Message != "The provider request timed out." {
		t.Errorf("error = %v, want timeout/The provider request timed out.", err)
	}
	stub.mu.Lock()
	calls := len(stub.requests)
	stub.mu.Unlock()
	if calls != 0 {
		t.Errorf("HTTP calls = %d, want 0 (request never issued)", calls)
	}
}
