package whichmodel

import (
	"bytes"
	"testing"
)

// captureExecute swaps Stdout/Stderr, runs ExecuteArgs, and restores them.
func captureExecute(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	prevOut, prevErr := Stdout, Stderr
	Stdout, Stderr = outBuf, errBuf
	defer func() { Stdout, Stderr = prevOut, prevErr }()
	return ExecuteArgs(args), outBuf.String(), errBuf.String()
}

func TestRoot(t *testing.T) {
	t.Run("bare root prints help exit 0", func(t *testing.T) {
		code, out, _ := captureExecute(t, nil)
		if code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
		if out == "" {
			t.Error("expected help output on stdout")
		}
	})

	t.Run("help flag exit 0", func(t *testing.T) {
		code, _, _ := captureExecute(t, []string{"--help"})
		if code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
	})

	t.Run("unknown command is a usage failure", func(t *testing.T) {
		code, _, errOut := captureExecute(t, []string{"nosuchcmd"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		if errOut == "" {
			t.Error("expected failure line on stderr")
		}
	})

	t.Run("root name is fixed", func(t *testing.T) {
		if got := NewRootCmd().Use; got != "which-model" {
			t.Errorf("Use = %q, want which-model", got)
		}
	})

	t.Run("errors silenced", func(t *testing.T) {
		cmd := NewRootCmd()
		if !cmd.SilenceErrors || !cmd.SilenceUsage {
			t.Error("SilenceErrors and SilenceUsage must both be true")
		}
	})

	t.Run("completion command disabled", func(t *testing.T) {
		for _, c := range NewRootCmd().Commands() {
			if c.Name() == "completion" {
				t.Fatal("completion command must be disabled")
			}
		}
	})
}
