package config

import (
	"errors"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Usage.Enabled != UsageAuto {
		t.Fatalf("Default().Usage.Enabled = %q, want %q", cfg.Usage.Enabled, UsageAuto)
	}
	if cfg.Providers == nil {
		t.Fatal("Default().Providers is nil")
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("Default().Providers len = %d, want 0", len(cfg.Providers))
	}

	other := Default()
	other.Usage.Enabled = UsageFalse
	if cfg.Usage.Enabled != UsageAuto {
		t.Fatal("two Default calls returned shared state")
	}
}

func TestConfigErrorExitCode(t *testing.T) {
	for _, kind := range []ErrorKind{KindNotFound, KindUnreadable, KindInvalidTOML, KindInvalidValue} {
		t.Run(string(rune('0'+kind)), func(t *testing.T) {
			if got := (&ConfigError{Kind: kind}).ExitCode(); got != 2 {
				t.Fatalf("ExitCode() = %d, want 2", got)
			}
		})
	}
}

func TestConfigErrorMessage(t *testing.T) {
	missing := (&ConfigError{Kind: KindNotFound, Path: "/x.toml"}).Error()
	if !strings.Contains(missing, "/x.toml") || !strings.Contains(missing, "not found") {
		t.Fatalf("missing error = %q", missing)
	}
	invalid := (&ConfigError{Kind: KindInvalidValue, Key: "usage.enabled", Err: errors.New("bad")}).Error()
	if !strings.Contains(invalid, "usage.enabled") || !strings.Contains(invalid, "bad") {
		t.Fatalf("invalid error = %q", invalid)
	}
}

func TestConfigErrorUnwrap(t *testing.T) {
	cause := errors.New("cause")
	err := &ConfigError{Kind: KindInvalidValue, Err: cause}
	if got := errors.Unwrap(err); got != cause {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
	wrapped := errors.New("outer: " + err.Error())
	_ = wrapped
	var ce *ConfigError
	chain := fmtWrap{err: err}
	if !errors.As(chain, &ce) || ce != err {
		t.Fatal("errors.As did not find ConfigError")
	}
}

type fmtWrap struct{ err error }

func (w fmtWrap) Error() string { return "wrapped: " + w.err.Error() }
func (w fmtWrap) Unwrap() error { return w.err }
