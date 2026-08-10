//go:build !nousage

package credential

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestEnvResolver(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"

	cases := []struct {
		name    string
		env     map[string]string
		r       EnvResolver
		wantTok string
		wantErr error // ErrNotFound or nil
		wantSrc usage.AuthKind
		wantExtra map[string]string // nil = don't check
	}{
		{
			name:    "plain token", // case 1
			env:     map[string]string{"WM_TEST_TOK": "tok123"},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantTok: "tok123",
			wantSrc: usage.AuthEnvVar,
		},
		{
			name:    "unset var", // case 2
			env:     map[string]string{},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantErr: ErrNotFound,
		},
		{
			name:    "empty value", // case 3
			env:     map[string]string{"WM_TEST_TOK": ""},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantErr: ErrNotFound,
		},
		{
			name:    "blank value", // case 4
			env:     map[string]string{"WM_TEST_TOK": "   "},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantErr: ErrNotFound,
		},
		{
			name:    "double-quoted", // case 5
			env:     map[string]string{"WM_TEST_TOK": `"tok123"`},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantTok: "tok123",
			wantSrc: usage.AuthEnvVar,
		},
		{
			name:    "single-quoted", // case 6
			env:     map[string]string{"WM_TEST_TOK": "'tok123'"},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantTok: "tok123",
			wantSrc: usage.AuthEnvVar,
		},
		{
			name:    "control char", // case 7
			env:     map[string]string{"WM_TEST_TOK": "bad\ntok"},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantErr: ErrNotFound,
		},
		{
			name:    "extra populated", // case 8
			env:     map[string]string{"WM_TEST_TOK": "tok123", "WM_TEST_PROJ": "proj-1"},
			r:       EnvResolver{Var: "WM_TEST_TOK", Extra: []string{"WM_TEST_PROJ"}},
			wantTok: "tok123",
			wantSrc: usage.AuthEnvVar,
			wantExtra: map[string]string{"WM_TEST_PROJ": "proj-1"},
		},
		{
			name:    "extra unset", // case 9
			env:     map[string]string{"WM_TEST_TOK": "tok123"},
			r:       EnvResolver{Var: "WM_TEST_TOK", Extra: []string{"WM_TEST_PROJ"}},
			wantTok: "tok123",
			wantSrc: usage.AuthEnvVar,
			wantExtra: map[string]string{},
		},
		{
			name:    "canary resolves", // case 10A
			env:     map[string]string{"WM_TEST_TOK": canary},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantTok: canary,
			wantSrc: usage.AuthEnvVar,
		},
		{
			name:    "canary with newline rejected", // case 10B
			env:     map[string]string{"WM_TEST_TOK": canary + "\n"},
			r:       EnvResolver{Var: "WM_TEST_TOK"},
			wantErr: ErrNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cred, err := tc.r.Resolve(context.Background())
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Resolve() error = %v, want %v", err, tc.wantErr)
				}
				if tc.name == "canary with newline rejected" && err != nil && strings.Contains(err.Error(), canary) {
					t.Fatalf("error %q leaks canary", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}
			if cred.Token != tc.wantTok {
				t.Fatalf("Resolve() token = %q, want %q", cred.Token, tc.wantTok)
			}
			if cred.Source != tc.wantSrc {
				t.Fatalf("Resolve() source = %v, want %v", cred.Source, tc.wantSrc)
			}
			if tc.wantExtra != nil {
				if len(cred.Extra) != len(tc.wantExtra) {
					t.Fatalf("Resolve() Extra = %v, want %v", cred.Extra, tc.wantExtra)
				}
				for k, v := range tc.wantExtra {
					if cred.Extra[k] != v {
						t.Fatalf("Resolve() Extra[%q] = %q, want %q", k, cred.Extra[k], v)
					}
				}
			}
			if strings.Contains(cred.String(), canary) {
				t.Fatalf("Credential.String() = %q leaks canary", cred.String())
			}
		})
	}
}
