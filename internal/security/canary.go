package security

import (
	"errors"
	"strings"
)

// ErrCanaryLeak is returned by WithCanary when the canary string appears in the
// returned error's text. Its message is fixed and never echoes the canary or
// the offending error.
var ErrCanaryLeak = errors.New("security: canary token leaked into error text")

// WithCanary runs fn. If fn returns an error whose text contains canary,
// WithCanary returns ErrCanaryLeak (detectable via errors.Is); otherwise it
// returns fn's error unchanged (nil stays nil).
func WithCanary(canary string, fn func() error) error {
	err := fn()
	if err != nil && strings.Contains(err.Error(), canary) {
		return ErrCanaryLeak
	}
	return err
}
