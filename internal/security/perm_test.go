package security

import (
	"io/fs"
	"testing"
)

func TestHasBroadPermissions(t *testing.T) {
	tests := []struct {
		name string
		mode fs.FileMode
		want bool
	}{
		{"0600", fs.FileMode(0o600), false},
		{"0700", fs.FileMode(0o700), false},
		{"0000", fs.FileMode(0o000), false},
		{"0644", fs.FileMode(0o644), true},
		{"0640", fs.FileMode(0o640), true},
		{"0604", fs.FileMode(0o604), true},
		{"0607", fs.FileMode(0o607), true},
		{"0770", fs.FileMode(0o770), true},
		{"0777", fs.FileMode(0o777), true},
		{"0060", fs.FileMode(0o060), true},
		{"0007", fs.FileMode(0o007), true},
		{"0666", fs.FileMode(0o666), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasBroadPermissions(tt.mode); got != tt.want {
				t.Fatalf("HasBroadPermissions(%o) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}
