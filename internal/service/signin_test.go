//go:build !nousage

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/claude"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codex"
)

// registerTestDeviceFlowProvider registers a descriptor with a device-flow
// auth source pointing at the given test server URLs. Registered at most
// once per id (usage.Register panics on duplicates and the registry has no
// removal), so the server URLs are swapped atomically per test instead.

func clearTestSignInFlow(provider string) {
	signInMu.Lock()
	if active, ok := signInFlows[provider]; ok {
		if active.cancel != nil {
			active.cancel()
		}
		delete(signInFlows, provider)
	}
	signInMu.Unlock()
}

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
	t.Cleanup(func() { clearTestSignInFlow(testFlowProviderID) })
	start, err := svc.SignIn().Start(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if start.UserCode != "WDML-TEST" ||
		start.VerificationURI != "https://github.com/login/device" ||
		start.PasteRequired {
		t.Fatalf("Start() = %+v", start)
	}
	signInMu.Lock()
	_, active := signInFlows[testFlowProviderID]
	signInMu.Unlock()
	if !active {
		t.Fatal("no active flow recorded after Start")
	}
}

func TestSignInFlowIDRejectsStaleOperationsAndDuplicateStart(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	tokenBodies := make(chan string, 1)
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+testFlowProviderID+"]\nenabled = true\n"))

	started, err := svc.SignIn().Start(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatal(err)
	}
	if started.FlowID == "" {
		t.Fatal("Start returned an empty flow id")
	}
	if _, err := svc.SignIn().Start(context.Background(), testFlowProviderID); err == nil {
		t.Fatal("duplicate Start error = nil")
	}
	if err := svc.SignIn().Confirm(context.Background(), testFlowProviderID, "stale-flow", "Stale"); err == nil {
		t.Fatal("Confirm accepted a stale flow id")
	}
	if err := svc.SignIn().Cancel(testFlowProviderID, "stale-flow"); err == nil {
		t.Fatal("Cancel accepted a stale flow id")
	}
	signInMu.Lock()
	active := signInFlows[testFlowProviderID]
	signInMu.Unlock()
	if active.id != started.FlowID {
		t.Fatal("stale operation replaced or removed the active flow")
	}
	if err := svc.SignIn().Cancel(testFlowProviderID, started.FlowID); err != nil {
		t.Fatalf("Cancel(active flow): %v", err)
	}
}

func TestSignInCancelledFlowCannotCommitAfterReplacement(t *testing.T) {
	oldStart := startCursorLogin
	t.Cleanup(func() {
		startCursorLogin = oldStart
		clearTestSignInFlow("cursor")
	})
	firstWaitStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	startCount := 0
	startCursorLogin = func(context.Context) (*cursorSignIn, error) {
		startCount++
		if startCount == 1 {
			return &cursorSignIn{
				verificationURL: "https://cursor.com/oauth/authorize",
				wait: func(context.Context) error {
					close(firstWaitStarted)
					<-releaseFirst
					return nil
				},
			}, nil
		}
		return &cursorSignIn{
			verificationURL: "https://cursor.com/oauth/authorize",
			wait: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
		}, nil
	}
	stubModelsDevFetch(t, nil)
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"codexbar\"\n\n[auth]\nuse_keychain = false\n\n[providers.cursor]\nenabled = true\n"))

	first, err := svc.SignIn().Start(context.Background(), "cursor")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- svc.SignIn().Confirm(context.Background(), "cursor", first.FlowID, "Stale")
	}()
	<-firstWaitStarted
	if err := svc.SignIn().Cancel("cursor", first.FlowID); err != nil {
		t.Fatal(err)
	}
	replacement, err := svc.SignIn().Start(context.Background(), "cursor")
	if err != nil {
		t.Fatal(err)
	}
	close(releaseFirst)
	if err := <-done; err == nil {
		t.Fatal("cancelled Confirm committed after a replacement Start")
	}
	detail, err := svc.Providers().Detail(context.Background(), "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Accounts) != 0 {
		t.Fatalf("cancelled Confirm recorded accounts: %+v", detail.Accounts)
	}
	if err := svc.SignIn().Cancel("cursor", replacement.FlowID); err != nil {
		t.Fatal(err)
	}
}

func TestCursorOAuthSignInUsesProviderClientAndRecordsAccount(t *testing.T) {
	oldStart := startCursorLogin
	t.Cleanup(func() {
		startCursorLogin = oldStart
		clearTestSignInFlow("cursor")
	})
	waited := false
	startCursorLogin = func(context.Context) (*cursorSignIn, error) {
		return &cursorSignIn{
			verificationURL: "https://cursor.com/oauth/authorize",
			wait: func(context.Context) error {
				waited = true
				return nil
			},
		}, nil
	}
	stubModelsDevFetch(t, nil)
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"codexbar\"\n\n[auth]\nuse_keychain = false\n\n[providers.cursor]\nenabled = true\n"))

	started, err := svc.SignIn().Start(context.Background(), "cursor")
	if err != nil {
		t.Fatalf("Start(cursor): %v", err)
	}
	if started.VerificationURI != "https://cursor.com/oauth/authorize" ||
		started.UserCode != "" ||
		started.PasteRequired {
		t.Fatalf("Start(cursor) = %+v", started)
	}
	if err := svc.SignIn().Confirm(context.Background(), "cursor", started.FlowID, "Work"); err != nil {
		t.Fatalf("Confirm(cursor): %v", err)
	}
	if !waited {
		t.Fatal("Confirm(cursor) did not wait for Cursor Agent")
	}
	detail, err := svc.Providers().Detail(context.Background(), "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !detail.OAuthSupported || len(detail.Accounts) != 1 || detail.Accounts[0].Ref != cursorOAuthRef {
		t.Fatalf("cursor detail after sign-in = %+v", detail)
	}
	store, err := svc.managedStoreFor("cursor")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Resolve(context.Background(), "cursor"); err == nil {
		t.Fatal("Cursor sign-in wrote a sentinel to the managed credential store")
	}
}

func TestAntigravityOAuthSignInStoresCredentialAndRecordsAccount(t *testing.T) {
	oldStart := startAntigravityLogin
	t.Cleanup(func() {
		startAntigravityLogin = oldStart
		clearTestSignInFlow("antigravity")
	})
	startAntigravityLogin = func(context.Context) (*antigravitySignIn, error) {
		return &antigravitySignIn{
			verificationURL: "https://accounts.google.com/o/oauth2/v2/auth?state=test",
			wait: func(context.Context) (string, error) {
				return "antigravity-oauth.test-credential", nil
			},
		}, nil
	}
	stubModelsDevFetch(t, nil)
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"codexbar\"\n\n[auth]\nuse_keychain = false\n\n[providers.antigravity]\nenabled = true\n"))

	started, err := svc.SignIn().Start(context.Background(), "antigravity")
	if err != nil {
		t.Fatalf("Start(antigravity): %v", err)
	}
	if !strings.HasPrefix(started.VerificationURI, "https://accounts.google.com/") ||
		started.PasteRequired {
		t.Fatalf("Start(antigravity) = %+v", started)
	}
	if err := svc.SignIn().Confirm(context.Background(), "antigravity", started.FlowID, "Google"); err != nil {
		t.Fatalf("Confirm(antigravity): %v", err)
	}
	store, err := svc.managedStoreFor("antigravity")
	if err != nil {
		t.Fatal(err)
	}
	credential, _, err := store.Resolve(context.Background(), "antigravity")
	if err != nil {
		t.Fatal(err)
	}
	if credential.Token != "antigravity-oauth.test-credential" {
		t.Fatal("Confirm(antigravity) did not persist the OAuth credential")
	}
	detail, err := svc.Providers().Detail(context.Background(), "antigravity")
	if err != nil {
		t.Fatal(err)
	}
	if !detail.OAuthSupported || len(detail.Accounts) != 1 || detail.Accounts[0].Ref != managedOAuthRef {
		t.Fatalf("antigravity detail after sign-in = %+v", detail)
	}
}

func TestSignInConfirmSavesCredential(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	stubModelsDevFetch(t, []modelsdev.ProviderModel{{
		Provider:     testFlowProviderID,
		ModelID:      "claude-opus-5",
		Name:         "Claude Opus 5",
		EffortLevels: []string{"max"},
	}})
	svc, rec := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[auth]\nuse_keychain = false\n\n[providers."+testFlowProviderID+"]\nenabled = true\n\n[[providers."+testFlowProviderID+".accounts]]\nname = \"GitHub\"\nkind = \"oauth\"\nref = \"\"\n"))
	stubCatalogRepoFromCache(t, svc)
	tokenBodies := make(chan string, 8)
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)
	tokenBodies <- `{"access_token":"tok-confirm-1","token_type":"bearer"}`

	started, err := svc.SignIn().Start(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := svc.SignIn().Confirm(context.Background(), testFlowProviderID, started.FlowID, "GitHub"); err != nil {
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
	if len(detail.Models) == 0 {
		t.Fatal("Detail.Models is empty after Confirm, want auto-refreshed catalogue routes")
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
	if err := svc.SignIn().Confirm(context.Background(), testFlowProviderID, started.FlowID, "GitHub"); err == nil {
		t.Fatal("second Confirm error = nil, want validation failure")
	}
}

func TestSignInSaveAPIKeyStoresSecretOutsideConfig(t *testing.T) {
	const secret = "sk-managed-canary-123"
	svc, rec := newTestServices(t, WithConfigTOML("[auth]\nuse_keychain = false\n\n[providers.claude]\nenabled = true\n"))

	if err := svc.SignIn().SaveAPIKey(context.Background(), "claude", "Production", secret); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	detail, err := svc.Providers().Detail(context.Background(), "claude")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	wantAccounts := []ProviderAccountDTO{{
		Name: "Production",
		Kind: AccountKindToken,
		Ref:  managedOAuthRef,
	}}
	if !reflect.DeepEqual(detail.Accounts, wantAccounts) {
		t.Fatalf("accounts = %+v, want %+v", detail.Accounts, wantAccounts)
	}
	data, err := os.ReadFile(svc.paths.UserConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatal("config.toml contains the API key")
	}
	store, err := svc.managedStoreFor("claude")
	if err != nil {
		t.Fatal(err)
	}
	cred, _, err := store.Resolve(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != secret || cred.Source != usage.AuthEnvVar {
		t.Fatalf("stored credential = %s", cred.String())
	}
	var sawConfig, sawUsage bool
	for _, event := range rec.Events() {
		sawConfig = sawConfig || event.Event == EventConfigChanged
		sawUsage = sawUsage || event.Event == EventUsageUpdated
	}
	if !sawConfig || !sawUsage {
		t.Fatalf("events = %+v, want config and usage invalidation", rec.Events())
	}
}

func blockSignInConfigWrite(t *testing.T, svc *Services) {
	t.Helper()
	original := svc.paths.UserConfigFile
	blocker := filepath.Join(t.TempDir(), "config-target")
	if err := os.Mkdir(blocker, 0o700); err != nil {
		t.Fatal(err)
	}
	svc.paths.UserConfigFile = blocker
	t.Cleanup(func() { svc.paths.UserConfigFile = original })
}

func TestSignInConfirmRestoresCredentialWhenConfigWriteFails(t *testing.T) {
	oldStart := startAntigravityLogin
	t.Cleanup(func() {
		startAntigravityLogin = oldStart
		clearTestSignInFlow("antigravity")
	})
	startAntigravityLogin = func(context.Context) (*antigravitySignIn, error) {
		return &antigravitySignIn{
			verificationURL: "https://accounts.google.com/o/oauth2/v2/auth?state=test",
			wait: func(context.Context) (string, error) {
				return "replacement-managed-token", nil
			},
		}, nil
	}
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"codexbar\"\n\n[auth]\nuse_keychain = false\n\n[providers.antigravity]\nenabled = true\n"))
	store, err := svc.managedStoreFor("antigravity")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("antigravity", "original-managed-token"); err != nil {
		t.Fatal(err)
	}
	started, err := svc.SignIn().Start(context.Background(), "antigravity")
	if err != nil {
		t.Fatal(err)
	}
	blockSignInConfigWrite(t, svc)

	if err := svc.SignIn().Confirm(context.Background(), "antigravity", started.FlowID, "Google"); err == nil {
		t.Fatal("Confirm error = nil, want config persistence failure")
	}
	restored, _, err := store.Resolve(context.Background(), "antigravity")
	if err != nil || restored.Token != "original-managed-token" {
		t.Fatalf("credential after failed Confirm = %s, %v", restored.String(), err)
	}
}

func TestSignInSaveAPIKeyRestoresCredentialWhenConfigWriteFails(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[auth]\nuse_keychain = false\n\n[providers.claude]\nenabled = true\n"))
	store, err := svc.managedStoreFor("claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("claude", "original-managed-token"); err != nil {
		t.Fatal(err)
	}
	blockSignInConfigWrite(t, svc)

	if err := svc.SignIn().SaveAPIKey(context.Background(), "claude", "Production", "replacement-api-key"); err == nil {
		t.Fatal("SaveAPIKey error = nil, want config persistence failure")
	}
	restored, _, err := store.Resolve(context.Background(), "claude")
	if err != nil || restored.Token != "original-managed-token" ||
		restored.Source != usage.AuthOAuthDeviceFlow {
		t.Fatalf("credential after failed SaveAPIKey = %s, %v", restored.String(), err)
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

// noFlowProviderID is a registered provider WITHOUT a device-flow source and
// without a Claude/Codex login path — Start still refuses it.
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
	err := svc.SignIn().Confirm(context.Background(), testFlowProviderID, "missing-flow", "Account")
	if err == nil {
		t.Fatal("Confirm without Start error = nil, want error")
	}
	if !strings.Contains(err.Error(), "no matching sign-in in progress") {
		t.Fatalf("err = %v, want no-matching-flow message", err)
	}
}

func TestSignInCancelClearsFlow(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	tokenBodies := make(chan string, 8)
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)

	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+testFlowProviderID+"]\nenabled = true\n"))
	started, err := svc.SignIn().Start(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SignIn().Cancel(testFlowProviderID, started.FlowID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	signInMu.Lock()
	_, active := signInFlows[testFlowProviderID]
	signInMu.Unlock()
	if active {
		t.Fatal("flow still active after Cancel")
	}
	// Cancelling again is success.
	if err := svc.SignIn().Cancel(testFlowProviderID, started.FlowID); err != nil {
		t.Fatalf("double Cancel error = %v", err)
	}
}

func TestSignInCancelAbortsConfirm(t *testing.T) {
	registerTestDeviceFlowProvider(t)
	tokenBodies := make(chan string)
	t.Cleanup(func() { close(tokenBodies) })
	newFlowTestTargets(t, `{"device_code":"abc123","user_code":"WDML-TEST","verification_uri":"https://github.com/login/device","expires_in":900}`, tokenBodies)

	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[providers."+testFlowProviderID+"]\nenabled = true\n"))
	started, err := svc.SignIn().Start(context.Background(), testFlowProviderID)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- svc.SignIn().Confirm(context.Background(), testFlowProviderID, started.FlowID, "Account")
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
	if err := svc.SignIn().Cancel(testFlowProviderID, started.FlowID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	err = <-done
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
		t.Fatalf("Version = %q, want 2.1.0", got.Version)
	}
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

func TestSignInClaudeSubmitCodeThenConfirm(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stubModelsDevFetch(t, []modelsdev.ProviderModel{})
	persistClaudeLogin = func(claude.Tokens) error { return nil }
	t.Cleanup(func() { persistClaudeLogin = claude.PersistLogin })

	var login *claude.BrowserLogin
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"claude-gui-token","refresh_token":"r","expires_in":60}`))
	}))
	t.Cleanup(srv.Close)
	startClaudeLogin = func() (*claude.BrowserLogin, error) {
		var err error
		login, err = claude.StartBrowserLogin()
		if err != nil {
			return nil, err
		}
		login.TokenURL = srv.URL
		login.HTTP = srv.Client()
		return login, nil
	}
	t.Cleanup(func() { startClaudeLogin = func() (*claude.BrowserLogin, error) { return claude.StartBrowserLogin() } })

	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[auth]\nuse_keychain = false\n\n[providers.claude]\nenabled = true\n"))
	stubCatalogRepoFromCache(t, svc)
	started, err := svc.SignIn().Start(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if started.UserCode != "" ||
		!started.PasteRequired ||
		!strings.Contains(started.VerificationURI, "oauth/authorize") {
		t.Fatalf("Start = %+v, want authorize URL with pasted-code completion", started)
	}
	done := make(chan error, 1)
	go func() {
		done <- svc.SignIn().Confirm(context.Background(), "claude", started.FlowID, "Claude")
	}()
	if login == nil {
		t.Fatal("claude login was not started")
	}
	if err := svc.SignIn().SubmitCode("claude", started.FlowID, "the-code#"+login.State); err != nil {
		t.Fatalf("SubmitCode: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	store, err := svc.managedStoreFor("claude")
	if err != nil {
		t.Fatal(err)
	}
	cred, _, err := store.Resolve(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "claude-gui-token" {
		t.Fatalf("stored token = %q", redactConst(cred.Token))
	}
}

func TestSignInCodexDeviceSavesCredential(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stubModelsDevFetch(t, []modelsdev.ProviderModel{})
	persistCodexLogin = func(codex.Tokens) error { return nil }
	t.Cleanup(func() { persistCodexLogin = codex.PersistLogin })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/deviceauth/usercode"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"device_auth_id":"dev-1","user_code":"WDML-CDX","interval":"1"}`))
		case strings.HasSuffix(r.URL.Path, "/deviceauth/token"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"authorization_code":"ac","code_challenge":"ch","code_verifier":"ver"}`))
		case strings.HasSuffix(r.URL.Path, "/oauth/token"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"codex-gui-token","refresh_token":"rr"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	startCodexLogin = func(ctx context.Context) (*codex.DeviceLogin, error) {
		login, err := codex.StartDeviceLogin(ctx, srv.URL, "test-client", srv.Client())
		if err != nil {
			return nil, err
		}
		login.Sleep = func(time.Duration) {}
		login.Interval = time.Millisecond
		return login, nil
	}
	t.Cleanup(func() {
		startCodexLogin = func(ctx context.Context) (*codex.DeviceLogin, error) {
			return codex.StartDeviceLogin(ctx, codex.Issuer, codex.ClientID, nil)
		}
	})

	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n\n[auth]\nuse_keychain = false\n\n[providers.codex]\nenabled = true\n"))
	stubCatalogRepoFromCache(t, svc)
	started, err := svc.SignIn().Start(context.Background(), "codex")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.UserCode != "WDML-CDX" ||
		started.PasteRequired ||
		!strings.Contains(started.VerificationURI, "/codex/device") {
		t.Fatalf("Start = %+v", started)
	}
	if err := svc.SignIn().Confirm(context.Background(), "codex", started.FlowID, "Codex"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	store, err := svc.managedStoreFor("codex")
	if err != nil {
		t.Fatal(err)
	}
	cred, _, err := store.Resolve(context.Background(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "codex-gui-token" {
		t.Fatalf("stored token = %q", redactConst(cred.Token))
	}
}
