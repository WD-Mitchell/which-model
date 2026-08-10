//go:build !nousage

package credential

import (
	"context"
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Test case 5: the alias re-export compiles (usage.Credential = Credential).
var _ usage.Credential = Credential{}

// TestResolveChainDegenerate covers the F12-T1 degenerate paths: zero-length
// sources and first-source kinds F12 does not implement.
func TestResolveChainDegenerate(t *testing.T) {
	cases := []struct {
		name    string
		sources []usage.AuthSource
	}{
		{"nil sources", nil},                              // case 1
		{"empty sources", []usage.AuthSource{}},           // case 2
		{"rpc kind", []usage.AuthSource{{Kind: usage.AuthSubprocessRPC}}},  // case 3
		{"grpc-web kind", []usage.AuthSource{{Kind: usage.AuthGRPCWebToken}}}, // case 4
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cred, warnings, err := ResolveChain(context.Background(), tc.sources, nil)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("ResolveChain() error = %v, want ErrNotFound", err)
			}
			if cred.Token != "" || cred.Source != usage.AuthEnvVar {
				t.Fatalf("ResolveChain() credential = %+v, want zero", cred)
			}
			if len(warnings) != 0 {
				t.Fatalf("ResolveChain() warnings = %v, want none", warnings)
			}
		})
	}
}

// TestErrNotFoundIsNotFailure: case 6 — ErrNotFound is a plain sentinel,
// never a *usage.FailureError.
func TestErrNotFoundIsNotFailure(t *testing.T) {
	f, ok := usage.AsFailure(ErrNotFound)
	if ok || f != (usage.Failure{}) {
		t.Fatalf("AsFailure(ErrNotFound) = (%v, %v), want (Failure{}, false)", f, ok)
	}
}
