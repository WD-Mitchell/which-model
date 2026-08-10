//go:build !nousage

package copilot

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeClock mirrors the .mjs test clock: now starts at zero; sleep advances
// it and records the wait.
type fakeClock struct {
	now   time.Time
	waits []time.Duration
}

func (c *fakeClock) Now() time.Time        { return c.now }
func (c *fakeClock) Sleep(d time.Duration) { c.now = c.now.Add(d); c.waits = append(c.waits, d) }

func TestStartDeviceFlow(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		want     DeviceFlow
		wantCode string
		wantMsg  string
	}{
		{
			name:   "case 1: valid response",
			status: http.StatusOK,
			body:   `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":60,"interval":1}`,
			want: DeviceFlow{
				DeviceCode:      "canary-device-code-456",
				UserCode:        "ABCD-1234",
				VerificationURI: "https://github.com/login/device",
				ExpiresIn:       60,
				Interval:        1,
			},
		},
		{
			name:     "case 2a: user_code too short",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ab","verification_uri":"https://github.com/login/device","expires_in":60,"interval":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 2b: user_code invalid chars",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"bad code!","verification_uri":"https://github.com/login/device","expires_in":60,"interval":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 3a: verification_uri trailing slash",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device/","expires_in":60,"interval":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 3b: verification_uri wrong host",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://evil.example/","expires_in":60,"interval":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 4a: expires_in 0",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":0,"interval":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 4b: expires_in 1801",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":1801,"interval":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 4c: interval 0",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":60,"interval":0}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 4d: interval 31",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":60,"interval":31}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 4e: expires_in missing",
			status:   http.StatusOK,
			body:     `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","interval":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub returned an unsupported device-login response.",
		},
		{
			name:     "case 5: 401",
			status:   http.StatusUnauthorized,
			body:     `{"message":"nope"}`,
			wantCode: "unauthorized",
			wantMsg:  "GitHub device login rejected the credential.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubTransport{queue: []stubResponse{{status: tc.status, body: tc.body}}}
			client := &http.Client{Transport: stub}

			flow, err := StartDeviceFlow(context.Background(), client)

			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("StartDeviceFlow = %+v, nil; want error %q", flow, tc.wantCode)
				}
				pe, ok := err.(*Error)
				if !ok {
					t.Fatalf("error type = %T, want *copilot.Error", err)
				}
				if pe.Code != tc.wantCode {
					t.Errorf("code = %q, want %q", pe.Code, tc.wantCode)
				}
				if pe.Message != tc.wantMsg {
					t.Errorf("message = %q, want %q", pe.Message, tc.wantMsg)
				}
				if strings.Contains(err.Error(), "canary-device-code-456") {
					t.Errorf("error %q leaks the device code", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("StartDeviceFlow = %v", err)
			}
			if flow != tc.want {
				t.Errorf("flow = %+v\nwant  %+v", flow, tc.want)
			}

			// Request assertions (case 1): POST device-code URL, form body.
			stub.mu.Lock()
			defer stub.mu.Unlock()
			if len(stub.requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(stub.requests))
			}
			req := stub.requests[0]
			if req.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", req.Method)
			}
			if req.URL.String() != GitHubDeviceCodeURL {
				t.Errorf("URL = %s, want %s", req.URL.String(), GitHubDeviceCodeURL)
			}
			if got := req.Header.Get("Accept"); got != "application/json" {
				t.Errorf("Accept = %q, want %q", got, "application/json")
			}
			if got := req.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
				t.Errorf("Content-Type = %q, want %q", got, "application/x-www-form-urlencoded")
			}
			body := stub.bodies[0]
			if !strings.Contains(body, "client_id=Iv1.b507a08c87ecfe98") {
				t.Errorf("body %q missing client_id", body)
			}
			if !strings.Contains(body, "scope=read%3Auser") {
				t.Errorf("body %q missing scope=read%%3Auser", body)
			}
		})
	}
}

// TestStartDeviceFlowIntervalDefault pins interval's default of 5 when absent.
func TestStartDeviceFlowIntervalDefault(t *testing.T) {
	stub := &stubTransport{queue: []stubResponse{{status: http.StatusOK, body: `{"device_code":"canary-device-code-456","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":60}`}}}
	flow, err := StartDeviceFlow(context.Background(), &http.Client{Transport: stub})
	if err != nil {
		t.Fatalf("StartDeviceFlow = %v", err)
	}
	if flow.Interval != 5 {
		t.Errorf("Interval = %d, want 5 (default)", flow.Interval)
	}
}

func TestPollDeviceFlow(t *testing.T) {
	cases := []struct {
		name      string
		replies   []string
		expiresIn int
		interval  int
		wantToken string
		wantCode  string
		wantMsg   string
		wantWaits []time.Duration
		wantCalls int
	}{
		{
			// .mjs test 10 parity: pending → slow_down → token; waits [1000,1000,6000].
			name:      "case 6: pending + slow_down then token",
			replies:   []string{`{"error":"authorization_pending"}`, `{"error":"slow_down"}`, `{"access_token":"canary-secret-token-123"}`},
			expiresIn: 60,
			interval:  1,
			wantToken: "canary-secret-token-123",
			wantWaits: []time.Duration{1000 * time.Millisecond, 1000 * time.Millisecond, 6000 * time.Millisecond},
			wantCalls: 3,
		},
		{
			name:      "case 7: access_denied",
			replies:   []string{`{"error":"access_denied"}`},
			expiresIn: 60,
			interval:  1,
			wantCode:  "access_denied",
			wantMsg:   "GitHub device login was denied or cancelled.",
			wantWaits: []time.Duration{1000 * time.Millisecond},
			wantCalls: 1,
		},
		{
			name:      "case 8: expired_token",
			replies:   []string{`{"error":"expired_token"}`},
			expiresIn: 60,
			interval:  1,
			wantCode:  "device_expired",
			wantMsg:   "GitHub device login expired.",
			wantWaits: []time.Duration{1000 * time.Millisecond},
			wantCalls: 1,
		},
		{
			// .mjs test 12 parity: interval exceeds expiry; zero requests.
			name:      "case 9: never polls at/after the deadline",
			replies:   []string{`{"access_token":"canary-secret-token-123"}`},
			expiresIn: 5,
			interval:  10,
			wantCode:  "device_expired",
			wantMsg:   "GitHub device login expired.",
			wantWaits: []time.Duration{5000 * time.Millisecond},
			wantCalls: 0,
		},
		{
			// .mjs test 13 parity: repeated slow_down; waits [1000,6000,11000].
			name:      "case 10: repeated slow_down",
			replies:   []string{`{"error":"slow_down"}`, `{"error":"slow_down"}`, `{"access_token":"canary-secret-token-123"}`},
			expiresIn: 30,
			interval:  1,
			wantToken: "canary-secret-token-123",
			wantWaits: []time.Duration{1000 * time.Millisecond, 6000 * time.Millisecond, 11000 * time.Millisecond},
			wantCalls: 3,
		},
		{
			name:      "case 11: unknown error",
			replies:   []string{`{"error":"unknown_error"}`},
			expiresIn: 60,
			interval:  1,
			wantCode:  "unsupported_response",
			wantMsg:   "GitHub returned an unsupported device-login response.",
			wantWaits: []time.Duration{1000 * time.Millisecond},
			wantCalls: 1,
		},
		{
			// Deadline reached mid-wait: the third sleep clamps to the
			// remaining 1s (min(interval, remaining)), then the post-sleep
			// deadline check breaks BEFORE the third request (.mjs port).
			name:      "case 12: deadline reached while pending",
			replies:   []string{`{"error":"authorization_pending"}`, `{"error":"authorization_pending"}`, `{"error":"authorization_pending"}`},
			expiresIn: 3,
			interval:  1,
			wantCode:  "device_expired",
			wantMsg:   "GitHub device login expired.",
			wantWaits: []time.Duration{1000 * time.Millisecond, 1000 * time.Millisecond, 1000 * time.Millisecond},
			wantCalls: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubTransport{}
			for _, r := range tc.replies {
				stub.queue = append(stub.queue, stubResponse{status: http.StatusOK, body: r})
			}
			clock := &fakeClock{}
			opts := &PollOptions{Now: clock.Now, Sleep: clock.Sleep}

			token, err := PollDeviceFlow(context.Background(), &http.Client{Transport: stub}, DeviceFlow{
				DeviceCode: "canary-device-code-456",
				ExpiresIn:  tc.expiresIn,
				Interval:   tc.interval,
			}, opts)

			if tc.wantToken != "" {
				if err != nil {
					t.Fatalf("PollDeviceFlow = %q, %v; want token %q", token, err, tc.wantToken)
				}
				if token != tc.wantToken {
					t.Errorf("token = %q, want %q", token, tc.wantToken)
				}
			} else {
				if err == nil {
					t.Fatalf("PollDeviceFlow = %q, nil; want error %q", token, tc.wantCode)
				}
				pe, ok := err.(*Error)
				if !ok {
					t.Fatalf("error type = %T, want *copilot.Error", err)
				}
				if pe.Code != tc.wantCode {
					t.Errorf("code = %q, want %q", pe.Code, tc.wantCode)
				}
				if pe.Message != tc.wantMsg {
					t.Errorf("message = %q, want %q", pe.Message, tc.wantMsg)
				}
				if strings.Contains(err.Error(), "canary-device-code-456") || strings.Contains(err.Error(), "canary-secret-token-123") {
					t.Errorf("error %q leaks device code or token", err)
				}
			}

			if len(clock.waits) != len(tc.wantWaits) {
				t.Errorf("waits = %v, want %v", clock.waits, tc.wantWaits)
			} else {
				for i := range tc.wantWaits {
					if clock.waits[i] != tc.wantWaits[i] {
						t.Errorf("waits = %v, want %v", clock.waits, tc.wantWaits)
						break
					}
				}
			}
			stub.mu.Lock()
			calls := len(stub.requests)
			stub.mu.Unlock()
			if calls != tc.wantCalls {
				t.Errorf("HTTP calls = %d, want %d", calls, tc.wantCalls)
			}

			// Token POST body contract (grant_type; device_code; client_id).
			if calls > 0 {
				stub.mu.Lock()
				body := stub.bodies[len(stub.bodies)-1]
				stub.mu.Unlock()
				if !strings.Contains(body, "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code") {
					t.Errorf("token POST body %q missing grant_type", body)
				}
				if !strings.Contains(body, "device_code=canary-device-code-456") {
					t.Errorf("token POST body %q missing device_code", body)
				}
				if !strings.Contains(body, "client_id=Iv1.b507a08c87ecfe98") {
					t.Errorf("token POST body %q missing client_id", body)
				}
			}
		})
	}
}
