package whichmodel

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

func TestExitCodeFor(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := ExitCodeFor(nil); got != 0 {
			t.Errorf("ExitCodeFor(nil) = %d, want 0", got)
		}
	})

	t.Run("usage error", func(t *testing.T) {
		if got := ExitCodeFor(&UsageError{}); got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})

	t.Run("coded unauthorized", func(t *testing.T) {
		if got := ExitCodeFor(&CodedError{Code: "unauthorized"}); got != 5 {
			t.Errorf("CodedError got %d, want 5", got)
		}
		if got := ExitCodeFor(&httpkit.Error{Code: "unauthorized", StatusCode: 401}); got != 5 {
			t.Errorf("httpkit.Error got %d, want 5", got)
		}
	})

	t.Run("coded usage_disabled", func(t *testing.T) {
		if got := ExitCodeFor(&CodedError{Code: "usage_disabled"}); got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})

	t.Run("coded unknown", func(t *testing.T) {
		if got := ExitCodeFor(&CodedError{Code: "nope"}); got != 1 {
			t.Errorf("got %d, want 1", got)
		}
	})

	t.Run("exit code interface", func(t *testing.T) {
		if got := ExitCodeFor(&config.ConfigError{Kind: config.KindInvalidTOML}); got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})

	t.Run("plain error", func(t *testing.T) {
		if got := ExitCodeFor(errors.New("boom")); got != 1 {
			t.Errorf("got %d, want 1", got)
		}
	})

	t.Run("registered code", func(t *testing.T) {
		RegisterExitCode("no_viable_candidate", 3)
		if got := ExitCodeFor(&CodedError{Code: "no_viable_candidate"}); got != 3 {
			t.Errorf("got %d, want 3", got)
		}
	})

	t.Run("reported exit unwrapped", func(t *testing.T) {
		// F26 registers auth_required→5 in its own init(); this test
		// registers it explicitly so the mapping is exercised here.
		RegisterExitCode("auth_required", 5)
		if got := ExitCodeFor(&ReportedError{Err: &CodedError{Code: "auth_required"}}); got != 5 {
			t.Errorf("got %d, want 5", got)
		}
	})
}

func TestCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"usage", &UsageError{}, "arguments"},
		{"coded", &CodedError{Code: "x"}, "x"},
		{"config", &config.ConfigError{}, "config"},
		{"plain", errors.New("e"), "error"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CodeFor(tc.err); got != tc.want {
				t.Errorf("CodeFor = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFailureRendering(t *testing.T) {
	t.Run("reported json suppressed", func(t *testing.T) {
		outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
		prevOut, prevErr, prevJSON := Stdout, Stderr, Global.JSON
		Stdout, Stderr, Global.JSON = outBuf, errBuf, true
		defer func() { Stdout, Stderr, Global.JSON = prevOut, prevErr, prevJSON }()

		code := renderError(nil, &ReportedError{Err: errors.New("boom")})
		if code != 1 {
			t.Errorf("exit = %d, want 1", code)
		}
		if errBuf.Len() == 0 {
			t.Error("stderr must carry the failure line")
		}
		if outBuf.Len() != 0 {
			t.Errorf("stdout must NOT carry a JSON error document for ReportedError, got %q", outBuf.String())
		}
	})

	t.Run("unknown command failure line", func(t *testing.T) {
		code, _, errOut := captureExecute(t, []string{"nosuchcmd"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		if !strings.HasPrefix(errOut, "which-model nosuchcmd: [arguments]") {
			t.Errorf("stderr = %q, want prefix `which-model nosuchcmd: [arguments]`", errOut)
		}
	})

	t.Run("json error document", func(t *testing.T) {
		code, out, errOut := captureExecute(t, []string{"--json", "--text"})
		if code != 2 {
			t.Errorf("exit = %d, want 2", code)
		}
		if errOut == "" {
			t.Error("stderr must carry the failure line")
		}
		if !strings.Contains(out, `"error"`) || !strings.Contains(out, `"schema_version":"2.0"`) {
			t.Errorf("stdout JSON error document missing fields: %q", out)
		}
	})
}
