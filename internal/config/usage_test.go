package config

import (
	"strings"
	"testing"
)

func TestParseUsageEnabled(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want UsageEnabled
		err  bool
	}{
		{"auto", "auto", UsageAuto, false},
		{"true", "true", UsageTrue, false},
		{"false", "false", UsageFalse, false},
		{"on", "on", "", true},
		{"uppercase", "TRUE", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUsageEnabled(tt.in)
			if tt.err {
				if err == nil || !strings.Contains(err.Error(), "usage.enabled") {
					t.Fatalf("ParseUsageEnabled(%q) error = %v", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUsageEnabled(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseUsageEnabled(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUsageEnabledUnmarshalTOML(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want UsageEnabled
		err  bool
	}{
		{"true", true, UsageTrue, false},
		{"false", false, UsageFalse, false},
		{"auto", "auto", UsageAuto, false},
		{"banana", "banana", "", true},
		{"int", int64(1), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u UsageEnabled
			err := (&u).UnmarshalTOML(tt.in)
			if tt.err {
				if err == nil {
					t.Fatalf("UnmarshalTOML(%#v) error = nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalTOML(%#v) error = %v", tt.in, err)
			}
			if u != tt.want {
				t.Fatalf("UnmarshalTOML(%#v) set %q, want %q", tt.in, u, tt.want)
			}
		})
	}
}
