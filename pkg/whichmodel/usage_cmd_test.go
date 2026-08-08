//go:build !nousage

package whichmodel

import (
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestUsageCommandRegistered(t *testing.T) {
	for _, cmd := range registeredCommands() {
		if cmd.Name() == "usage" {
			return
		}
	}
	t.Fatal("registeredCommands() does not contain usage")
}

func TestUsageCommandShape(t *testing.T) {
	cmd := NewUsageCmd()
	if cmd.Use != "usage [provider...]" {
		t.Fatalf("Use = %q, want usage [provider...]", cmd.Use)
	}
	all := cmd.Flags().Lookup("all")
	if all == nil || all.Value.Type() != "bool" || all.DefValue != "false" {
		t.Fatalf("--all flag = %#v, want bool false", all)
	}
	source := cmd.Flags().Lookup("source")
	if source == nil || source.Value.Type() != "string" || source.DefValue != "" {
		t.Fatalf("--source flag = %#v, want string empty", source)
	}
}

func TestUsageRunENoProviders(t *testing.T) {
	Global = GlobalFlags{}
	t.Cleanup(func() { Global = GlobalFlags{} })
	cmd := NewUsageCmd()
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "no providers requested") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}

func TestUsageRunEAllAndProvider(t *testing.T) {
	Global = GlobalFlags{}
	t.Cleanup(func() { Global = GlobalFlags{} })
	cmd := NewUsageCmd()
	cmd.SetArgs([]string{"--all", "claude"})
	err := cmd.Execute()
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}

func TestUsageRunEUnknownProvider(t *testing.T) {
	Global = GlobalFlags{}
	t.Cleanup(func() { Global = GlobalFlags{} })
	_, _ = usage.Get("claude")
	cmd := NewUsageCmd()
	cmd.SetArgs([]string{"not-a-provider"})
	err := cmd.Execute()
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "unknown provider") || !strings.Contains(err.Error(), "valid providers:") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}
