//go:build !nousage

package whichmodel

import (
	"strings"
	"testing"
)

func TestAuthCommandRegistered(t *testing.T) {
	for _, cmd := range registeredCommands() {
		if cmd.Name() == "auth" {
			return
		}
	}
	t.Fatal("registeredCommands() does not contain auth")
}

func TestAuthCommandShape(t *testing.T) {
	cmd := NewAuthCmd()
	if cmd.Name() != "auth" || cmd.Use != "auth status|login|logout" {
		t.Fatalf("auth shape = %q %q", cmd.Name(), cmd.Use)
	}
	var names []string
	for _, child := range cmd.Commands() {
		names = append(names, child.Name())
	}
	want := []string{"status", "login", "logout"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("subcommands = %v, want %v", names, want)
	}
}

func TestAuthLogoutYesFlag(t *testing.T) {
	flag := NewAuthCmd().CommandPath()
	_ = flag
	logout, _, err := NewAuthCmd().Find([]string{"logout"})
	if err != nil {
		t.Fatal(err)
	}
	yes := logout.Flags().Lookup("yes")
	if yes == nil || yes.Value.Type() != "bool" || yes.DefValue != "false" {
		t.Fatalf("--yes = %#v", yes)
	}
}

func TestAuthUnknownProvider(t *testing.T) {
	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status", "not-a-provider"})
	err := cmd.Execute()
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "unknown provider") || !strings.Contains(err.Error(), "valid providers:") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}

func TestAuthRequiresSubcommand(t *testing.T) {
	cmd := NewAuthCmd()
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "requires a subcommand") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}
