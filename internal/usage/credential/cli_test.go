//go:build !nousage

package credential

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestCLIResolver(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"

	sh := func(script string) []string { return []string{"-c", script} }

	cases := []struct {
		name    string
		r       CLIResolver
		wantTok string
		wantErr error
		maxWall time.Duration // 0 = no wall-clock assertion
	}{
		{
			name:    "plain token", // case 1
			r:       CLIResolver{Command: "sh", Args: sh(`printf 'tok123\n'`)},
			wantTok: "tok123",
		},
		{
			name:    "crlf stripped", // case 2
			r:       CLIResolver{Command: "sh", Args: sh(`printf 'tok123\r\n'`)},
			wantTok: "tok123",
		},
		{
			name:    "extra newline unsafe", // case 3
			r:       CLIResolver{Command: "sh", Args: sh(`printf 'tok123\n\n'`)},
			wantErr: ErrNotFound,
		},
		{
			name:    "nonzero exit", // case 4
			r:       CLIResolver{Command: "sh", Args: sh("exit 3")},
			wantErr: ErrNotFound,
		},
		{
			name:    "timeout", // case 5
			r:       CLIResolver{Command: "sh", Args: sh("sleep 5"), Timeout: 100 * time.Millisecond},
			wantErr: ErrNotFound,
			maxWall: 2 * time.Second,
		},
		{
			name:    "output over cap", // case 6
			r:       CLIResolver{Command: "sh", Args: sh(`head -c 40000 /dev/zero | tr "\0" "a"`)},
			wantErr: ErrNotFound,
		},
		{
			name:    "binary not found", // case 7
			r:       CLIResolver{Command: "./definitely-not-a-binary"},
			wantErr: ErrNotFound,
		},
		{
			name:    "cancelled context", // case 8
			r:       CLIResolver{Command: "sh", Args: sh("sleep 5")},
			wantErr: ErrNotFound,
			maxWall: 2 * time.Second,
		},
		{
			name:    "unsafe shape", // case 9
			r:       CLIResolver{Command: "sh", Args: sh(`printf 'bad\ttok\n'`)},
			wantErr: ErrNotFound,
		},
		{
			name:    "canary exit failure", // case 10
			r:       CLIResolver{Command: "sh", Args: sh(`printf '` + canary + `\n'; exit 3`)},
			wantErr: ErrNotFound,
		},
		{
			name:    "generous timeout respected", // case 11
			r:       CLIResolver{Command: "sh", Args: sh(`printf 'tok\n'`), Timeout: 10 * time.Second},
			wantTok: "tok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			ctx := context.Background()
			if tc.name == "cancelled context" {
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			cred, err := tc.r.Resolve(ctx)
			if tc.maxWall > 0 {
				if elapsed := time.Since(start); elapsed > tc.maxWall {
					t.Fatalf("Resolve() took %v, want < %v", elapsed, tc.maxWall)
				}
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Resolve() error = %v, want %v", err, tc.wantErr)
				}
				if tc.name == "canary exit failure" && err != nil && strings.Contains(err.Error(), canary) {
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
			if cred.Source != usage.AuthCLIShellOut {
				t.Fatalf("Resolve() source = %v, want AuthCLIShellOut", cred.Source)
			}
		})
	}
}

func TestCLIResolverCapOverride(t *testing.T) {
	// A custom small cap must bound output before the 32 KiB default.
	r := &CLIResolver{
		Command:        "sh",
		Args:           []string{"-c", `head -c 1000 /dev/zero | tr "\0" "a"`},
		MaxOutputBytes: 100,
	}
	_, err := r.Resolve(context.Background())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want ErrNotFound (overridden cap)", err)
	}
}
