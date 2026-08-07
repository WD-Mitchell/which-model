package security

import (
	"testing"
)

func TestValidateExactHTTPS(t *testing.T) {
	const refused = "endpoint_refused: The provider endpoint was refused."
	const notValid = "endpoint_refused: The provider endpoint is not a valid URL."

	tests := []struct {
		name    string
		rawURL  string
		allowed []string
		want    string // canonical URL on success; "" means error expected
		wantErr string // exact error text when want == ""
	}{
		{"bare origin", "https://api.anthropic.com", []string{"https://api.anthropic.com"}, "https://api.anthropic.com", ""},
		{"query allowed", "https://api.anthropic.com/v1/organizations/cost_report?x=1", []string{"https://api.anthropic.com/v1/organizations/cost_report?x=1"}, "https://api.anthropic.com/v1/organizations/cost_report?x=1", ""},
		{"no prefix matching", "https://api.anthropic.com/v1/x", []string{"https://api.anthropic.com"}, "", refused},
		{"non-https", "http://api.anthropic.com", []string{"http://api.anthropic.com"}, "", refused},
		{"userinfo", "https://user@example.com/", []string{"https://user@example.com/"}, "", refused},
		{"fragment", "https://example.com/#frag", []string{"https://example.com/#frag"}, "", refused},
		{"trailing slash vs bare", "https://example.com/", []string{"https://example.com"}, "", refused},
		{"bare vs trailing slash", "https://example.com", []string{"https://example.com/"}, "", refused},
		{"unparseable", "ht!tp://%%%", []string{"https://example.com"}, "", notValid},
		{"empty host", "https://", []string{"https://example.com"}, "", refused},
		{"port preserved", "https://api.anthropic.com:8443/x", []string{"https://api.anthropic.com:8443/x"}, "https://api.anthropic.com:8443/x", ""},
		{"member of longer list", "https://example.com", []string{"https://other.com", "https://example.com"}, "https://example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateExactHTTPS(tt.rawURL, tt.allowed)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateExactHTTPS(%q, %v) error = %v, want nil", tt.rawURL, tt.allowed, err)
				}
				if got != tt.want {
					t.Fatalf("ValidateExactHTTPS(%q, %v) = %q, want %q", tt.rawURL, tt.allowed, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateExactHTTPS(%q, %v) = %q, want error %q", tt.rawURL, tt.allowed, got, tt.wantErr)
			}
			if gotErr := err.Error(); gotErr != tt.wantErr {
				t.Fatalf("ValidateExactHTTPS(%q, %v) error = %q, want %q", tt.rawURL, tt.allowed, gotErr, tt.wantErr)
			}
		})
	}
}
