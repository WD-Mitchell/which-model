//go:build !nousage

package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Canary values (F17-T7 instruction 2).
const (
	canaryDeviceCode = "canary-device-code-456"
	canaryLogin      = "synthetic-user"
	canaryBodyMarker = "canary-body-marker-42"
)

// F17-T7 case 1: canary token with a 401 identity response whose body echoes
// the canary — the error must be free of it.
func TestCanaryIdentity401(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{{status: http.StatusUnauthorized, body: `{"message":"canary-secret-token-123"}`}}}
	err := security.WithCanary(canaryToken, func() error {
		_, err := ValidateIdentity(context.Background(), canaryToken, &http.Client{Transport: stub})
		return err
	})
	if err == nil {
		t.Fatalf("ValidateIdentity succeeded, want unauthorized error")
	}
	if errors.Is(err, security.ErrCanaryLeak) {
		t.Fatalf("canary leaked into error text: %v", err)
	}
	pe, ok := err.(*Error)
	if !ok || pe.Code != "unauthorized" {
		t.Errorf("error = %v, want unauthorized *copilot.Error", err)
	}
}

// F17-T7 case 2: canary token with a transport error containing the canary.
func TestCanaryTransportError(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{{err: errors.New("boom canary-secret-token-123")}}}
	err := security.WithCanary(canaryToken, func() error {
		_, err := ValidateIdentity(context.Background(), canaryToken, &http.Client{Transport: stub})
		return err
	})
	if err == nil || errors.Is(err, security.ErrCanaryLeak) {
		t.Fatalf("err = %v, want network error without canary", err)
	}
	pe, ok := err.(*Error)
	if !ok || pe.Code != "network" || pe.Message != "The provider request failed." {
		t.Errorf("error = %v, want network/The provider request failed.", err)
	}
}

// F17-T7 case 3: success path with login "synthetic-user" — the login appears
// ONLY in Snapshot.Account, never in any window, message, or Failure.
func TestCanarySuccessPath(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{
		{status: http.StatusOK, body: `{"login":"synthetic-user","id":42}`},
		{status: http.StatusOK, body: case15Fixture},
	}}
	var snap usage.Snapshot
	err := security.WithCanary(canaryLogin, func() error {
		var err error
		snap, err = Fetch(context.Background(), usage.Credential{Token: canaryToken}, &http.Client{Transport: stub})
		return err
	})
	if err != nil {
		t.Fatalf("Fetch = %v", err)
	}
	if snap.Account != canaryLogin {
		t.Errorf("Account = %q, want %q", snap.Account, canaryLogin)
	}
	// Serialize everything except Account and scan for the login.
	snap.Account = ""
	blob, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(blob), canaryLogin) {
		t.Errorf("login %q leaked into snapshot fields: %s", canaryLogin, blob)
	}
}

// F17-T7 case 4: a usage 200 body containing the canary body marker — the
// normalized windows and errors must not carry it.
func TestCanaryBodyMarker(t *testing.T) {
	body := `{"quota_snapshots":{"chat":{"remaining":10,"entitlement":20,"note":"canary-body-marker-42"}}}`
	stub := &stubTransport{queue: []stubResponse{{status: http.StatusOK, body: body}}}
	err := security.WithCanary(canaryBodyMarker, func() error {
		windows, err := fetchUsage(context.Background(), canaryToken, &http.Client{Transport: stub})
		if err != nil {
			return err
		}
		blob, _ := json.Marshal(windows)
		if strings.Contains(string(blob), canaryBodyMarker) {
			t.Errorf("body marker leaked into windows: %s", blob)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("fetchUsage = %v", err)
	}
}

// TestCanaryDeviceDeniedPoll is F17-T7 case 5: device flow access_denied with
// the canary device code — the message must not echo the code (.mjs test 11
// parity). Real clock, single poll.
func TestCanaryDeviceDeniedPoll(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{{status: http.StatusOK, body: `{"error":"access_denied"}`}}}
	err := security.WithCanary(canaryDeviceCode, func() error {
		_, err := PollDeviceFlow(context.Background(), &http.Client{Transport: stub}, DeviceFlow{
			DeviceCode: canaryDeviceCode,
			ExpiresIn:  60,
			Interval:   1,
		}, nil)
		return err
	})
	if err == nil || errors.Is(err, security.ErrCanaryLeak) {
		t.Fatalf("err = %v, want access_denied without the device code", err)
	}
	pe, ok := err.(*Error)
	if !ok || pe.Code != "access_denied" || pe.Message != "GitHub device login was denied or cancelled." {
		t.Errorf("error = %v, want access_denied with the fixed message", err)
	}
}

// deniedPattern is the .mjs test 14 output pattern: denied-global |
// denied-system | denied-gh | canary-secret.
var deniedPattern = regexp.MustCompile(`denied-global|denied-system|denied-gh|canary-secret`)

// F17-T7 case 6: full Fetch with denied-style tokens — no denied-global/
// denied-system/denied-gh/canary-secret in any error or Snapshot field.
func TestCanaryFullFetchDeniedTokens(t *testing.T) {
	for _, token := range []string{"denied-global-token", "denied-system-token", "denied-gh-token", canaryToken} {
		t.Run(token, func(t *testing.T) {
			stub := &stubTransport{queue: []stubResponse{
				{status: http.StatusUnauthorized, body: `{"message":"` + token + `"}`},
			}}
			var snap usage.Snapshot
			err := security.WithCanary(token, func() error {
				var err error
				snap, err = Fetch(context.Background(), usage.Credential{Token: token}, &http.Client{Transport: stub})
				return err
			})
			if err != nil {
				if errors.Is(err, security.ErrCanaryLeak) {
					t.Fatalf("canary leaked into error text: %v", err)
				}
				t.Fatalf("Fetch = %v", err)
			}
			if snap.Failure == nil {
				t.Fatalf("snapshot.Failure = nil, want unauthorized")
			}
			blob, _ := json.Marshal(snap)
			if deniedPattern.Match(blob) {
				t.Errorf("denied-style token leaked into snapshot JSON: %s", blob)
			}
		})
	}
}
