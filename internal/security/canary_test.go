package security

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithCanary(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		if err := WithCanary("X", func() error { return nil }); err != nil {
			t.Fatalf("WithCanary() error = %v, want nil", err)
		}
	})

	t.Run("plain error stays unchanged", func(t *testing.T) {
		err := WithCanary("X", func() error { return errors.New("plain") })
		if err == nil || err.Error() != "plain" {
			t.Fatalf("WithCanary() error = %v, want plain", err)
		}
		if errors.Is(err, ErrCanaryLeak) {
			t.Fatal("WithCanary() returned ErrCanaryLeak for an error without the canary")
		}
	})

	t.Run("wrapped domain error stays unchanged", func(t *testing.T) {
		err := WithCanary("X", func() error {
			return fmt.Errorf("wrapped: %w", &Error{Code: "network", Message: "m"})
		})
		if err == nil || err.Error() != "wrapped: network: m" {
			t.Fatalf("WithCanary() error = %v, want wrapped domain error", err)
		}
		var domainErr *Error
		if !errors.As(err, &domainErr) || domainErr.Code != "network" {
			t.Fatalf("WithCanary() error = %v, want wrapped *Error{Code: network}", err)
		}
		if errors.Is(err, ErrCanaryLeak) {
			t.Fatal("WithCanary() returned ErrCanaryLeak for an error without the canary")
		}
	})

	t.Run("direct leak returns sentinel", func(t *testing.T) {
		err := WithCanary("X", func() error { return errors.New("prefix X suffix") })
		if !errors.Is(err, ErrCanaryLeak) {
			t.Fatalf("WithCanary() error = %v, want ErrCanaryLeak", err)
		}
	})

	t.Run("wrapped sentinel survives", func(t *testing.T) {
		err := WithCanary("X", func() error { return fmt.Errorf("wrapped: %w", ErrCanaryLeak) })
		if !errors.Is(err, ErrCanaryLeak) {
			t.Fatalf("WithCanary() error = %v, want wrapped ErrCanaryLeak", err)
		}
	})
}

func TestCanarySweep(t *testing.T) {
	const canary = "CANARY_9f2e17b4"

	dir := t.TempDir()
	canaryFile := filepath.Join(dir, "cred")
	if err := os.WriteFile(canaryFile, []byte(canary), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		wantCode string
		invoke   func() error
	}{
		{
			"unsafe token",
			"unsafe_credential",
			func() error { return ValidateOpaqueToken(canary + "\n") },
		},
		{
			"missing path",
			"credential_file",
			func() error { _, _, err := ReadBoundedFile(filepath.Join(dir, canary), MaxCredentialBytes); return err },
		},
		{
			"credential content",
			"credential_file",
			func() error { _, _, err := ReadBoundedFile(canaryFile, 1); return err },
		},
		{
			"endpoint URL",
			"endpoint_refused",
			func() error {
				_, err := ValidateExactHTTPS("https://"+canary+".example.com/", []string{"https://other.com"})
				return err
			},
		},
		{
			"base URL",
			"untrusted_origin",
			func() error {
				_, err := ValidateTrustedBaseURL("https://"+canary+".example.com/", "https://trusted.example.com")
				return err
			},
		},
		{
			"trusted origin",
			"untrusted_origin",
			func() error {
				_, err := ValidateTrustedBaseURL("https://trusted.example.com/", "https://"+canary)
				return err
			},
		},
		{
			"response body",
			"response_too_large",
			func() error {
				_, err := ReadResponseBounded(&http.Response{
					ContentLength: -1,
					Body:          io.NopCloser(strings.NewReader(canary + canary)),
				}, 2)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WithCanary(canary, tt.invoke)
			if err == nil {
				t.Fatal("WithCanary() = nil, want domain error")
			}
			if errors.Is(err, ErrCanaryLeak) {
				t.Fatalf("WithCanary() returned ErrCanaryLeak: %v", err)
			}
			if strings.Contains(err.Error(), canary) {
				t.Fatalf("error text %q leaks canary %q", err, canary)
			}
			var domainErr *Error
			if !errors.As(err, &domainErr) {
				t.Fatalf("error %v is not a *Error", err)
			}
			if domainErr.Code != tt.wantCode {
				t.Fatalf("error code = %q, want %q", domainErr.Code, tt.wantCode)
			}
		})
	}
}
