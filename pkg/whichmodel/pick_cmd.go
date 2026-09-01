// F26: `pick` command wiring (specs/features/F26-cmd-pick/). Registered in
// all builds (NO build tag): degraded usage behavior is SPEC §2.4 and the
// toggle stub resolves (false, "compiled_out") under -tags nousage.
package whichmodel

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	// Exit codes per F26 CONTRACTS §4 — registered before the command so
	// ExitCodeFor resolves them from the first call.
	RegisterExitCode("no_pick", 3)
	RegisterExitCode("usage_gated", 4)
	RegisterExitCode("auth_required", 5)
	// Strict no_providers misconfiguration (SPEC §2.14). usage_disabled is
	// already in the global §1.6 table (exit 2).
	RegisterExitCode("usage_config", 2)
	register(NewPickCmd)
}

// NewPickCmd builds the `pick` command (F26 CONTRACTS §3).
func NewPickCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Pick a model for a task profile",
		Args:  cobra.NoArgs,
		RunE:  runPickE,
	}
	cmd.Flags().String("profile", "", "profile id (one of the eleven annex-c §2.1 names)")
	cmd.Flags().String("task-category", "", "task category selector, paired with --complexity")
	cmd.Flags().String("complexity", "", "task complexity for --task-category (simple|medium|complex)")
	cmd.Flags().String("strategy", "", "strategy name (default: closest-to-reset with usage, priority without)")
	cmd.Flags().StringSlice("available", nil, "allowlist file of model ids (repeatable)")
	return cmd
}

func runPickE(c *cobra.Command, args []string) error {
	profile, _ := c.Flags().GetString("profile")
	taskCategory, _ := c.Flags().GetString("task-category")
	complexity, _ := c.Flags().GetString("complexity")
	strategy, _ := c.Flags().GetString("strategy")
	available, _ := c.Flags().GetStringSlice("available")
	out := c.OutOrStdout()
	return RunPick(PickArgs{
		Profile:      profile,
		TaskCategory: taskCategory,
		Complexity:   complexity,
		Strategy:     strategy,
		Allowlists:   available,
		NoUsage:      Global.NoUsage,
		JSON:         Global.JSON || !stdoutIsTTY(out),
		ConfigPath:   Global.ConfigPath,
	}, out, c.ErrOrStderr())
}

// stdoutIsTTY reports whether w is a terminal. Non-TTY stdout forces JSON
// output (annex-c §4.6 agent mode; SPEC D-8). Injectable for tests — a
// *bytes.Buffer is never a terminal.
var stdoutIsTTY = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// PickArgs is the fully-validated pick command input (F26 CONTRACTS §2).
type PickArgs struct {
	Profile      string   // resolved profile id (after --task-category mapping)
	TaskCategory string   // raw --task-category (resolved in T2)
	Complexity   string   // raw --complexity
	Strategy     string   // blank selects closest-to-reset with usage, priority without
	Allowlists   []string // --available paths (raw, pre-read)
	NoUsage      bool     // Global.NoUsage
	JSON         bool     // Global.JSON; forced true when stdout is not a TTY
	ConfigPath   string   // Global.ConfigPath
}
