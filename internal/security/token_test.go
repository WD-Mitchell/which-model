package security

import (
	"strings"
	"testing"
)

func TestValidateOpaqueToken(t *testing.T) {
	const fixedErr = "unsafe_credential: The credential is missing or unsafe."

	tests := []struct {
		name  string
		token string
		want  string // "" means nil error
	}{
		{"valid short", "sk-ant-abcdefgh", ""},
		{"valid max length", strings.Repeat("a", 8192), ""},
		{"too long", strings.Repeat("a", 8193), fixedErr},
		{"empty", "", fixedErr},
		{"valid min length", "a", ""},
		{"newline", "abc\ndef", fixedErr},
		{"tab", "abc\tdef", fixedErr},
		{"space", "abc def", fixedErr},
		{"nul", "abc\x00def", fixedErr},
		{"c0 control", "abc\x1fdef", fixedErr},
		{"del", "abc\x7fdef", fixedErr},
		{"nbsp", "abc\u00a0def", fixedErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOpaqueToken(tt.token)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("ValidateOpaqueToken(%q) = %v, want nil", tt.token, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateOpaqueToken(%q) = nil, want error %q", tt.token, tt.want)
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("ValidateOpaqueToken(%q) error = %q, want %q", tt.token, got, tt.want)
			}
			if tt.token != "" && strings.Contains(err.Error(), tt.token) {
				t.Fatalf("error text %q leaks the token %q", err.Error(), tt.token)
			}
		})
	}
}
