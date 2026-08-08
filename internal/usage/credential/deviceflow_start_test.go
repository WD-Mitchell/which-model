//go:build !nousage

package credential

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestDeviceFlowStart(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"
	const verURI = "https://github.com/login/device"

	newServer := func(t *testing.T, body string, status int) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(body))
		}))
	}

	specFor := func(t *testing.T, serverURL string) usage.OAuthSpec {
		t.Helper()
		return usage.OAuthSpec{
			ClientID:        "client-1",
			Scope:           "read:user",
			DeviceCodeURL:   serverURL,
			VerificationURI: verURI,
		}
	}

	t.Run("happy path", func(t *testing.T) { // case 1
		srv := newServer(t, `{"device_code":"abc123","user_code":"ABCD-EFGH","verification_uri":"`+verURI+`","expires_in":900}`, 200)
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		code, err := flow.Start(context.Background())
		if err != nil {
			t.Fatalf("Start() error = %v, want nil", err)
		}
		if code.DeviceCode != "abc123" || code.UserCode != "ABCD-EFGH" || code.VerificationURI != verURI {
			t.Fatalf("Start() = %+v, want populated device code", code)
		}
		if code.ExpiresIn != 900*time.Second {
			t.Fatalf("Start() ExpiresIn = %v, want 900s", code.ExpiresIn)
		}
		if code.Interval != 5*time.Second {
			t.Fatalf("Start() Interval = %v, want default 5s", code.Interval)
		}
	})

	t.Run("bad user code", func(t *testing.T) { // case 2
		srv := newServer(t, `{"device_code":"abc123","user_code":"x!","verification_uri":"`+verURI+`","expires_in":900}`, 200)
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		_, err := flow.Start(context.Background())
		if code := failureCode(t, err); code != "unsupported_response" {
			t.Fatalf("Start() error code = %q, want unsupported_response", code)
		}
	})

	t.Run("verification uri mismatch", func(t *testing.T) { // case 3
		srv := newServer(t, `{"device_code":"abc123","user_code":"ABCD-EFGH","verification_uri":"https://evil.example","expires_in":900}`, 200)
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		_, err := flow.Start(context.Background())
		if code := failureCode(t, err); code != "unsupported_response" {
			t.Fatalf("Start() error code = %q, want unsupported_response", code)
		}
	})

	t.Run("control char device code", func(t *testing.T) { // case 4
		srv := newServer(t, `{"device_code":"bad\ncode","user_code":"ABCD-EFGH","verification_uri":"`+verURI+`","expires_in":900}`, 200)
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		_, err := flow.Start(context.Background())
		if code := failureCode(t, err); code != "unsupported_response" {
			t.Fatalf("Start() error code = %q, want unsupported_response", code)
		}
	})

	t.Run("server 500", func(t *testing.T) { // case 5
		srv := newServer(t, `oops`, 500)
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		_, err := flow.Start(context.Background())
		if code := failureCode(t, err); code != "provider_status" {
			t.Fatalf("Start() error code = %q, want provider_status", code)
		}
	})

	t.Run("redirect refused", func(t *testing.T) { // case 6
		hits := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			http.Redirect(w, r, "https://elsewhere.example/", http.StatusFound)
		}))
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		_, err := flow.Start(context.Background())
		if code := failureCode(t, err); code != "redirect_refused" {
			t.Fatalf("Start() error code = %q, want redirect_refused", code)
		}
		if hits != 1 {
			t.Fatalf("Start() followed redirect: %d requests, want 1", hits)
		}
	})

	t.Run("expires in zero", func(t *testing.T) { // case 7
		srv := newServer(t, `{"device_code":"abc123","user_code":"ABCD-EFGH","verification_uri":"`+verURI+`","expires_in":0}`, 200)
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		_, err := flow.Start(context.Background())
		if code := failureCode(t, err); code != "unsupported_response" {
			t.Fatalf("Start() error code = %q, want unsupported_response", code)
		}
	})

	t.Run("canary device code", func(t *testing.T) { // case 8
		srv := newServer(t, `{"device_code":"`+canary+`\n","user_code":"ABCD-EFGH","verification_uri":"`+verURI+`","expires_in":900}`, 200)
		defer srv.Close()
		flow := NewDeviceFlow(specFor(t, srv.URL))
		flow.ValidateURL = func(string) error { return nil }
		_, err := flow.Start(context.Background())
		if code := failureCode(t, err); code != "unsupported_response" {
			t.Fatalf("Start() error code = %q, want unsupported_response", code)
		}
		if strings.Contains(err.Error(), canary) {
			t.Fatalf("error %q leaks canary", err)
		}
	})

	t.Run("empty spec panics", func(t *testing.T) { // case 9
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("NewDeviceFlow(OAuthSpec{}) did not panic")
			}
			msg, ok := r.(string)
			if !ok || !strings.Contains(msg, "VerificationURI") {
				t.Fatalf("panic = %v, want it to mention VerificationURI", r)
			}
		}()
		NewDeviceFlow(usage.OAuthSpec{})
	})
}
