// Package whichmodel implements the which-model CLI command tree
// (specs/features/F22-cli-skeleton/SPEC.md). The binary entrypoint is
// cmd/which-model; all behaviour lives here. argv[0] is never inspected —
// the display name is fixed (annex-d §1.1a).
package whichmodel

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/output"
)

const rootUse = "which-model"

// Stdout and Stderr are the CLI's output streams; tests swap them.
var (
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)

// NewRootCmd builds the root command with the global persistent flags and
// the registered command tree (SPEC §2, §3). Help is ordered by
// commandOrder, so cobra's default command sorting is disabled.
func NewRootCmd() *cobra.Command {
	cobra.EnableCommandSorting = false
	cmd := &cobra.Command{
		Use:           rootUse,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return &UsageError{Message: err.Error()}
	})
	cmd.SetOut(Stdout)
	cmd.SetErr(Stderr)
	if err := Global.Bind(cmd); err != nil {
		panic(err)
	}
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if err := Global.Normalize(); err != nil {
			return err
		}
		return Global.Validate()
	}
	cmd.AddCommand(registeredCommands()...)
	return cmd
}

// Execute runs the CLI with os.Args[1:] and returns the process exit code.
func Execute() int { return ExecuteArgs(os.Args[1:]) }

// ExecuteArgs runs the CLI with the given args and returns the exit code
// (SPEC §2–§9): --version/--schema pre-scan, Normalize+Validate, cobra
// execution, and unified failure rendering.
func ExecuteArgs(args []string) int {
	if args == nil {
		args = []string{}
	}
	if handled, code := versionShortCircuit(args); handled {
		return code
	}
	if handled, code := schemaShortCircuit(args); handled {
		return code
	}
	// --version=false is ignored by the pre-scan and is not a registered
	// flag, so drop it before cobra rejects it (SPEC §7).
	args = stripToken(args, "--version=false")
	root := NewRootCmd()
	root.SetArgs(args)
	if err := Global.Normalize(); err != nil {
		return renderError(nil, err)
	}
	if err := Global.Validate(); err != nil {
		return renderError(nil, err)
	}
	found, err := root.ExecuteC()
	if err == nil {
		// The bare root has no Run, so cobra never invokes
		// PersistentPreRunE; re-validate after flag parse so contradictory
		// flags fail even without a subcommand.
		if verr := Global.Validate(); verr != nil {
			return renderError(found, verr)
		}
	}
	return renderError(found, err)
}

// stripToken removes exact token occurrences before the first "--"
// terminator.
func stripToken(args []string, token string) []string {
	out := make([]string, 0, len(args))
	for i, a := range args {
		if a == "--" {
			out = append(out, args[i:]...)
			return out
		}
		if a != token {
			out = append(out, a)
		}
	}
	return out
}

// versionShortCircuit pre-scans raw args (before any "--" terminator) for
// --version / --version=true (SPEC §7); --version=false is ignored.
func versionShortCircuit(args []string) (bool, int) {
	version, jsonOut := false, false
	for _, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "--version", "--version=true":
			version = true
		case "--json":
			jsonOut = true
		}
	}
	if !version {
		return false, 0
	}
	if jsonOut {
		output.RenderJSON(Stdout, output.OutputEnvelope{}, VersionJSON())
	} else {
		fmt.Fprintln(Stdout, VersionLine())
	}
	return true, 0
}

// schemaShortCircuit pre-scans raw args for --schema / --schema=true
// (SPEC §8). The token is removed, the remaining args resolve the command
// path, and that command's schema document is printed — the command itself
// is never executed.
func schemaShortCircuit(args []string) (bool, int) {
	pos := -1
	for i, a := range args {
		if a == "--" {
			break
		}
		if a == "--schema" || a == "--schema=true" {
			pos = i
			break
		}
	}
	if pos < 0 {
		return false, 0
	}
	remaining := make([]string, 0, len(args)-1)
	remaining = append(remaining, args[:pos]...)
	remaining = append(remaining, args[pos+1:]...)
	if len(remaining) == 0 {
		if err := output.PrintSchemaIndex(Stdout, SchemaIndex()); err != nil {
			return true, renderError(nil, err)
		}
		return true, 0
	}
	root := NewRootCmd()
	found, _, err := root.Find(remaining)
	if err != nil {
		return true, renderError(nil, err)
	}
	path := strings.TrimPrefix(found.CommandPath(), rootUse)
	path = strings.TrimPrefix(path, " ")
	doc, ok := lookupSchema(path)
	if !ok {
		return true, renderError(found, &UsageError{Message: fmt.Sprintf("no schema for command %q", path)})
	}
	if err := output.PrintSchema(Stdout, doc); err != nil {
		return true, renderError(found, err)
	}
	return true, 0
}

// renderError renders one failure exactly once (SPEC §6): the failure line
// to Stderr via F03, plus the JSON error document to Stdout under --json
// (never for *ReportedError, whose payload the command already wrote).
func renderError(found *cobra.Command, err error) int {
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if strings.HasPrefix(err.Error(), `unknown command "`) {
		err = &UsageError{Message: err.Error()}
	}
	code := CodeFor(err)
	output.WriteFailure(Stderr, commandLabel(found, err), code, err.Error())
	if Global.JSON {
		var reported *ReportedError
		if !errors.As(err, &reported) {
			output.RenderJSON(Stdout, output.OutputEnvelope{}, map[string]any{
				"error": map[string]string{"code": code, "message": err.Error()},
			})
		}
	}
	return ExitCodeFor(err)
}

// commandLabel is the "<command>" component of the failure line: the unknown
// command's name for cobra's unknown-command errors, the executed command's
// path otherwise.
func commandLabel(found *cobra.Command, err error) string {
	msg := err.Error()
	if strings.HasPrefix(msg, `unknown command "`) {
		rest := msg[len(`unknown command "`):]
		if end := strings.IndexByte(rest, '"'); end > 0 {
			return rest[:end]
		}
	}
	if found != nil && found.Name() != rootUse {
		return strings.TrimPrefix(found.CommandPath(), rootUse+" ")
	}
	return rootUse
}
