//go:build !nousage

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

// registerTestDeviceFlowProvider registers a descriptor with a device-flow
// auth source pointing at the given test server URLs. Registered at most
// once per id (usage.Register panics on duplicates and the registry has no
// removal), so the server URLs are swapped atomically per test instead.

const testFlowProviderID = "wm-test-signin"

func registerTestDeviceFlowProvider(t *testing.T) {
	t.Helper()
	for _, existing := range usage.IDs() {
		if existing == testFlowProviderID {
			return
		}
	}
	usage.Register(usage.Descriptor{
		ID:          testFlowProviderID,
		DisplayName: "Test Sign-in",
		Kind:        usage.KindSubscription,
		Tier:        1,
		Auth: []usage.AuthSource{{
			Kind: usage.AuthOAuthDeviceFlow,
			OAuth: &usage.OAuthSpec{
				ClientID:        "test-client",
				Scope:           "read:user",
				DeviceCodeURL:   "https://placeholder.invalid/device",
				TokenURL:        "https://placeholder.invalid/token",
				VerificationURI: "https://github.com/login/device",
			},
		}},
	})
}

// newFlowTestTargets wires the registered descriptor's endpoints to per-test
// httptest servers and relaxes the flow's https self-allow-list so http test
// servers pass (security.ValidateExactHTTPS requires https; the production
// flow keeps the strict check). Restored after each test via t.Cleanup.
func newFlowTestTargets(t *testing.T, deviceBody string, tokenBodies chan string) *httptest.Server {
	t.Helper()
	oldNewFlow := newDeviceFlow
	t.Cleanup(func() { newDeviceFlow = oldNewFlow })
	device := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(deviceBody))
	}))
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, ok := <-tokenBodies
		if !ok {
			body = `{"error":"server_closed"}`
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(func() {
		device.Close()
		token.Close()
	})
	repointTestFlowSpec(device.URL, token.URL)
	newDeviceFlow = func(spec usage.OAuthSpec) *credential.DeviceFlow {
		flow := credential.NewDeviceFlow(spec)
		flow.ValidateURL = func(string) error { return nil }
		return flow
	}
	return device
}

// repointTestFlowSpec mutates the registered descriptor's OAuth spec in place
// (the registry stores pointers). Only valid in tests; production endpoints
// are constants.
func repointTestFlowSpec(deviceURL, tokenURL string) {
	desc, err := usage.Get(testFlowProviderID)
	if err != nil {
		return
	}
	for _, source := range desc.Auth {
		if source.Kind == usage.AuthOAuthDeviceFlow && source.OAuth != nil {
			source.OAuth.DeviceCodeURL = deviceURL
			source.OAuth.TokenURL = tokenURL
		}
	}
}

func TestSignInStartHappyPath(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	tokenBodies := make(chan string, 8)
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)

	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+testFlowProviderID+"]\nenabled = true\n"))
	start, err := svc.SignIn().Start(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if start.UserCode != "WDML-TEST" || start.VerificationURI != "https://github.com/login/device" {
		t.Fatalf("Start() = %+v", start)
	}
	signInMu.Lock()
	_, active := signInFlows[testFlowProviderID]
	signInMu.Unlock()
	if !active {
		t.Fatal("no active flow recorded after Start")
	}
}

func TestSignInConfirmSavesCredential(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	svc, rec := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[auth]\nuse_keychain = false\n\n[providers."+testFlowProviderID+"]\nenabled = true\n\n[[providers."+testFlowProviderID+".accounts]]\nname = \"GitHub\"\nkind = \"oauth\"\nref = \"\"\n"))
	tokenBodies := make(chan string, 8)
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)
	tokenBodies <- `{"access_token":"tok-confirm-1","token_type":"bearer"}`

	if _, err := svc.SignIn().Start(context.Background(), testFlowProviderID); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := svc.SignIn().Confirm(context.Background(), testFlowProviderID); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	store, err := svc.managedStoreFor(testFlowProviderID)
	if err != nil {
		t.Fatal(err)
	}
	cred, _, err := store.Resolve(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatalf("Resolve after Confirm: %v", err)
	}
	if cred.Token != "tok-confirm-1" {
		t.Fatalf("stored token = %q, want the issued token", redactConst(cred.Token))
	}
	detail, err := svc.Providers().Detail(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatalf("Detail after Confirm: %v", err)
	}
	if len(detail.Accounts) != 1 || detail.Accounts[0].Ref != managedOAuthRef {
		t.Fatalf("accounts after Confirm = %+v, want oauth ref %q", detail.Accounts, managedOAuthRef)
	}
	var sawConfig, sawUsage bool
	for _, ev := range rec.Events() {
		if ev.Event == EventConfigChanged {
			sawConfig = true
		}
		if ev.Event == EventUsageUpdated {
			sawUsage = true
		}
	}
	if !sawConfig || !sawUsage {
		t.Fatalf("events after Confirm = %+v, want config:changed and usage:updated", rec.Events())
	}
	// Flow is consumed: a second Confirm has nothing to poll.
	if err := svc.SignIn().Confirm(context.Background(), testFlowProviderID); err == nil {
		t.Fatal("second Confirm error = nil, want validation failure")
	}
}

// redactConst keeps raw tokens out of failure output (global SPEC §6).
func redactConst(s string) string {
	if s == "" {
		return "<empty>"
	}
	return "<redacted " + s[len(s)-1:] + ">"
}

func TestSignInStartUsageDisabled(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	// No providers enabled → ResolveUsageEnabled returns false → gate refuses.
	svc, _ := newTestServices(t)
	_, err := svc.SignIn().Start(context.Background(), testFlowProviderID)
	if err == nil {
		t.Fatal("Start with usage disabled error = nil, want error")
	}
	if !strings.Contains(err.Error(), "usage is disabled") {
		t.Fatalf("err = %v, want usage-disabled message", err)
	}
}

func TestSignInStartUnknownProvider(t *testing.T) {
	svc, _ := newTestServices(t)
	_, err := svc.SignIn().Start(context.Background(), "no-such-provider")
	if err == nil {
		t.Fatal("Start(unknown) error = nil, want error")
	}
}

// noFlowProviderID is a registered provider WITHOUT a device-flow source —
// the analog of "claude" in the real binary (env/file auth only).
const noFlowProviderID = "wm-test-noflow"

func TestSignInStartNoDeviceFlowSource(t *testing.T) {
	registerTestProvider(t, noFlowProviderID)
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+noFlowProviderID+"]\nenabled = true\n"))
	_, err := svc.SignIn().Start(context.Background(), noFlowProviderID)
	if err == nil {
		t.Fatal("Start(no-flow) error = nil, want not-supported error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err = %v, want not-supported message", err)
	}
}

func TestSignInConfirmWithoutStart(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+testFlowProviderID+"]\nenabled = true\n"))
	err := svc.SignIn().Confirm(context.Background(), testFlowProviderID)
	if err == nil {
		t.Fatal("Confirm without Start error = nil, want error")
	}
	if !strings.Contains(err.Error(), "no sign-in in progress") {
		t.Fatalf("err = %v, want no-flow message", err)
	}
}

func TestSignInCancelClearsFlow(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	tokenBodies := make(chan string, 8)
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)

	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+testFlowProviderID+"]\nenabled = true\n"))
	if _, err := svc.SignIn().Start(context.Background(), testFlowProviderID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SignIn().Cancel(testFlowProviderID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	signInMu.Lock()
	_, active := signInFlows[testFlowProviderID]
	signInMu.Unlock()
	if active {
		t.Fatal("flow still active after Cancel")
	}
	// Cancelling again is success.
	if err := svc.SignIn().Cancel(testFlowProviderID); err != nil {
		t.Fatalf("double Cancel error = %v", err)
	}
}

func TestSignInCancelAbortsConfirm(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	tokenBodies := make(chan string)
	t.Cleanup(func() { close(tokenBodies) })
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)

	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+testFlowProviderID+"]\nenabled = true\n"))
	if _, err := svc.SignIn().Start(context.Background(), testFlowProviderID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- svc.SignIn().Confirm(context.Background(), testFlowProviderID)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		signInMu.Lock()
		_, active := signInFlows[testFlowProviderID]
		signInMu.Unlock()
		if active {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("flow never became active")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := svc.SignIn().Cancel(testFlowProviderID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	err := <-done
	if err == nil {
		t.Fatal("Confirm after Cancel error = nil, want cancellation")
	}
}

func TestSignInVersionRoundTrip(t *testing.T) {
	svc, _ := newTestServices(t)
	svc.SetVersion("2.1.0")
	got, err := svc.Settings().Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "2.1.0" {
		t.Fatalf("Get().Version = %q", got.Version)
	}
	// Set ignores it (read-only) and the emitted payload still carries it.
	if err := svc.Settings().Set(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	got2, err := svc.Settings().Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got2.Version != got.Version {
		t.Fatalf("Version changed across Set: %q -> %q", got.Version, got2.Version)
	}
}

// deviceFlowSpecNilCheck guards the extraction helper against silent drift.
func TestSignInDeviceFlowSpecExtraction(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	desc, err := usage.Get(testFlowProviderID)
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := deviceFlowSpec(desc)
	if !ok || spec.ClientID != "test-client" {
		t.Fatalf("deviceFlowSpec() = %+v, ok=%v", spec, ok)
	}
}
