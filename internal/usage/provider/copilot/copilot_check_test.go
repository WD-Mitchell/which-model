//go:build !nousage

package copilot

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// case9Fixture is copied from usage-allowance.test.mjs case 9.
const case9Fixture = `{"quota_snapshots":{"chat":{"remaining":10,"entitlement":20}}}`

func TestFetch(t *testing.T) {
	cases := []struct {
		name          string
		token         string
		userStatus    int
		userBody      string
		usageStatus   int
		usageBody     string
		wantAccount   string
		wantCode      string
		wantMsg       string
		wantCalls     int
		wantCallOrder []string
	}{
		{
			// .mjs test 15 parity: [USER, USAGE] order.
			name:          "case 1: success with case-15 fixture",
			token:         canaryToken,
			userStatus:    http.StatusOK,
			userBody:      `{"login":"synthetic-user","id":42}`,
			usageStatus:   http.StatusOK,
			usageBody:     case15Fixture,
			wantAccount:   "synthetic-user",
			wantCalls:     2,
			wantCallOrder: []string{GitHubUserURL, CopilotUsageURL},
		},
		{
			// .mjs test 16 parity: identity failure stops before the private
			// endpoint.
			name:          "case 2: identity 401 stops before usage",
			token:         canaryToken,
			userStatus:    http.StatusUnauthorized,
			userBody:      `{"message":"canary-secret-token-123"}`,
			wantCode:      "unauthorized",
			wantMsg:       "GitHub identity rejected the credential.",
			wantCalls:     1,
			wantCallOrder: []string{GitHubUserURL},
		},
		{
			name:          "case 3: invalid login stops before usage",
			token:         canaryToken,
			userStatus:    http.StatusOK,
			userBody:      `{"login":"bad login!"}`,
			wantCode:      "unsupported_response",
			wantMsg:       "GitHub returned an unsupported identity response.",
			wantCalls:     1,
			wantCallOrder: []string{GitHubUserURL},
		},
		{
			name:      "case 4: empty cred",
			token:     "",
			wantCode:  "login_required",
			wantMsg:   "No usable GitHub token was found; rerun with --login to start device login.",
			wantCalls: 0,
		},
		{
			name:          "case 5: usage 401",
			token:         canaryToken,
			userStatus:    http.StatusOK,
			userBody:      `{"login":"synthetic-user","id":42}`,
			usageStatus:   http.StatusUnauthorized,
			wantCode:      "unauthorized",
			wantMsg:       "GitHub Copilot rejected the credential.",
			wantCalls:     2,
			wantCallOrder: []string{GitHubUserURL, CopilotUsageURL},
		},
		{
			name:          "case 6: success with case-9 fixture",
			token:         canaryToken,
			userStatus:    http.StatusOK,
			userBody:      `{"login":"synthetic-user","id":42}`,
			usageStatus:   http.StatusOK,
			usageBody:     case9Fixture,
			wantAccount:   "synthetic-user",
			wantCalls:     2,
			wantCallOrder: []string{GitHubUserURL, CopilotUsageURL},
		},
		{
			name:          "case 7: usage body malformed",
			token:         canaryToken,
			userStatus:    http.StatusOK,
			userBody:      `{"login":"octocat"}`,
			usageStatus:   http.StatusOK,
			usageBody:     `{bad`,
			wantCode:      "response_json",
			wantMsg:       "The provider returned unsupported JSON.",
			wantCalls:     2,
			wantCallOrder: []string{GitHubUserURL, CopilotUsageURL},
		},
		{
			name:          "case 8: usage body empty object",
			token:         canaryToken,
			userStatus:    http.StatusOK,
			userBody:      `{"login":"octocat"}`,
			usageStatus:   http.StatusOK,
			usageBody:     `{}`,
			wantCode:      "unsupported_response",
			wantMsg:       "GitHub Copilot returned an unsupported usage shape.",
			wantCalls:     2,
			wantCallOrder: []string{GitHubUserURL, CopilotUsageURL},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubTransport{
				queue: []stubResponse{
					{status: tc.userStatus, body: tc.userBody},
					{status: tc.usageStatus, body: tc.usageBody},
				},
			}
			cred := usage.Credential{Token: tc.token}
			snap, err := Fetch(context.Background(), cred, &http.Client{Transport: stub})
			if err != nil {
				t.Fatalf("Fetch = %+v, %v; want snapshot with Failure set, nil error", snap, err)
			}

			if tc.wantCode != "" {
				if snap.Failure == nil {
					t.Fatalf("snapshot.Failure = nil, want code %q", tc.wantCode)
				}
				if snap.Failure.Code != tc.wantCode {
					t.Errorf("Failure.Code = %q, want %q", snap.Failure.Code, tc.wantCode)
				}
				if snap.Failure.Message != tc.wantMsg {
					t.Errorf("Failure.Message = %q, want %q", snap.Failure.Message, tc.wantMsg)
				}
				if strings.Contains(snap.Failure.Message, canaryToken) {
					t.Errorf("Failure.Message %q leaks the canary token", snap.Failure.Message)
				}
				if tc.token == "" && len(stub.requests) != 0 {
					// covered by wantCalls below
				}
			} else {
				if snap.Failure != nil {
					t.Fatalf("snapshot.Failure = %+v, want nil", snap.Failure)
				}
				if snap.Account != tc.wantAccount {
					t.Errorf("Account = %q, want %q", snap.Account, tc.wantAccount)
				}
				if snap.Provider != "copilot" {
					t.Errorf("Provider = %q, want %q", snap.Provider, "copilot")
				}
				if snap.Source != usage.SourceOAuth {
					t.Errorf("Source = %q, want %q", snap.Source, usage.SourceOAuth)
				}
				if snap.Confidence != "live" {
					t.Errorf("Confidence = %q, want %q", snap.Confidence, "live")
				}
				if snap.FetchedAt.IsZero() {
					t.Errorf("FetchedAt = zero, want now UTC")
				}
			}

			stub.mu.Lock()
			calls := len(stub.requests)
			order := make([]string, 0, calls)
			for _, r := range stub.requests {
				order = append(order, r.URL.String())
			}
			stub.mu.Unlock()
			if calls != tc.wantCalls {
				t.Errorf("HTTP calls = %d, want %d", calls, tc.wantCalls)
			}
			if len(tc.wantCallOrder) > 0 {
				if len(order) != len(tc.wantCallOrder) {
					t.Errorf("call order = %v, want %v", order, tc.wantCallOrder)
				} else {
					for i := range tc.wantCallOrder {
						if order[i] != tc.wantCallOrder[i] {
							t.Errorf("call order = %v, want %v", order, tc.wantCallOrder)
							break
						}
					}
				}
			}
		})
	}
}

// TestFetchWindows asserts the windows and account of the two success
// scenarios (cases 1 and 6).
func TestFetchWindows(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{
		{status: http.StatusOK, body: `{"login":"synthetic-user","id":42}`},
		{status: http.StatusOK, body: case15Fixture},
	}}
	snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch = %v", err)
	}
	if len(snap.Windows) != 1 {
		t.Fatalf("len(Windows) = %d, want 1", len(snap.Windows))
	}
	w := snap.Windows[0]
	if w.ID != "chat" || w.Remaining == nil || *w.Remaining != 225 || w.Limit == nil || *w.Limit != 300 ||
		w.UsedPercent == nil || *w.UsedPercent != 25 {
		t.Errorf("window = %+v, want chat remaining 225 limit 300 used 25", w)
	}

	stub2 := &stubTransport{queue: []stubResponse{
		{status: http.StatusOK, body: `{"login":"synthetic-user","id":42}`},
		{status: http.StatusOK, body: case9Fixture},
	}}
	snap2, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub2})
	if err != nil {
		t.Fatalf("Fetch = %v", err)
	}
	w2 := snap2.Windows[0]
	if w2.ID != "chat" || w2.Remaining == nil || *w2.Remaining != 10 || w2.Limit == nil || *w2.Limit != 20 {
		t.Errorf("window = %+v, want chat remaining 10 limit 20", w2)
	}
}

func TestFetchSnapshotUsageKnown(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		known      bool
	}{
		{"positive", case15Fixture, 200, true},
		{"zero", `{"quota_snapshots":{"chat":{"percent_remaining":100}}}`, 200, true},
		{"unlimited", `{"quota_snapshots":{"chat":{"unlimited":true}}}`, 200, true},
		{"failure", case15Fixture, 401, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubTransport{queue: []stubResponse{{status: 200, body: `{"login":"synthetic-user"}`}, {status: tc.status, body: tc.body}}}
			snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
			if err != nil {
				t.Fatal(err)
			}
			if snap.UsageKnown != tc.known {
				t.Errorf("snapshot known=%v want=%v", snap.UsageKnown, tc.known)
			}
			if (snap.Failure != nil) != (tc.status != 200) {
				t.Errorf("unexpected failure: %v", snap.Failure)
			}
		})
	}
}
