package publish

import (
	"strings"
	"testing"
)

func TestValidateCron(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantSub string
	}{
		{"valid default", "0 6 * * *", false, ""},
		{"valid step", "*/15 * * * *", false, ""},
		{"valid list", "5,10,15 * * * *", false, ""},
		{"valid names", "0 6 * JAN MON", false, ""},
		{"valid names lowercase", "0 6 * * sat", false, ""},
		{"six fields", "0 6 * * * *", true, "5 fields"},
		{"at keyword", "@daily", true, "@"},
		{"minute out of bounds", "60 6 * * *", true, "minute"},
		{"dow out of bounds", "0 6 * * 7", true, "day-of-week"},
		{"empty", "", true, "5 fields"},
		{"names in range", "MON-FRI * * * *", true, ""},
		{"step zero", "*/0 * * * *", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCron(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateCron(%q) = nil, want error", tt.input)
				}
				if tt.wantSub != "" && !strings.Contains(err.Error(), tt.wantSub) {
					t.Errorf("ValidateCron(%q) error = %q, want substring %q", tt.input, err, tt.wantSub)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateCron(%q) = %v, want nil", tt.input, err)
			}
		})
	}
}
