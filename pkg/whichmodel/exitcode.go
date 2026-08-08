package whichmodel

import (
	"errors"
	"sync"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// UsageError is an argument/config usage failure → exit 2, code "arguments".
type UsageError struct{ Message string }

func (e *UsageError) Error() string { return e.Message }

// CodedError carries a stable code from global CONTRACTS §1.6 through the
// error path. Unknown codes exit 1.
type CodedError struct{ Code, Message string }

func (e *CodedError) Error() string { return e.Message }

// ReportedError marks a failure whose deliverable already went to stdout
// (F25 auth status, F27 verify): ExecuteArgs renders the stderr failure line
// only, NEVER the JSON error document.
type ReportedError struct{ Err error }

func (e *ReportedError) Error() string { return e.Err.Error() }
func (e *ReportedError) Unwrap() error { return e.Err }

// codedExit is the global CONTRACTS §1.6 exit-code table (auth codes → 5,
// usage-disabled codes → 2; every other code defaults to 1).
var (
	codedExitMu sync.RWMutex
	codedExit   = map[string]int{
		"unauthorized":       5,
		"expired_credential": 5,
		"login_required":     5,
		"credential_file":    5,
		"credential_json":    5,
		"unsafe_credential":  5,
		"access_denied":      5,
		"device_expired":     5,
		"cookie_unavailable": 5,
		"signing_failed":     5,
		"usage_disabled":     2,
		"usage_compiled_out": 2,
	}
)

// RegisterExitCode extends the code→exit table (F26 registers 3/4).
func RegisterExitCode(code string, exit int) {
	codedExitMu.Lock()
	defer codedExitMu.Unlock()
	codedExit[code] = exit
}

func lookupExit(code string) (int, bool) {
	codedExitMu.RLock()
	defer codedExitMu.RUnlock()
	exit, ok := codedExit[code]
	return exit, ok
}

// ExitCodeFor maps an execution error to the process exit code
// (specs/global/SPEC.md §5): nil→0; *UsageError→2; *CodedError→§1.6 table
// (default 1); *httpkit.Error→§1.6 table by Code; any error exposing
// ExitCode() int→that value (F01 ConfigError→2); *ReportedError→unwrapped;
// else 1.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		return 2
	}
	var coded *CodedError
	if errors.As(err, &coded) {
		if exit, ok := lookupExit(coded.Code); ok {
			return exit
		}
		return 1
	}
	var httpErr *httpkit.Error
	if errors.As(err, &httpErr) {
		if exit, ok := lookupExit(httpErr.Code); ok {
			return exit
		}
		return 1
	}
	var reporter interface{ ExitCode() int }
	if errors.As(err, &reporter) {
		return reporter.ExitCode()
	}
	var reported *ReportedError
	if errors.As(err, &reported) {
		return ExitCodeFor(reported.Err)
	}
	return 1
}

// CodeFor returns the failure code string for the failure line: "arguments"
// for UsageError; the carried code for CodedError/httpkit.Error; "config"
// for F01 ConfigError (and any exit-2 error without a code); "error"
// otherwise. ReportedError unwraps.
func CodeFor(err error) string {
	if err == nil {
		return ""
	}
	var reported *ReportedError
	if errors.As(err, &reported) {
		return CodeFor(reported.Err)
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		return "arguments"
	}
	var coded *CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	var httpErr *httpkit.Error
	if errors.As(err, &httpErr) {
		return httpErr.Code
	}
	var cfgErr *config.ConfigError
	if errors.As(err, &cfgErr) {
		return "config"
	}
	var reporter interface{ ExitCode() int }
	if errors.As(err, &reporter) && reporter.ExitCode() == 2 {
		return "config"
	}
	return "error"
}
