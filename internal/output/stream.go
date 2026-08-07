package output

import (
	"fmt"
	"io"
)

// WriteFailure writes the fixed failure line
// "which-model <command>: [<code>] <message>\n" (annex-d §1.3). Callers route
// it to stderr.
func WriteFailure(w io.Writer, command, code, message string) error {
	_, err := fmt.Fprintf(w, "which-model %s: [%s] %s\n", command, code, message)
	return err
}

// WriteWarning writes "warning: <message>\n" (annex-d §1.3). Callers route it
// to stderr.
func WriteWarning(w io.Writer, message string) error {
	_, err := fmt.Fprintf(w, "warning: %s\n", message)
	return err
}

// RedactIdentity implements the --show-identity contract: returns nil when
// show is false (the value is omitted entirely, never masked — annex-d §1.2),
// or a pointer to value when show is true. JSON callers use omitempty; text
// renderers skip nil values.
func RedactIdentity(value string, show bool) *string {
	if !show {
		return nil
	}
	return &value
}
