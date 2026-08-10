//go:build nousage

package credential_test

import (
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

func TestWarningMessage(t *testing.T) {
	w := credential.Warning{Message: "boom"}
	if w.Message != "boom" {
		t.Errorf("Message = %q, want %q", w.Message, "boom")
	}
}

func TestWarningZeroValue(t *testing.T) {
	var w credential.Warning
	if w.Message != "" {
		t.Errorf("zero value Message = %q, want empty", w.Message)
	}
}
