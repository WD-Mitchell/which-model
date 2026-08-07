package security

import (
	"testing"
)

func TestValidateTrustedBaseURL(t *testing.T) {
	const untrusted = "untrusted_origin: The configured Codex fallback origin was not explicitly trusted."

	tests := []struct {
		name          string
		rawURL        string
		trustedOrigin string
		want          string // fallback target on success; "" means error expected
	}{
		{"trailing slash base", "https://chatgpt.com/backend-api/", "https://chatgpt.com", "https://chatgpt.com/backend-api/api/codex/usage"},
		{"no trailing slash base", "https://chatgpt.com/backend-api", "https://chatgpt.com", "https://chatgpt.com/backend-api/api/codex/usage"},
		{"root base", "https://chatgpt.com/", "https://chatgpt.com", "https://chatgpt.com/api/codex/usage"},
		{"different origin", "https://chatgpt.com/backend-api/", "https://other.com", ""},
		{"trust has path", "https://chatgpt.com/backend-api/", "https://chatgpt.com/foo", ""},
		{"trust has query", "https://chatgpt.com/backend-api/", "https://chatgpt.com/?q=1", ""},
		{"trust has fragment", "https://chatgpt.com/backend-api/", "https://chatgpt.com/#h", ""},
		{"base has query", "https://chatgpt.com/backend-api/?q=1", "https://chatgpt.com", ""},
		{"base has fragment", "https://chatgpt.com/backend-api/#h", "https://chatgpt.com", ""},
		{"base non-https", "http://chatgpt.com/backend-api/", "https://chatgpt.com", ""},
		{"base userinfo", "https://user@chatgpt.com/backend-api/", "https://chatgpt.com", ""},
		{"trust unparseable", "https://chatgpt.com/backend-api/", "::bad::", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTrustedBaseURL(tt.rawURL, tt.trustedOrigin)
			if tt.want != "" {
				if err != nil {
					t.Fatalf("ValidateTrustedBaseURL(%q, %q) error = %v, want nil", tt.rawURL, tt.trustedOrigin, err)
				}
				if got != tt.want {
					t.Fatalf("ValidateTrustedBaseURL(%q, %q) = %q, want %q", tt.rawURL, tt.trustedOrigin, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateTrustedBaseURL(%q, %q) = %q, want error %q", tt.rawURL, tt.trustedOrigin, got, untrusted)
			}
			if gotErr := err.Error(); gotErr != untrusted {
				t.Fatalf("ValidateTrustedBaseURL(%q, %q) error = %q, want %q", tt.rawURL, tt.trustedOrigin, gotErr, untrusted)
			}
		})
	}
}
