package whichmodel

import (
	"strings"
	"testing"
)

func TestCatalogCmdRegistered(t *testing.T) {
	found := false
	for _, c := range NewRootCmd().Commands() {
		if c.Name() == "catalog" {
			found = true
		}
	}
	if !found {
		t.Error("catalog not registered in root tree")
	}
}

func TestCatalogCmdOrderPosition(t *testing.T) {
	names := make([]string, 0)
	for _, c := range NewRootCmd().Commands() {
		names = append(names, c.Name())
	}
	catalogIdx, schemaIdx, serveIdx, configIdx, versionIdx := -1, -1, -1, -1, -1
	for i, n := range names {
		switch n {
		case "catalog":
			catalogIdx = i
		case "schema":
			schemaIdx = i
		case "serve":
			serveIdx = i
		case "config":
			configIdx = i
		case "version":
			versionIdx = i
		}
	}
	if catalogIdx < 0 {
		t.Fatal("catalog not found")
	}
	for _, idx := range []int{schemaIdx, serveIdx, configIdx, versionIdx} {
		if idx >= 0 && catalogIdx >= idx {
			t.Errorf("catalog index %d must be before %d", catalogIdx, idx)
		}
	}
}

func TestCatalogBareHelp(t *testing.T) {
	code, _, _ := captureExecuteFresh(t, []string{"catalog"})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestCatalogUnknownSubcommand(t *testing.T) {
	code, _, stderr := captureExecuteFresh(t, []string{"catalog", "nosuch"})
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "[arguments]") {
		t.Errorf("stderr = %q, want it to contain [arguments]", stderr)
	}
}

func TestCatalogCmdUse(t *testing.T) {
	if got := NewCatalogCmd().Use; got != "catalog" {
		t.Errorf("Use = %q, want catalog", got)
	}
}
