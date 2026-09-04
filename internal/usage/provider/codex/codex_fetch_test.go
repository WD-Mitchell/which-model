//go:build !nousage

package codex

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

const (
	canaryToken  = "canary-secret-token-123"
	canaryAcct   = "acct-synthetic"
	fixtureCase6 = `{"rate_limit":{"primary_window":{"used_percent":20,"reset_at":1900000000}}}`
)

// stubTransport records every request and returns canned responses
// (mirrors F15-T4's stub).
type stubTransport struct {
	reqs []*http.Request
	fn   func(*http.Request) (*http.Response, error)
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.reqs = append(s.reqs, req)
	return s.fn(req)
}

func canned(status int, body string) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	}
}

// writeAuth writes an auth.json into a fresh CODEX_HOME and returns it.
func writeAuth(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "auth.json"), content)
	t.Setenv("CODEX_HOME", dir)
	return dir
}

func wantHeaders(t *testing.T, req *http.Request) {
	t.Helper()
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want application/json", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer "+canaryToken {
		t.Errorf("Authorization = %q, want Bearer %s", got, canaryToken)
	}
	if got := req.Header.Get("ChatGPT-Account-Id"); got != canaryAcct {
		t.Errorf("ChatGPT-Account-Id = %q, want %s", got, canaryAcct)
	}
	if n := len(req.Header); n != 3 {
		t.Errorf("len(Header) = %d, want exactly 3: %v", n, req.Header)
	}
}

// TestFetch ports the checkCodexUsage table from F16-T5.
func TestFetch(t *testing.T) {
	baseAuth := `{"tokens":{"access_token":"` + canaryToken + `","account_id":"` + canaryAcct + `"}}`

	cases := []struct {
		name        string
		auth        string // "" = write baseAuth
		statuses    []int
		bodies      []string
		trusted     string // WithTrustedOrigin value; "" = none
		wantFail    string // "code: message" or ""
		wantReqURLs []string
		wantAcct    string
	}{
		{
			name:     "primary 200 normalizes",
			statuses: []int{200},
			bodies:   []string{fixtureCase6},
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "404 no configured base",
			statuses: []int{404},
			bodies:   []string{`{}`},
			wantFail: "fallback_unavailable: Codex did not advertise a configured fallback endpoint.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "404 configured base without trusted origin",
			auth:     `{"tokens":{"access_token":"` + canaryToken + `","account_id":"` + canaryAcct + `"},"base_url":"https://trusted.example/v1"}`,
			statuses: []int{404},
			bodies:   []string{`{}`},
			wantFail: "untrusted_origin: The configured Codex fallback origin was not explicitly trusted.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "404 trusted fallback 200",
			auth:     `{"tokens":{"access_token":"` + canaryToken + `","account_id":"` + canaryAcct + `"},"base_url":"https://trusted.example/v1"}`,
			statuses: []int{404, 200},
			bodies:   []string{`{}`, fixtureCase6},
			trusted:  "https://trusted.example",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
				"https://trusted.example/v1/api/codex/usage",
			},
		},
		{
			name:     "401 never falls back",
			auth:     `{"tokens":{"access_token":"` + canaryToken + `","account_id":"` + canaryAcct + `"},"base_url":"https://trusted.example/v1"}`,
			statuses: []int{401},
			bodies:   []string{`{}`},
			trusted:  "https://trusted.example",
			wantFail: "unauthorized: Codex rejected the credential.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "429 never falls back",
			auth:     `{"tokens":{"access_token":"` + canaryToken + `","account_id":"` + canaryAcct + `"},"base_url":"https://trusted.example/v1"}`,
			statuses: []int{429},
			bodies:   []string{`{}`},
			trusted:  "https://trusted.example",
			wantFail: "rate_limited: Codex rate-limited the usage request.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "404 http base origin mismatch",
			auth:     `{"tokens":{"access_token":"` + canaryToken + `","account_id":"` + canaryAcct + `"},"base_url":"http://unsafe.example"}`,
			statuses: []int{404},
			bodies:   []string{`{}`},
			trusted:  "https://unsafe.example",
			wantFail: "untrusted_origin: The configured Codex fallback origin was not explicitly trusted.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "fallback 500 provider status",
			auth:     `{"tokens":{"access_token":"` + canaryToken + `","account_id":"` + canaryAcct + `"},"base_url":"https://trusted.example/v1"}`,
			statuses: []int{404, 500},
			bodies:   []string{`{}`, `{}`},
			trusted:  "https://trusted.example",
			wantFail: "provider_status: Codex fallback usage is unavailable (HTTP 500).",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
				"https://trusted.example/v1/api/codex/usage",
			},
		},
		{
			name:     "primary 200 malformed body",
			statuses: []int{200},
			bodies:   []string{`{bad`},
			wantFail: "response_json: The provider returned unsupported JSON.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "auth.json missing",
			auth:     "MISSING",
			statuses: []int{200},
			bodies:   []string{fixtureCase6},
			wantFail: "credential_file: Codex credentials were not found; sign in with Codex first.",
		},
		{
			name:     "primary 302 redirect refused",
			statuses: []int{302},
			bodies:   []string{``},
			wantFail: "redirect_refused: The provider attempted an unsafe redirect.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
		{
			name:     "primary 200 empty shape",
			statuses: []int{200},
			bodies:   []string{`{}`},
			wantFail: "unsupported_response: Codex returned an unsupported usage shape.",
			wantReqURLs: []string{
				"https://chatgpt.com/backend-api/wham/usage",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.auth == "" {
				tc.auth = baseAuth
			}
			if tc.auth != "MISSING" {
				writeAuth(t, tc.auth)
			} else {
				t.Setenv("CODEX_HOME", t.TempDir())
			}

			stub := &stubTransport{}
			stub.fn = func(req *http.Request) (*http.Response, error) {
				idx := len(stub.reqs) - 1
				if idx < 0 || idx >= len(tc.statuses) {
					t.Fatalf("unexpected request %d to %s", len(stub.reqs), req.URL)
				}
				return canned(tc.statuses[idx], tc.bodies[idx])(req)
			}
			client := &http.Client{Transport: stub}

			ctx := context.Background()
			if tc.trusted != "" {
				ctx = WithTrustedOrigin(ctx, tc.trusted)
			}
			snap, err := Fetch(ctx, usage.Credential{}, client)
			if err != nil {
				t.Fatalf("Fetch() error: %v", err)
			}
			if tc.wantFail != "" {
				if snap.Failure == nil {
					t.Fatalf("Fetch() = %+v, want Failure %q", snap, tc.wantFail)
				}
				if got := snap.Failure.Code + ": " + snap.Failure.Message; got != tc.wantFail {
					t.Errorf("Failure = %q, want %q", got, tc.wantFail)
				}
			} else {
				if snap.Failure != nil {
					t.Fatalf("unexpected Failure: %+v", snap.Failure)
				}
			}
			if snap.Provider != "codex" {
				t.Errorf("Provider = %q, want codex", snap.Provider)
			}
			if snap.Account != "" {
				t.Errorf("Account = %q, want unset", snap.Account)
			}
			if snap.Source != usage.SourceOAuth {
				t.Errorf("Source = %q, want %q", snap.Source, usage.SourceOAuth)
			}
			if snap.Confidence != "live" {
				t.Errorf("Confidence = %q, want live", snap.Confidence)
			}
			if len(stub.reqs) != len(tc.wantReqURLs) {
				t.Fatalf("requests = %d, want %d", len(stub.reqs), len(tc.wantReqURLs))
			}
			for i, wantURL := range tc.wantReqURLs {
				if got := stub.reqs[i].URL.String(); got != wantURL {
					t.Errorf("request %d URL = %q, want %q", i, got, wantURL)
				}
				wantHeaders(t, stub.reqs[i])
			}
			if tc.wantFail == "" {
				if len(snap.Windows) != 1 || snap.Windows[0].ID != "5h" {
					t.Fatalf("windows = %+v, want one 5h window", snap.Windows)
				}
				if snap.Windows[0].UsedPercent == nil || *snap.Windows[0].UsedPercent != 20 {
					t.Errorf("UsedPercent = %v, want 20", snap.Windows[0].UsedPercent)
				}
				if snap.Windows[0].ResetsAt == nil || !snap.Windows[0].ResetsAt.Equal(*reset2030) {
					t.Errorf("ResetsAt = %v, want 2030-03-17T17:46:40Z", snap.Windows[0].ResetsAt)
				}
			}
		})
	}
}

// TestFetchIgnoreAuthFileRemnant pins that the loader is authoritative: a
// fresh temp dir with no auth.json fails with credential_file even when a
// chain credential is passed (SPEC §2.5).
func TestFetchLoaderAuthoritative(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	stub := &stubTransport{fn: canned(200, fixtureCase6)}
	snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "credential_file" {
		t.Fatalf("Failure = %+v, want credential_file", snap.Failure)
	}
	if len(stub.reqs) != 0 {
		t.Errorf("requests = %d, want 0", len(stub.reqs))
	}
}

// TestFetchNetworkError maps transport failures to network.
func TestFetchNetworkError(t *testing.T) {
	writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
	stub := &stubTransport{fn: func(*http.Request) (*http.Response, error) {
		return nil, os.ErrClosed
	}}
	snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "network" || snap.Failure.Message != "The provider request failed." {
		t.Fatalf("Failure = %+v, want network", snap.Failure)
	}
}

func TestFetchSnapshotUsageKnown(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		known      bool
	}{
		{"positive", fixtureCase6, 200, true},
		{"zero", `{"rate_limit":{"primary_window":{"used_percent":0}}}`, 200, true},
		{"credits only", `{"credits":{"balance":0}}`, 200, true},
		{"failure", fixtureCase6, 401, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
			snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: &stubTransport{fn: canned(tc.status, tc.body)}})
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
