//go:build !nousage

package codex

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// globalFailureCodes is the stable Failure.Code set from
// specs/global/CONTRACTS.md §1.6 (hard-coded, per F16-T6 instruction 4).
var globalFailureCodes = map[string]bool{
	"unauthorized":         true,
	"rate_limited":         true,
	"provider_status":      true,
	"expired_credential":   true,
	"unsupported_response": true,
	"login_required":       true,
	"endpoint_refused":     true,
	"untrusted_origin":     true,
	"redirect_refused":     true,
	"response_too_large":   true,
	"timeout":              true,
	"network":              true,
	"response_json":        true,
	"credential_file":      true,
	"credential_json":      true,
	"unsafe_credential":    true,
	"access_denied":        true,
	"device_expired":       true,
	"fallback_unavailable": true,
	"usage_disabled":       true,
	"usage_compiled_out":   true,
	"keychain_unavailable": true,
	"cookie_unavailable":   true,
	"signing_failed":       true,
	"rpc_protocol":         true,
}

// TestMapStatus covers mapStatus for both provider names (CONTRACTS §7).
func TestMapStatus(t *testing.T) {
	cases := []struct {
		provider string
		status   int
		wantCode string
		wantMsg  string
	}{
		{"Codex", 401, "unauthorized", "Codex rejected the credential."},
		{"Codex", 403, "unauthorized", "Codex rejected the credential."},
		{"Codex", 429, "rate_limited", "Codex rate-limited the usage request."},
		{"Codex", 400, "provider_status", "Codex usage is unavailable (HTTP 400)."},
		{"Codex", 404, "provider_status", "Codex usage is unavailable (HTTP 404)."},
		{"Codex", 500, "provider_status", "Codex usage is unavailable (HTTP 500)."},
		{"Codex", 502, "provider_status", "Codex usage is unavailable (HTTP 502)."},
		{"Codex", 503, "provider_status", "Codex usage is unavailable (HTTP 503)."},
		{"Codex fallback", 401, "unauthorized", "Codex fallback rejected the credential."},
		{"Codex fallback", 403, "unauthorized", "Codex fallback rejected the credential."},
		{"Codex fallback", 429, "rate_limited", "Codex fallback rate-limited the usage request."},
		{"Codex fallback", 500, "provider_status", "Codex fallback usage is unavailable (HTTP 500)."},
	}
	for _, tc := range cases {
		t.Run(tc.provider+"/"+strings.ReplaceAll(tc.wantMsg, " ", "_"), func(t *testing.T) {
			e := mapStatus(tc.provider, tc.status)
			if e.Code != tc.wantCode {
				t.Errorf("mapStatus(%q, %d).Code = %q, want %q", tc.provider, tc.status, e.Code, tc.wantCode)
			}
			if e.Message != tc.wantMsg {
				t.Errorf("mapStatus(%q, %d).Message = %q, want %q", tc.provider, tc.status, e.Message, tc.wantMsg)
			}
			if !globalFailureCodes[e.Code] {
				t.Errorf("code %q not in the global §1.6 set", e.Code)
			}
		})
	}
}

// TestTrustedFallbackURL covers the validateTrustedBaseUrl precondition table
// (F16-T6 cases 5-11).
func TestTrustedFallbackURL(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		origin  string
		want    string
		wantErr string
	}{
		{
			name:   "valid origin and path",
			base:   "https://trusted.example/v1",
			origin: "https://trusted.example",
			want:   "https://trusted.example/v1/api/codex/usage",
		},
		{
			name:    "origin mismatch",
			base:    "https://other.example/v1",
			origin:  "https://trusted.example",
			wantErr: "untrusted_origin",
		},
		{
			name:   "trusted pathname slash accepted",
			base:   "https://trusted.example/v1",
			origin: "https://trusted.example/",
			want:   "https://trusted.example/v1/api/codex/usage",
		},
		{
			name:    "userinfo on base",
			base:    "https://user@trusted.example/v1",
			origin:  "https://trusted.example",
			wantErr: "untrusted_origin",
		},
		{
			name:    "query on base",
			base:    "https://trusted.example/v1?q=1",
			origin:  "https://trusted.example",
			wantErr: "untrusted_origin",
		},
		{
			name:    "http scheme",
			base:    "http://trusted.example/v1",
			origin:  "https://trusted.example",
			wantErr: "untrusted_origin",
		},
		{
			name:    "unparseable base",
			base:    "://bad",
			origin:  "https://trusted.example",
			wantErr: "untrusted_origin",
		},
		{
			name:    "trusted origin with path",
			base:    "https://trusted.example/v1",
			origin:  "https://trusted.example/not-bare",
			wantErr: "untrusted_origin",
		},
		{
			name:    "base path escapes origin",
			base:    "https://trusted.example/../escape",
			origin:  "https://trusted.example",
			wantErr: "endpoint_refused",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := trustedFallbackURL(tc.base, tc.origin)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("trustedFallbackURL() = %q, want error %q", got, tc.wantErr)
				}
				var ce *Error
				if !asCodexError(err, &ce) || ce.Code != tc.wantErr {
					t.Errorf("error = %v, want code %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("trustedFallbackURL() error: %v", err)
			}
			if got != tc.want {
				t.Errorf("trustedFallbackURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func asCodexError(err error, target **Error) bool {
	ce, ok := err.(*Error)
	if !ok {
		return false
	}
	*target = ce
	return true
}

// TestRedirectsRefused asserts 3xx statuses are refused at the request layer
// for every redirect status (they never reach mapStatus).
func TestRedirectsRefused(t *testing.T) {
	for _, status := range []int{301, 302, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
			stub := &stubTransport{}
			stub.fn = func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: status,
					Status:     http.StatusText(status),
					Header:     http.Header{"Location": []string{"https://evil.example/steal"}},
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
				}, nil
			}
			snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
			if err != nil {
				t.Fatalf("Fetch() error: %v", err)
			}
			if snap.Failure == nil || snap.Failure.Code != "redirect_refused" {
				t.Fatalf("Failure = %+v, want redirect_refused", snap.Failure)
			}
			if snap.Failure.Message != "The provider attempted an unsafe redirect." {
				t.Errorf("Message = %q, want %q", snap.Failure.Message, "The provider attempted an unsafe redirect.")
			}
			if len(stub.reqs) != 1 {
				t.Errorf("requests = %d, want 1", len(stub.reqs))
			}
		})
	}
}

// TestEveryEmittedCodeInGlobalSet asserts every code this feature emits is a
// member of the global §1.6 set.
func TestEveryEmittedCodeInGlobalSet(t *testing.T) {
	emitted := []string{
		"credential_file", "credential_json", "unsafe_credential",
		"endpoint_refused", "untrusted_origin", "redirect_refused",
		"response_too_large", "response_json", "timeout", "network",
		"unauthorized", "rate_limited", "provider_status",
		"fallback_unavailable", "unsupported_response",
	}
	for _, code := range emitted {
		if !globalFailureCodes[code] {
			t.Errorf("emitted code %q is not in the global §1.6 set", code)
		}
	}
}
