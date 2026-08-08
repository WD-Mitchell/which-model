package usage

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrUsageCompiledOut(t *testing.T) {
	if got := ErrUsageCompiledOut.Error(); got != "usage subsystem compiled out (-tags nousage)" {
		t.Errorf("Error() = %q, want %q", got, "usage subsystem compiled out (-tags nousage)")
	}
}

func TestErrUsageCompiledOutIs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "identical sentinel", err: ErrUsageCompiledOut, want: true},
		{name: "wrapped sentinel", err: fmt.Errorf("wrap: %w", ErrUsageCompiledOut), want: true},
		{name: "unrelated error", err: errors.New("other"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, ErrUsageCompiledOut); got != tt.want {
				t.Errorf("errors.Is(%v, ErrUsageCompiledOut) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
