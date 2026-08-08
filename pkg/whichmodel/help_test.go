package whichmodel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpGolden(t *testing.T) {
	want, err := os.ReadFile(helpGoldenPath)
	if err != nil {
		t.Fatalf("golden file: %v (regenerate with: go run ./cmd/which-model --help > pkg/whichmodel/%s)", err, helpGoldenPath)
	}
	code, out, _ := captureExecute(t, []string{"--help"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != string(want) {
		t.Errorf("help output differs from golden:\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
	for i, name := range wantTreeOrder {
		if !strings.Contains(out, name) {
			t.Errorf("help missing command %q (position %d)", name, i)
		}
	}
}

// TestAliasInvariance proves argv[0] is never inspected: the binary and its
// install-time symlinks (wm, wmodel, whichm) produce byte-identical output
// (annex-d §1.1a).
func TestAliasInvariance(t *testing.T) {
	rootDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "which-model")
	build := exec.Command("go", "build", "-o", bin, "./cmd/which-model")
	build.Dir = rootDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	names := []string{"which-model", "wm", "wmodel", "whichm"}
	paths := map[string]string{"which-model": bin}
	for _, alias := range names[1:] {
		link := filepath.Join(dir, alias)
		if err := os.Symlink(bin, link); err != nil {
			t.Fatal(err)
		}
		paths[alias] = link
	}

	for _, args := range [][]string{{"--help"}, {"version"}} {
		var first string
		var firstName string
		for _, name := range names {
			out, err := exec.Command(paths[name], args...).CombinedOutput()
			if err != nil {
				t.Fatalf("%s %v: %v\n%s", name, args, err, out)
			}
			if first == "" {
				first = string(out)
				firstName = name
				if args[0] == "version" && !strings.HasPrefix(first, "which-model ") {
					t.Errorf("version stdout must start with `which-model `, got %q", first)
				}
				continue
			}
			if string(out) != first {
				t.Errorf("%s %v output differs from %s:\n%s\nvs\n%s", name, args, firstName, out, first)
			}
		}
	}

	// Failure output self-identifies as which-model under any alias.
	cmd := exec.Command(paths["wm"], "nosuchcmd")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("wm nosuchcmd must fail")
	}
	stderr := string(out)
	if !strings.HasPrefix(stderr, "which-model nosuchcmd:") {
		t.Errorf("wm failure stderr = %q, want prefix `which-model nosuchcmd:`", stderr)
	}
}
