//go:build !nousage

package claude

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// globalFailureCodes is the stable Failure.Code set from
// specs/global/CONTRACTS.md §1.6 (hard-coded here).
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

// TestMapStatusEveryStatus exercises mapStatus over every HTTP status in
// 100..599 and asserts the exact (code, message) pair per CONTRACTS §6.
func TestMapStatusEveryStatus(t *testing.T) {
	for status := 100; status <= 599; status++ {
		err := mapStatus("Claude", status)
		if status == 200 {
			if err != nil {
				t.Errorf("mapStatus(200) = %v, want nil", err)
			}
			continue
		}
		if err == nil {
			t.Errorf("mapStatus(%d) = nil, want an error", status)
			continue
		}
		var wantCode, wantMsg string
		switch status {
		case 401, 403:
			wantCode, wantMsg = "unauthorized", "Claude rejected the credential."
		case 429:
			wantCode, wantMsg = "rate_limited", "Claude rate-limited the usage request."
		default:
			wantCode, wantMsg = "provider_status", fmt.Sprintf("Claude usage is unavailable (HTTP %d).", status)
		}
		if err.Code != wantCode {
			t.Errorf("mapStatus(%d).Code = %q, want %q", status, err.Code, wantCode)
		}
		if err.Message != wantMsg {
			t.Errorf("mapStatus(%d).Message = %q, want %q", status, err.Message, wantMsg)
		}
		if !globalFailureCodes[err.Code] {
			t.Errorf("mapStatus(%d).Code %q is not a member of the global §1.6 Failure.Code set", status, err.Code)
		}
	}
}

// TestRedirectStatusesAtRequestLayer pins that 3xx statuses never reach
// mapStatus: they are redirect_refused at the request layer (SPEC §2.5).
func TestRedirectStatusesAtRequestLayer(t *testing.T) {
	for _, status := range []int{301, 302, 307, 308} {
		t.Run(fmt.Sprintf("status %d", status), func(t *testing.T) {
			stub := &stubTransport{status: status, location: "https://example.com/redirected"}
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
		})
	}
}

// TestMapStatusSanitization pins that a 401 body carrying the canary never
// surfaces in the failure message (global SPEC §6 item 5).
func TestMapStatusSanitization(t *testing.T) {
	stub := &stubTransport{status: 401, body: canaryToken}
	snap, err := Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure == nil || snap.Failure.Code != "unauthorized" {
		t.Fatalf("Failure = %+v, want unauthorized", snap.Failure)
	}
	if strings.Contains(snap.Failure.Message, canaryToken) {
		t.Errorf("Failure.Message leaks credential: %q", snap.Failure.Message)
	}
	if snap.Failure.Message != "Claude rejected the credential." {
		t.Errorf("Message = %q, want %q", snap.Failure.Message, "Claude rejected the credential.")
	}
}

// TestEmittedCodesGlobalSet asserts every code the feature can emit is a
// member of the global §1.6 Failure.Code set.
func TestEmittedCodesGlobalSet(t *testing.T) {
	emitted := []string{
		"credential_file",
		"credential_json",
		"unsafe_credential",
		"expired_credential",
		"endpoint_refused",
		"redirect_refused",
		"response_too_large",
		"response_json",
		"timeout",
		"network",
		"unauthorized",
		"rate_limited",
		"provider_status",
		"unsupported_response",
	}
	for _, code := range emitted {
		if !globalFailureCodes[code] {
			t.Errorf("emitted code %q is not a member of the global §1.6 Failure.Code set", code)
		}
	}
}
