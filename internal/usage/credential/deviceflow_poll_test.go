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

// fakeDeviceClock is the Now/Sleep seam pair for the Poll loop.
type fakeDeviceClock struct {
	now   time.Time
	slept []time.Duration
}

func (c *fakeDeviceClock) nowFn() time.Time { return c.now }
func (c *fakeDeviceClock) sleepFn(d time.Duration) {
	c.slept = append(c.slept, d)
	c.now = c.now.Add(d)
}

func TestDeviceFlowPoll(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"
	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	// scripted server: each entry is one response; the raw body of the
	// last request is captured so the form can be asserted.
	newServer := func(t *testing.T, script []struct {
		status int
		body   string
	}, lastForm *string) *httptest.Server {
		t.Helper()
		i := 0
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			*lastForm = r.PostForm.Encode()
			if i >= len(script) {
				t.Fatalf("server received %d requests, script has %d", i+1, len(script))
			}
			w.WriteHeader(script[i].status)
			w.Write([]byte(script[i].body))
			i++
		}))
	}

	newFlow := func(t *testing.T, serverURL string, clock *fakeDeviceClock) *DeviceFlow {
		t.Helper()
		flow := NewDeviceFlow(usage.OAuthSpec{
			ClientID:        "client-1",
			ClientSecret:    "",
			TokenURL:        serverURL,
			DeviceCodeURL:   serverURL,
			VerificationURI: "https://github.com/login/device",
		})
		flow.ValidateURL = func(string) error { return nil }
		flow.Now = clock.nowFn
		flow.Sleep = clock.sleepFn
		return flow
	}

	code := DeviceCode{
		DeviceCode:      "abc123",
		UserCode:        "ABCD-EFGH",
		VerificationURI: "https://github.com/login/device",
		ExpiresIn:       5 * time.Minute,
		Interval:        5 * time.Second,
	}

	t.Run("immediate success", func(t *testing.T) { // case 1
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{200, `{"access_token":"gho_xyz","token_type":"bearer"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		token, err := flow.Poll(context.Background(), code)
		if err != nil {
			t.Fatalf("Poll() error = %v, want nil", err)
		}
		if token != "gho_xyz" {
			t.Fatalf("Poll() = %q, want gho_xyz", token)
		}
		if !strings.Contains(form, "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code") ||
			!strings.Contains(form, "device_code=abc123") || !strings.Contains(form, "client_id=client-1") {
			t.Fatalf("Poll() form = %q, missing required fields", form)
		}
		if len(clock.slept) != 0 {
			t.Fatalf("Poll() slept %v, want no sleep", clock.slept)
		}
	})

	t.Run("pending then success", func(t *testing.T) { // case 2
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{400, `{"error":"authorization_pending"}`}, {200, `{"access_token":"tok2"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		token, err := flow.Poll(context.Background(), code)
		if err != nil {
			t.Fatalf("Poll() error = %v, want nil", err)
		}
		if token != "tok2" {
			t.Fatalf("Poll() = %q, want tok2", token)
		}
		if len(clock.slept) != 1 || clock.slept[0] != 5*time.Second {
			t.Fatalf("Poll() slept %v, want [5s]", clock.slept)
		}
	})

	t.Run("github 200 pending then success", func(t *testing.T) {
		// GitHub Apps (and some OAuth app responses) return HTTP 200 with
		// error=authorization_pending instead of RFC 8628's 400. Treating
		// that as a token-body failure aborts the flow the moment the user
		// is asked to confirm — before GitHub has issued a token.
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{200, `{"error":"authorization_pending"}`}, {200, `{"access_token":"gho_ok"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		token, err := flow.Poll(context.Background(), code)
		if err != nil {
			t.Fatalf("Poll() error = %v, want nil", err)
		}
		if token != "gho_ok" {
			t.Fatalf("Poll() = %q, want gho_ok", token)
		}
		if len(clock.slept) != 1 || clock.slept[0] != 5*time.Second {
			t.Fatalf("Poll() slept %v, want [5s]", clock.slept)
		}
	})

	t.Run("github 200 slow_down then success", func(t *testing.T) {
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{200, `{"error":"slow_down"}`}, {200, `{"access_token":"gho_slow"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		token, err := flow.Poll(context.Background(), code)
		if err != nil {
			t.Fatalf("Poll() error = %v, want nil", err)
		}
		if token != "gho_slow" {
			t.Fatalf("Poll() = %q, want gho_slow", token)
		}
		if len(clock.slept) != 1 || clock.slept[0] != 10*time.Second {
			t.Fatalf("Poll() slept %v, want [10s]", clock.slept)
		}
	})

	t.Run("github 200 access_denied", func(t *testing.T) {
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{200, `{"error":"access_denied"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		_, err := flow.Poll(context.Background(), code)
		if code := failureCode(t, err); code != "access_denied" {
			t.Fatalf("Poll() error code = %q, want access_denied", code)
		}
	})

	t.Run("slow down then pending", func(t *testing.T) { // case 3
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{400, `{"error":"slow_down"}`}, {400, `{"error":"authorization_pending"}`}, {200, `{"access_token":"tok3"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		token, err := flow.Poll(context.Background(), code)
		if err != nil {
			t.Fatalf("Poll() error = %v, want nil", err)
		}
		if token != "tok3" {
			t.Fatalf("Poll() = %q, want tok3", token)
		}
		want := []time.Duration{10 * time.Second, 10 * time.Second} // slow_down: +5s, then pending sleeps current interval
		if len(clock.slept) != len(want) {
			t.Fatalf("Poll() slept %v, want %v", clock.slept, want)
		}
		for i := range want {
			if clock.slept[i] != want[i] {
				t.Fatalf("Poll() slept %v, want %v", clock.slept, want)
			}
		}
	})

	t.Run("access denied", func(t *testing.T) { // case 4
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{400, `{"error":"access_denied"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		_, err := flow.Poll(context.Background(), code)
		if code := failureCode(t, err); code != "access_denied" {
			t.Fatalf("Poll() error code = %q, want access_denied", code)
		}
	})

	t.Run("expired token", func(t *testing.T) { // case 5
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{400, `{"error":"expired_token"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		_, err := flow.Poll(context.Background(), code)
		if code := failureCode(t, err); code != "device_expired" {
			t.Fatalf("Poll() error code = %q, want device_expired", code)
		}
	})

	t.Run("unknown error", func(t *testing.T) { // case 6
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{400, `{"error":"foo","error_description":"mystery"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		_, err := flow.Poll(context.Background(), code)
		if code := failureCode(t, err); code != "unsupported_response" {
			t.Fatalf("Poll() error code = %q, want unsupported_response", code)
		}
	})

	t.Run("server 500", func(t *testing.T) { // case 7
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{500, `oops`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		_, err := flow.Poll(context.Background(), code)
		if code := failureCode(t, err); code != "provider_status" {
			t.Fatalf("Poll() error code = %q, want provider_status", code)
		}
	})

	t.Run("canary token rejected", func(t *testing.T) { // case 8
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{200, `{"access_token":"` + canary + `\n"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		_, err := flow.Poll(context.Background(), code)
		if code := failureCode(t, err); code != "unsupported_response" {
			t.Fatalf("Poll() error code = %q, want unsupported_response", code)
		}
		if strings.Contains(err.Error(), canary) {
			t.Fatalf("error %q leaks canary", err)
		}
	})

	t.Run("expired before first request", func(t *testing.T) { // case 9
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{200, `{"access_token":"never"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := newFlow(t, srv.URL, clock)
		expired := code
		expired.ExpiresIn = -time.Minute // deadline is already in the past
		_, err := flow.Poll(context.Background(), expired)
		if code := failureCode(t, err); code != "device_expired" {
			t.Fatalf("Poll() error code = %q, want device_expired", code)
		}
		if form != "" {
			t.Fatalf("Poll() sent a request despite expiry (form %q)", form)
		}
	})

	t.Run("client secret appended", func(t *testing.T) { // case 10
		var form string
		srv := newServer(t, []struct {
			status int
			body   string
		}{{200, `{"access_token":"gho_sec"}`}}, &form)
		defer srv.Close()
		clock := &fakeDeviceClock{now: start}
		flow := NewDeviceFlow(usage.OAuthSpec{
			ClientID:        "client-1",
			ClientSecret:    "s3cr3t",
			TokenURL:        srv.URL,
			DeviceCodeURL:   srv.URL,
			VerificationURI: "https://github.com/login/device",
		})
		flow.ValidateURL = func(string) error { return nil }
		flow.Now = clock.nowFn
		flow.Sleep = clock.sleepFn
		if _, err := flow.Poll(context.Background(), code); err != nil {
			t.Fatalf("Poll() error = %v, want nil", err)
		}
		if !strings.Contains(form, "client_secret=s3cr3t") {
			t.Fatalf("Poll() form = %q, missing client_secret", form)
		}
	})
}
