package service

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/approvedexec"
)

func TestApprovedCopyChildHelper(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--" && len(os.Args) > i+1 {
			dir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(struct {
				Args []string
				Dir  string
			}{os.Args[i+2:], dir})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(os.Args[i+1], data, 0600); err != nil {
				t.Fatal(err)
			}
			os.Stdout.WriteString("COPY_STDOUT_CANARY")
			os.Exit(17)
		}
	}
}

func TestCompanyApprovedCopyPowerShellArguments(t *testing.T) {
	image, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(image)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	// Exercise quoting of the executable path as well as its arguments.
	image = filepath.Join(t.TempDir(), "native 'copy' fixture.exe")
	output, err := os.Create(image)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy fixture: %v, %v", copyErr, closeErr)
	}
	want := []string{"", `a"b`, `{"key":"value"}`, "apostrophe'value", `C:\space path\`, `backslash\"quote`, "two words", "tab\there", "line\nbreak", "$env:HOME; & echo injected", "unicodé-日本語", "smart‘’‚‛quotes", ""}
	for _, shell := range []string{"powershell.exe", "pwsh.exe"} {
		shellPath, err := exec.LookPath(shell)
		if err != nil {
			// Both versions are required by the Windows native CI fixture.
			if os.Getenv("GITHUB_ACTIONS") == "true" {
				t.Fatal(err)
			}
			t.Logf("%s unavailable: %v", shell, err)
			continue
		}
		for _, mode := range []string{"Legacy", "Standard"} {
			t.Run(shell+"/"+mode, func(t *testing.T) {
				project := t.TempDir()
				resultPath := filepath.Join(project, "child 'argv'.json")
				args := append([]string{"-test.run=^TestApprovedCopyChildHelper$", "--", resultPath}, want...)
				command := displayApprovedCommand(approvedexec.Plan{Path: image, Args: args})
				command = "$PSNativeCommandArgumentPassing = '" + mode + "'; Set-Location -LiteralPath '" + strings.ReplaceAll(project, "'", "''") + "'; " + command + "; exit $LASTEXITCODE"
				cmd := exec.Command(shellPath, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command)
				if data, err := cmd.CombinedOutput(); err == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 17 || !strings.Contains(string(data), "COPY_STDOUT_CANARY") {
					t.Fatalf("copied command lost native exit status: %v, %s", err, data)
				}
				data, err := os.ReadFile(resultPath)
				if err != nil {
					t.Fatal(err)
				}
				var got struct {
					Args []string
					Dir  string
				}
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatal(err)
				}
				wantDir, wantErr := os.Stat(project)
				gotDir, gotErr := os.Stat(got.Dir)
				if !reflect.DeepEqual(got.Args, want) || wantErr != nil || gotErr != nil || !os.SameFile(wantDir, gotDir) {
					t.Fatalf("approved argv or project changed: got %+v, want %q in %q", got, want, project)
				}
			})
		}
	}
}
