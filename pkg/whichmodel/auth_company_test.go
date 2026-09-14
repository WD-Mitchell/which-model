//go:build !nousage

package whichmodel

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
)

func TestAuthLoginPreservesCompanyPolicyErrors(t *testing.T) {
	oldTTY, oldEnv, oldStart, oldSave := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc
	t.Cleanup(func() {
		stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc = oldTTY, oldEnv, oldStart, oldSave
	})
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return false }
	for _, stage := range []string{"start", "poll", "save"} {
		t.Run(stage, func(t *testing.T) {
			denied := &company.Error{Reason: "required policy missing"}
			startDeviceFlowFunc = func(string) (DeviceFlow, error) {
				if stage == "start" {
					return DeviceFlow{}, denied
				}
				return DeviceFlow{Poll: func() (string, error) {
					if stage == "poll" {
						return "", denied
					}
					return "SYNTHETIC_TOKEN", nil
				}}, nil
			}
			saved := false
			saveCredentialFunc = func(string, string) error { saved = true; return denied }
			err := RunAuthLogin("copilot", io.Discard, io.Discard, strings.NewReader(""))
			if !errors.Is(err, denied) || ExitCodeFor(err) != 2 || saved != (stage == "save") {
				t.Fatalf("stage=%s err=%v exit=%d saved=%v", stage, err, ExitCodeFor(err), saved)
			}
		})
	}
}
