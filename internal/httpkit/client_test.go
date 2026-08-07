package httpkit

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	c := NewClient()
	if c.timeout != DefaultTimeout {
		t.Errorf("timeout = %v, want %v", c.timeout, DefaultTimeout)
	}
	if c.maxBytes != DefaultMaxResponseBytes {
		t.Errorf("maxBytes = %d, want %d", c.maxBytes, DefaultMaxResponseBytes)
	}
	if c.retries != 1 {
		t.Errorf("retries = %d, want 1", c.retries)
	}
	if c.backoff != 500*time.Millisecond {
		t.Errorf("backoff = %v, want 500ms", c.backoff)
	}
	if c.userAgent != "which-model/dev" {
		t.Errorf("userAgent = %q, want %q", c.userAgent, "which-model/dev")
	}
}

func TestNewClientOptions(t *testing.T) {
	c := NewClient(WithTimeout(3 * time.Second))
	if c.timeout != 3*time.Second {
		t.Errorf("timeout = %v, want 3s", c.timeout)
	}
	if c.maxBytes != DefaultMaxResponseBytes {
		t.Errorf("maxBytes = %d, want default %d", c.maxBytes, DefaultMaxResponseBytes)
	}

	c = NewClient(WithMaxBytes(100))
	if c.maxBytes != 100 {
		t.Errorf("maxBytes = %d, want 100", c.maxBytes)
	}

	c = NewClient(WithUserAgent("wm/9"))
	if c.userAgent != "wm/9" {
		t.Errorf("userAgent = %q, want wm/9", c.userAgent)
	}

	c = NewClient(WithRetries(0))
	if c.retries != 0 {
		t.Errorf("retries = %d, want 0", c.retries)
	}
}

func TestSetAllowListCopies(t *testing.T) {
	c := NewClient()
	urls := []string{"a", "b"}
	c.SetAllowList(urls)
	urls[0] = "mutated"
	if len(c.allowed) != 2 || c.allowed[0] != "a" || c.allowed[1] != "b" {
		t.Errorf("allowed = %v, want [a b] (must copy, not alias)", c.allowed)
	}
}

func TestErrorMessages(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		want string
	}{
		{"timeout", &Error{Code: "timeout"}, "timeout: the provider request timed out"},
		{"redirect", &Error{Code: "redirect_refused"}, "redirect_refused: the provider attempted an unsafe redirect"},
		{"unknown", &Error{Code: "bogus"}, "bogus: the request failed"},
		{"sanitized", &Error{Code: "network", Err: errors.New("SECRET_XYZ")}, "network: the provider request failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "SECRET_XYZ") {
				t.Errorf("Error() = %q leaks the underlying Err text", got)
			}
		})
	}
}

func TestAsError(t *testing.T) {
	if e, ok := AsError(errors.New("plain")); ok || e != nil {
		t.Errorf("AsError(plain) = (%v, %v), want (nil, false)", e, ok)
	}
	wrapped := fmt.Errorf("wrap: %w", &Error{Code: "network"})
	e, ok := AsError(wrapped)
	if !ok || e == nil || e.Code != "network" {
		t.Errorf("AsError(wrapped) = (%v, %v), want (network, true)", e, ok)
	}
}
