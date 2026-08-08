package whichmodel

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTree(t *testing.T) {
	t.Run("order respects commandOrder at F22 completion", func(t *testing.T) {
		var names []string
		for _, c := range registeredCommands() {
			names = append(names, c.Name())
		}
		want := []string{"schema", "serve", "config", "version"}
		if !reflect.DeepEqual(names, want) {
			t.Errorf("registeredCommands names = %v, want %v", names, want)
		}
	})

	t.Run("unknown names sort last then alphabetically", func(t *testing.T) {
		register(func() *cobra.Command { return &cobra.Command{Use: "zzz"} })
		register(func() *cobra.Command { return &cobra.Command{Use: "aaa"} })
		names := make([]string, 0)
		for _, c := range registeredCommands() {
			names = append(names, c.Name())
		}
		ia, iz := -1, -1
		for i, n := range names {
			switch n {
			case "aaa":
				ia = i
			case "zzz":
				iz = i
			}
		}
		if ia < 0 || iz < 0 {
			t.Fatalf("registered test commands missing: %v", names)
		}
		if !(ia < iz) {
			t.Errorf("aaa must sort before zzz: %v", names)
		}
		iv := -1
		for i, n := range names {
			if n == "version" {
				iv = i
			}
		}
		if !(iv < ia && iv < iz) {
			t.Errorf("unknown names must sort after version: %v", names)
		}
	})

	t.Run("built once", func(t *testing.T) {
		a := registeredCommands()
		b := registeredCommands()
		if &a[0] != &b[0] {
			t.Error("consecutive registeredCommands calls must return the identical slice")
		}
	})

	t.Run("root includes registry", func(t *testing.T) {
		root := NewRootCmd()
		have := map[string]bool{}
		for _, c := range root.Commands() {
			have[c.Name()] = true
		}
		for _, name := range []string{"schema", "serve", "config", "version"} {
			if !have[name] {
				t.Errorf("root missing command %q (has %v)", name, have)
			}
		}
	})

	t.Run("serve refusal exit", func(t *testing.T) {
		code, _, errOut := captureExecute(t, []string{"serve"})
		if code != 1 {
			t.Errorf("exit = %d, want 1", code)
		}
		if !strings.Contains(errOut, "[serve_unavailable]") {
			t.Errorf("stderr = %q, want [serve_unavailable]", errOut)
		}
	})

	t.Run("serve flags parse before refusal", func(t *testing.T) {
		code, _, _ := captureExecute(t, []string{"serve", "--interval", "1m"})
		if code != 1 {
			t.Errorf("exit = %d, want 1 (refusal after parse)", code)
		}
	})

	t.Run("serve help shows flags", func(t *testing.T) {
		code, out, _ := captureExecute(t, []string{"serve", "--help"})
		if code != 0 {
			t.Errorf("exit = %d, want 0", code)
		}
		if !strings.Contains(out, "--listen") {
			t.Errorf("serve --help missing --listen: %q", out)
		}
	})

	t.Run("register is additive", func(t *testing.T) {
		register(func() *cobra.Command { return &cobra.Command{Use: "usage"} })
		cmds := registeredCommands()
		if cmds[0].Name() != "usage" {
			t.Errorf("after registering usage, first command = %q, want usage", cmds[0].Name())
		}
	})
}
