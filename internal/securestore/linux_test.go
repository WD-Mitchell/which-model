//go:build linux && !nousage

package securestore

import (
	"bytes"
	"context"
	"errors"
	"github.com/godbus/dbus/v5"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinuxErrorStates(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind Kind
	}{{"org.freedesktop.Secret.Error.IsLocked", Locked}, {"org.freedesktop.DBus.Error.AccessDenied", Denied}, {"org.freedesktop.Secret.Error.NoSuchObject", Missing}, {"org.freedesktop.DBus.Error.ServiceUnknown", Unavailable}} {
		value := dbus.Error{Name: tc.name, Body: []interface{}{"SYNTHETIC_SECRET_ERROR"}}
		for _, err := range []error{value, &value} {
			got := classifyLinux(err)
			if !errors.Is(got, &Error{tc.kind}) || strings.Contains(got.Error(), "SYNTHETIC") {
				t.Fatalf("classification: %v", got)
			}
		}
	}
}

func TestLinuxRejectsUnsafeBus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bus")
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := (linuxStore{socket: path}).Get("service", "account"); !errors.Is(err, &Error{Unavailable}) {
		t.Fatalf("absent bus: %v", err)
	}
	if err := os.WriteFile(path, []byte("SYNTHETIC_CANARY"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (linuxStore{socket: path}).Get("service", "account"); !errors.Is(err, &Error{Denied}) {
		t.Fatalf("non-socket bus: %v", err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := (linuxStore{socket: path}).Get("service", "account"); !errors.Is(err, &Error{Denied}) {
		t.Fatalf("exposed bus directory: %v", err)
	}
}

func TestNativeLinuxStore(t *testing.T) {
	nativeTestOnly(t)
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(runtimeDir, "bus")
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "XDG_DATA_HOME=" + filepath.Join(dir, "data"), "XDG_CONFIG_HOME=" + filepath.Join(dir, "config"), "XDG_RUNTIME_DIR=" + runtimeDir, "DBUS_SESSION_BUS_ADDRESS=unix:path=" + socket}
	start := func(name string, args []string, input string) *exec.Cmd {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Env = env
		cmd.Stdin = strings.NewReader(input)
		// Daemon diagnostics are intentionally never printed: native OS text is not
		// an acceptable channel for credentials, even for synthetic fixtures.
		var discard bytes.Buffer
		cmd.Stdout = &discard
		cmd.Stderr = &discard
		if err := cmd.Start(); err != nil {
			t.Fatalf("fixture daemon start: %v", err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		return cmd
	}
	bus := start("/usr/bin/dbus-daemon", []string{"--session", "--nofork", "--nopidfile", "--address=unix:path=" + socket}, "")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture bus did not start")
		}
		time.Sleep(25 * time.Millisecond)
	}
	start("/usr/bin/gnome-keyring-daemon", []string{"--foreground", "--components=secrets", "--unlock"}, "which-model-ci-fixture\n")
	store := linuxStore{socket: socket}
	deadline = time.Now().Add(15 * time.Second)
	for {
		_, err := store.Get("which-model-ci-native", "synthetic-account")
		if errors.Is(err, &Error{Missing}) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fixture secret service did not start: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	roundTrip(t, store, "which-model-ci-native")
	t.Run("default collection changes", func(t *testing.T) {
		testNativeLinuxDefaultChange(t, store)
	})
	if err := store.Set("which-model-ci-native", "locked", "SYNTHETIC_CANARY"); err != nil {
		t.Fatal(err)
	}
	ctx, conn, close, err := store.connect()
	if err != nil {
		t.Fatal(err)
	}
	col, err := collection(ctx, conn)
	if err != nil {
		close()
		t.Fatal(err)
	}
	var locked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	err = conn.Object(secretName, "/org/freedesktop/secrets").CallWithContext(ctx, secretService+".Lock", 0, []dbus.ObjectPath{col}).Store(&locked, &prompt)
	close()
	if err != nil || len(locked) != 1 || prompt != "/" {
		t.Fatal("fixture collection did not lock")
	}
	if _, err := store.Get("which-model-ci-native", "locked"); !errors.Is(err, &Error{Locked}) {
		t.Fatalf("locked read: %v", err)
	}
	if err := store.Set("which-model-ci-native", "locked", "replacement"); !errors.Is(err, &Error{Locked}) {
		t.Fatalf("locked write: %v", err)
	}
	if err := store.Delete("which-model-ci-native", "locked"); !errors.Is(err, &Error{Locked}) {
		t.Fatalf("locked delete: %v", err)
	}
	_ = bus.Process.Kill()
	unavailableCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := store.Get("which-model-ci-native", "locked"); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, &Error{Unavailable}) {
			t.Fatalf("unavailable bus: %v", err)
		}
	case <-unavailableCtx.Done():
		t.Fatal("unavailable bus exceeded bound")
	}
}

func testNativeLinuxDefaultChange(t *testing.T, store linuxStore) {
	t.Helper()
	ctx, conn, close, err := store.connect()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	original, err := collection(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	// The disposable GNOME fixture also provides an unlocked session collection.
	var other dbus.ObjectPath
	service := conn.Object(secretName, "/org/freedesktop/secrets")
	if err := service.CallWithContext(ctx, secretService+".ReadAlias", 0, "session").Store(&other); err != nil || other == "/" || !other.IsValid() || other == original {
		t.Fatalf("fixture needs two distinct collections: %v", err)
	}
	const serviceName = "which-model-ci-default-change"
	if err := store.Set(serviceName, "account", "SYNTHETIC_BEFORE"); err != nil {
		t.Fatal(err)
	}
	if err := service.CallWithContext(ctx, secretService+".SetAlias", 0, "default", other).Err; err != nil {
		t.Fatal("fixture could not change default collection")
	}
	defer func() {
		if err := service.CallWithContext(ctx, secretService+".SetAlias", 0, "default", original).Err; err != nil {
			t.Error("fixture could not restore default collection")
		}
	}()
	if value, err := store.Get(serviceName, "account"); err != nil || value != "SYNTHETIC_BEFORE" {
		t.Fatalf("original entry unavailable after default change: %v", err)
	}
	if err := store.Set(serviceName, "account", "SYNTHETIC_AFTER"); err != nil {
		t.Fatal(err)
	}
	if value, err := store.Get(serviceName, "account"); err != nil || value != "SYNTHETIC_AFTER" {
		t.Fatalf("updated entry unavailable after default change: %v", err)
	}
	if err := store.Delete(serviceName, "account"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(serviceName, "account"); !errors.Is(err, &Error{Missing}) {
		t.Fatalf("deletion left an entry: %v", err)
	}
}
