//go:build !nousage

package codex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

const canaryBodyMarker = "canary-body-marker-42"

// mustBeCanaryFree runs fn and fails the test when its error text (or the
// Fetch Failure message, which fn must surface as an error) contains any
// canary value.
func mustBeCanaryFree(t *testing.T, canaries []string, fn func() error) {
	t.Helper()
	err := fn()
	if err == nil {
		return
	}
	for _, c := range canaries {
		if strings.Contains(err.Error(), c) {
			t.Fatalf("canary %q leaked into error text: %v", c, err)
		}
	}
}

// TestCanaryCredentialAndOriginPaths mirrors the .mjs assertion that output
// never matches /canary|acct-synthetic/ (usage-allowance.test.mjs test 6) and
// the global SPEC §6 item 5 invariant.
func TestCanaryCredentialAndOriginPaths(t *testing.T) {
	canaries := []string{canaryToken, canaryAcct, canaryBodyMarker, "canary.example"}

	// (a) canary token/account ID with a 401 whose body also carries the
	// canary: the Failure text is free of every canary.
	t.Run("unauthorized body free of canaries", func(t *testing.T) {
		writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
		stub := &stubTransport{}
		stub.fn = func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 401,
				Status:     "401 Unauthorized",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"error":"`+canaryBodyMarker+`"}`)),
				Request:    req,
			}, nil
		}
		mustBeCanaryFree(t, canaries, func() error {
			return security.WithCanary(canaryToken, func() error {
				snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
				if err != nil {
					return err
				}
				if snap.Failure == nil {
					return errors.New("no failure")
				}
				return errors.New(snap.Failure.Message)
			})
		})
	})

	// (b) canary token with a transport error that embeds the token: the
	// mapped network Failure never echoes it.
	t.Run("network error free of canary", func(t *testing.T) {
		writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
		stub := &stubTransport{}
		stub.fn = func(*http.Request) (*http.Response, error) {
			return nil, errors.New("boom " + canaryToken)
		}
		mustBeCanaryFree(t, canaries, func() error {
			return security.WithCanary(canaryToken, func() error {
				snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
				if err != nil {
					return err
				}
				if snap.Failure == nil || snap.Failure.Code != "network" {
					return errors.New("no network failure")
				}
				return errors.New(snap.Failure.Message)
			})
		})
	})

	// (c) a 200 body carrying the marker: normalized windows and the
	// Snapshot never contain it.
	t.Run("response marker absent from windows", func(t *testing.T) {
		writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
		body := `{"rate_limit":{"primary_window":{"used_percent":20,"reset_at":1900000000}},"extra":"` + canaryBodyMarker + `"}`
		stub := &stubTransport{fn: canned(200, body)}
		snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
		if err != nil {
			t.Fatalf("Fetch() error: %v", err)
		}
		out, _ := json.Marshal(snap)
		if strings.Contains(string(out), canaryBodyMarker) {
			t.Fatalf("response marker leaked into snapshot: %s", out)
		}
	})

	// (d) successful fetch with the canary account ID: the ID appears only in
	// the request header, never in the Snapshot.
	t.Run("account id request-header only", func(t *testing.T) {
		writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
		stub := &stubTransport{fn: canned(200, fixtureCase6)}
		snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
		if err != nil {
			t.Fatalf("Fetch() error: %v", err)
		}
		if len(stub.reqs) != 1 || stub.reqs[0].Header.Get("ChatGPT-Account-Id") != canaryAcct {
			t.Fatalf("account ID missing from request header: %v", stub.reqs)
		}
		if snap.Account != "" {
			t.Errorf("Snapshot.Account = %q, want unset", snap.Account)
		}
		out, _ := json.Marshal(snap)
		if strings.Contains(string(out), canaryAcct) || strings.Contains(string(out), canaryToken) {
			t.Fatalf("credential material leaked into snapshot: %s", out)
		}
	})

	// (e) untrusted_origin: the verbatim message never echoes the origin.
	t.Run("untrusted origin never echoed", func(t *testing.T) {
		writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"},"base_url":"https://canary.example/v1"}`)
		stub := &stubTransport{fn: canned(404, `{}`)}
		snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
		if err != nil {
			t.Fatalf("Fetch() error: %v", err)
		}
		if snap.Failure == nil || snap.Failure.Code != "untrusted_origin" {
			t.Fatalf("Failure = %+v, want untrusted_origin", snap.Failure)
		}
		if snap.Failure.Message != "The configured Codex fallback origin was not explicitly trusted." {
			t.Errorf("Message = %q, want the verbatim untrusted_origin message", snap.Failure.Message)
		}
		if strings.Contains(snap.Failure.Message, "canary.example") {
			t.Fatalf("origin echoed in error: %q", snap.Failure.Message)
		}
	})

	// (f) fallback_unavailable path with a canary credential: verbatim
	// message, free of canaries.
	t.Run("fallback unavailable free of canaries", func(t *testing.T) {
		writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
		stub := &stubTransport{fn: canned(404, `{}`)}
		mustBeCanaryFree(t, canaries, func() error {
			return security.WithCanary(canaryToken, func() error {
				snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
				if err != nil {
					return err
				}
				if snap.Failure == nil {
					return errors.New("no failure")
				}
				if snap.Failure.Message != "Codex did not advertise a configured fallback endpoint." {
					t.Errorf("Message = %q, want the verbatim fallback_unavailable message", snap.Failure.Message)
				}
				return errors.New(snap.Failure.Message)
			})
		})
	})

	// Full success path: the whole Snapshot is free of every canary (.mjs
	// test 6 parity: output does not match /canary|acct-synthetic/).
	t.Run("success snapshot canary free", func(t *testing.T) {
		writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"},"base_url":"https://trusted.example/v1"}`)
		stub := &stubTransport{fn: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == UsageURL {
				return canned(404, `{}`)(req)
			}
			return canned(200, fixtureCase6)(req)
		}}
		ctx := WithTrustedOrigin(context.Background(), "https://trusted.example")
		snap, err := Fetch(ctx, usage.Credential{}, &http.Client{Transport: stub})
		if err != nil {
			t.Fatalf("Fetch() error: %v", err)
		}
		if snap.Failure != nil {
			t.Fatalf("unexpected Failure: %+v", snap.Failure)
		}
		out, _ := json.Marshal(snap)
		for _, c := range canaries {
			if strings.Contains(string(out), c) {
				t.Fatalf("canary %q leaked into snapshot: %s", c, out)
			}
		}
	})
}
