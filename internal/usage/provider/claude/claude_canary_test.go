//go:build !nousage

package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

const canaryBodyMarker = "canary-body-marker-42"

// fetchFailure bridges Fetch's (Snapshot, error) shape into an error the
// canary harness can inspect: provider failures are returned as
// "<code>: <message>" errors; a successful fetch returns nil.
func fetchFailure(ctx context.Context, cred usage.Credential, client *http.Client) error {
	snap, err := Fetch(ctx, cred, client)
	if err != nil {
		return err
	}
	if snap.Failure != nil {
		return fmt.Errorf("%s: %s", snap.Failure.Code, snap.Failure.Message)
	}
	return nil
}

// assertNoCanary runs fn under security.WithCanary and fails the test when
// the canary leaks into the returned error text (global SPEC §6 item 5).
func assertNoCanary(t *testing.T, canary string, fn func() error) error {
	t.Helper()
	err := security.WithCanary(canary, fn)
	if errors.Is(err, security.ErrCanaryLeak) {
		t.Fatalf("canary %q leaked into error text", canary)
	}
	return err
}

func TestCanaryCredentialFileUnauthorized(t *testing.T) {
	home := t.TempDir()
	writeCred(t, home, ".claude/.credentials.json", 0o600,
		`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":`+itoa(time.Now().Add(time.Hour).UnixMilli())+`}}`)
	t.Setenv("HOME", home)

	stub := &stubTransport{status: 401, body: `{"message":"` + canaryToken + `"}`}
	cred := usage.Credential{Token: canaryToken, Source: usage.AuthFile}
	err := assertNoCanary(t, canaryToken, func() error {
		return fetchFailure(context.Background(), cred, &http.Client{Transport: stub})
	})
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("error = %v, want unauthorized failure", err)
	}
}

func TestCanaryNetworkError(t *testing.T) {
	stub := &stubTransport{err: errors.New("boom " + canaryToken)}
	cred := usage.Credential{Token: canaryToken}
	err := assertNoCanary(t, canaryToken, func() error {
		return fetchFailure(context.Background(), cred, &http.Client{Transport: stub})
	})
	if err == nil || !strings.Contains(err.Error(), "network") {
		t.Fatalf("error = %v, want network failure", err)
	}
}

func TestCanaryBodyMarkerAbsentFromWindows(t *testing.T) {
	stub := &stubTransport{status: 200, body: `{"five_hour":{"utilization":25,"resets_at":"` + canaryBodyMarker + `"},"note":"` + canaryBodyMarker + `"}`}
	cred := usage.Credential{Token: canaryToken}
	snap, err := Fetch(context.Background(), cred, &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Failure != nil {
		t.Fatalf("Failure = %+v, want nil", snap.Failure)
	}
	marshaled, err := json.Marshal(snap.Windows)
	if err != nil {
		t.Fatalf("marshal windows: %v", err)
	}
	if bytes.Contains(marshaled, []byte(canaryBodyMarker)) {
		t.Errorf("canary marker leaked into normalized windows: %s", marshaled)
	}
	// The marker is also absent from the full snapshot and from any error.
	full, _ := json.Marshal(snap)
	if bytes.Contains(full, []byte(canaryBodyMarker)) {
		t.Errorf("canary marker leaked into snapshot: %s", full)
	}
}

func TestCanaryExpiredCredential(t *testing.T) {
	home := t.TempDir()
	writeCred(t, home, ".claude/.credentials.json", 0o600,
		`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":`+itoa(time.Now().Add(-time.Hour).UnixMilli())+`}}`)
	t.Setenv("HOME", home)

	stub := &stubTransport{status: 200}
	cred := usage.Credential{Token: canaryToken, Source: usage.AuthFile}
	err := assertNoCanary(t, canaryToken, func() error {
		return fetchFailure(context.Background(), cred, &http.Client{Transport: stub})
	})
	if err == nil || !strings.Contains(err.Error(), "expired_credential") {
		t.Fatalf("error = %v, want expired_credential failure", err)
	}
}

func TestCanaryRedirectLocation(t *testing.T) {
	stub := &stubTransport{status: 302, location: "https://" + canaryToken + ".example/"}
	cred := usage.Credential{Token: canaryToken}
	err := assertNoCanary(t, canaryToken, func() error {
		return fetchFailure(context.Background(), cred, &http.Client{Transport: stub})
	})
	if err == nil || !strings.Contains(err.Error(), "redirect_refused") {
		t.Fatalf("error = %v, want redirect_refused failure", err)
	}
}

func TestCanaryBroadPermissionsWarning(t *testing.T) {
	home := t.TempDir()
	writeCred(t, home, ".claude/.credentials.json", 0o644,
		`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":`+itoa(time.Now().Add(time.Hour).UnixMilli())+`}}`)
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	stub := &stubTransport{status: 200, body: oauthBasicBody}
	cred := usage.Credential{Token: canaryToken, Source: usage.AuthFile, Mode: 0o644}
	err := assertNoCanary(t, canaryToken, func() error {
		return fetchFailure(context.Background(), cred, &http.Client{Transport: stub})
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if strings.Contains(buf.String(), canaryToken) {
		t.Errorf("canary leaked into captured stderr: %q", buf.String())
	}
	if !strings.Contains(buf.String(), broadPermissionsWarning) {
		t.Errorf("stderr = %q, want broad-permission warning", buf.String())
	}
}
