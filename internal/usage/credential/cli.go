//go:build !nousage

package credential

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// MaxCLIOutputBytes caps a shell-out's stdout (prototype maxBuffer 32_768).
const MaxCLIOutputBytes = 32 * 1024

// CLIResolver runs Command+Args; every failure (non-zero exit, timeout,
// output over cap, empty/unsafe output) → ErrNotFound (SPEC §5). Strips
// exactly one trailing \r\n or \n. Secrets are never passed via argv/env;
// future secret-input subprocesses receive secrets on stdin (SPEC D1) —
// this resolver only runs token-EMITTING commands.
type CLIResolver struct {
	Command        string
	Args           []string
	Timeout        time.Duration
	MaxOutputBytes int64 // <= 0 → MaxCLIOutputBytes
}

// Resolve runs the command under a hard deadline (min of ctx deadline and
// r.Timeout) and maps EVERY failure to ErrNotFound: the chain must be able
// to fall through to the next source when a CLI is missing, slow, or
// malformed (SPEC §5, D5). Command output never leaks into errors.
func (r *CLIResolver) Resolve(ctx context.Context) (usage.Credential, error) {
	cap := int64(MaxCLIOutputBytes)
	if r.MaxOutputBytes > 0 {
		cap = r.MaxOutputBytes
	}

	runCtx := ctx
	cancel := func() {}
	if r.Timeout > 0 {
		if dl, ok := ctx.Deadline(); !ok || time.Until(dl) > r.Timeout {
			runCtx, cancel = context.WithTimeout(ctx, r.Timeout)
		}
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, r.Command, r.Args...)
	out := &maxBufferWriter{max: cap}
	cmd.Stdout = out

	err := cmd.Run()
	if err != nil || out.overCap || out.buf.Len() == 0 {
		return Credential{}, ErrNotFound
	}

	token := strings.TrimSuffix(out.buf.String(), "\r\n")
	token = strings.TrimSuffix(token, "\n") // exactly one strip
	if err := security.ValidateOpaqueToken(token); err != nil {
		return Credential{}, ErrNotFound
	}
	return Credential{Token: token, Source: usage.AuthCLIShellOut}, nil
}

// maxBufferWriter accumulates stdout up to max bytes and then fails the
// write, aborting the copy loop (io.LimitedReader-style capping).
type maxBufferWriter struct {
	buf    bytes.Buffer
	max    int64
	overCap bool
}

func (w *maxBufferWriter) Write(p []byte) (int, error) {
	remaining := w.max - int64(w.buf.Len())
	if int64(len(p)) > remaining {
		if remaining > 0 {
			w.buf.Write(p[:remaining])
		}
		w.overCap = true
		return len(p), errOutputCap
	}
	w.buf.Write(p)
	return len(p), nil
}

// errOutputCap stops os/exec's stdout copy when the cap is exceeded. Its
// text is never surfaced: Resolve maps the failure to ErrNotFound.
var errOutputCap = errors.New("cli output exceeds cap")
