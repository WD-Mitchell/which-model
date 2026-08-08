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

func TestAuthLoginDeviceFlowPrompt(t *testing.T) {
	oldTTY, oldEnv, oldStart := stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc
	t.Cleanup(func() { stdinIsTTY, nonInteractiveEnv, startDeviceFlowFunc = oldTTY, oldEnv, oldStart })
	stdinIsTTY = func() bool { return true }
	nonInteractiveEnv = func() bool { return false }
	startDeviceFlowFunc = func(string) (DeviceFlow, error) {
		return DeviceFlow{Code: "WXYZ-1234", VerificationURI: "https://github.com/login/device"}, nil
	}
	var out, errOut strings.Builder
	if err := RunAuthLogin("copilot", &out, &errOut, strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Open https://github.com/login/device and enter code WXYZ-1234.\n" || !strings.Contains(errOut.String(), "waiting for confirmation...") {
		t.Fatalf("stdout = %q, stderr = %q", out.String(), errOut.String())
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
