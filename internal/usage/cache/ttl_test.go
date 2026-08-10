//go:build !nousage

package cache

import (
	"testing"
	"time"
)

func TestEffectiveTTL(t *testing.T) {
	tests := []struct {
		name   string
		base   time.Duration
		maxAge time.Duration
		want   time.Duration
	}{
		{"no max age override", 60 * time.Second, 0, 60 * time.Second},
		{"max age overrides base", 60 * time.Second, 5 * time.Minute, 5 * time.Minute},
		{"negative max age ignored", 60 * time.Second, -1 * time.Second, 60 * time.Second},
		{"both zero (provider uncached)", 0, 0, 0},
		{"zero base with max age", 0, 90 * time.Second, 90 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveTTL(tt.base, tt.maxAge); got != tt.want {
				t.Errorf("EffectiveTTL(%v, %v) = %v, want %v", tt.base, tt.maxAge, got, tt.want)
			}
		})
	}
}
