//go:build !nousage

package whichmodel

import (
	"errors"
	"strings"
	"testing"
)

func TestAuthLoginRejectsNonTTY(t *testing.T) {
	oldTTY, oldEnv, oldStart := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc
	t.Cleanup(func() { stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc = oldTTY, oldEnv, oldStart })
	stdinIsTTY = func() bool { return false }
	nonInteractiveEnv = func() bool { return false }
	called := false
	startDeviceFlowFunc = func(string) (DeviceFlow, error) { called = true; return DeviceFlow{}, nil }
	var out, errOut strings.Builder
	err := RunAuthLogin("copilot", &out, &errOut, strings.NewReader(""))
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "refusing unattended login; run from an interactive terminal") || called {
		t.Fatalf("err = %v, exit = %d, called = %v", err, ExitCodeFor(err), called)
	}
}

func TestAuthLoginRejectsNonInteractiveEnv(t *testing.T) {
	oldTTY, oldEnv, oldStart := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc
	t.Cleanup(func() { stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc = oldTTY, oldEnv, oldStart })
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return true }
	called := false
	startDeviceFlowFunc = func(string) (DeviceFlow, error) { called = true; return DeviceFlow{}, nil }
	var out, errOut strings.Builder
	err := RunAuthLogin("copilot", &out, &errOut, strings.NewReader(""))
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "refusing unattended login; run from an interactive terminal") || called {
		t.Fatalf("err = %v, exit = %d, called = %v", err, ExitCodeFor(err), called)
	}
}

func TestAuthLoginDeviceFlowCompletesAndSaves(t *testing.T) {
	oldTTY, oldEnv, oldStart, oldSave := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc
	t.Cleanup(func() {
		stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc = oldTTY, oldEnv, oldStart, oldSave
	})
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return false }
	polled := false
	startDeviceFlowFunc = func(string) (DeviceFlow, error) {
		return DeviceFlow{
			Code:            "WXYZ-1234",
			VerificationURI: "https://github.com/login/device",
			Poll: func() (string, error) {
				polled = true
				return "token-value", nil
			},
		}, nil
	}
	saved := false
	saveCredentialFunc = func(provider, token string) error {
		saved = provider == "copilot" && token == "token-value"
		return nil
	}
	var out, errOut strings.Builder
	if err := RunAuthLogin("copilot", &out, &errOut, strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if !polled || !saved {
		t.Fatalf("polled = %v, saved = %v", polled, saved)
	}
	if out.String() != "Open https://github.com/login/device and enter code WXYZ-1234.\n" || !strings.Contains(errOut.String(), "waiting for confirmation...") {
		t.Fatalf("stdout = %q, stderr = %q", out.String(), errOut.String())
	}
	if strings.Contains(out.String()+errOut.String(), "token-value") {
		t.Fatal("credential leaked to output")
	}
}

func TestAuthLoginUnsupportedProvider(t *testing.T) {
	oldTTY, oldEnv, oldStart := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc
	t.Cleanup(func() { stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc = oldTTY, oldEnv, oldStart })
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return false }
	called := false
	startDeviceFlowFunc = func(string) (DeviceFlow, error) { called = true; return DeviceFlow{}, nil }
	var out, errOut strings.Builder
	err := RunAuthLogin("claude", &out, &errOut, strings.NewReader(""))
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "not supported until M5") || !strings.Contains(err.Error(), "sign in with the provider's own client") || called {
		t.Fatalf("err = %v, exit = %d, called = %v", err, ExitCodeFor(err), called)
	}
}

func TestAuthLoginFlowError(t *testing.T) {
	oldTTY, oldEnv, oldStart := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc
	t.Cleanup(func() { stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc = oldTTY, oldEnv, oldStart })
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return false }
	startDeviceFlowFunc = func(string) (DeviceFlow, error) { return DeviceFlow{}, errors.New("flow failed") }
	var out, errOut strings.Builder
	err := RunAuthLogin("copilot", &out, &errOut, strings.NewReader(""))
	if ExitCodeFor(err) != 1 || !strings.Contains(err.Error(), "flow failed") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}

func TestAuthLoginPollErrorDoesNotSave(t *testing.T) {
	oldTTY, oldEnv, oldStart, oldSave := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc
	t.Cleanup(func() {
		stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc = oldTTY, oldEnv, oldStart, oldSave
	})
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return false }
	startDeviceFlowFunc = func(string) (DeviceFlow, error) {
		return DeviceFlow{
			Code:            "WXYZ-1234",
			VerificationURI: "https://github.com/login/device",
			Poll:            func() (string, error) { return "", errors.New("poll failed") },
		}, nil
	}
	saved := false
	saveCredentialFunc = func(string, string) error { saved = true; return nil }
	var out, errOut strings.Builder
	err := RunAuthLogin("copilot", &out, &errOut, strings.NewReader(""))
	if ExitCodeFor(err) != 1 || !strings.Contains(err.Error(), "poll failed") || saved {
		t.Fatalf("err = %v, exit = %d, saved = %v", err, ExitCodeFor(err), saved)
	}
}

func TestAuthLoginSaveErrorRedactsToken(t *testing.T) {
	const token = "canary-token-value"
	oldTTY, oldEnv, oldStart, oldSave := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc
	t.Cleanup(func() {
		stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc, saveCredentialFunc = oldTTY, oldEnv, oldStart, oldSave
	})
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return false }
	startDeviceFlowFunc = func(string) (DeviceFlow, error) {
		return DeviceFlow{
			Code:            "WXYZ-1234",
			VerificationURI: "https://github.com/login/device",
			Poll:            func() (string, error) { return token, nil },
		}, nil
	}
	saveCredentialFunc = func(string, string) error { return errors.New("save failed for " + token) }
	var out, errOut strings.Builder
	err := RunAuthLogin("copilot", &out, &errOut, strings.NewReader(""))
	if ExitCodeFor(err) != 1 || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
	if strings.Contains(err.Error()+out.String()+errOut.String(), token) {
		t.Fatal("credential leaked through save error")
	}
}
