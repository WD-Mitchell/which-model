// F29: `hooks` command wiring (specs/features/F29-agent-hooks/SPEC.md
// §1–§4; CONTRACTS §2). Registered in all builds: variant detection goes
// through F21's toggle package, whose nousage stub resolves
// (false, "compiled_out") → variant B (SPEC behaviour 13).
package whichmodel

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/hooks"
	"github.com/WD-Mitchell/which-model/internal/skills"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

func init() { register(NewHooksCmd) }

// ExecuteCommand runs a which-model subcommand in-process, writing its
// stdout/stderr to the given writers and returning the exit code. This is
// the Runner default for `hooks run` (F29 CONTRACTS §1/§5): the underlying
// annex-c command executes in-process, never as a subprocess (SPEC
// behaviour 4/12).
func ExecuteCommand(args []string, stdout, stderr io.Writer) int {
	prevOut, prevErr := Stdout, Stderr
	prevGlobal := Global
	defer func() { Stdout, Stderr, Global = prevOut, prevErr, prevGlobal }()
	// Commands hold mutable flags and parent pointers. Reusing the outer
	// tree would reparent the running hook and redirect its envelope into
	// the nested command's captured stdout.
	resetRegistryBuildCache()
	Stdout, Stderr = stdout, stderr
	return ExecuteArgs(args)
}

// NewHooksCmd builds the `hooks` command tree: install | remove | run
// (annex-d §2.10).
func NewHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Install, remove, or run agent dispatch hooks",
	}
	cmd.AddCommand(newHooksInstallCmd(), newHooksRemoveCmd(), newHooksRunCmd())
	return cmd
}

func hookNames() string {
	names := make([]string, 0, len(hooks.All))
	for _, h := range hooks.All {
		names = append(names, h.ID)
	}
	return strings.Join(names, ", ")
}

func validHooksTarget(target string) bool {
	return target == "claude" || target == "generic"
}

func newHooksInstallCmd() *cobra.Command {
	var target, repo string
	var usage, noUsage bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install agent hooks into the repo config",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if usage && noUsage {
				return &UsageError{Message: "--usage and --no-usage are mutually exclusive"}
			}
			if !validHooksTarget(target) {
				return &UsageError{Message: fmt.Sprintf("unknown target %q (known: claude, generic)", target)}
			}
			skills.SetRepoDir(repo)
			root, err := skills.RepoRoot()
			if err != nil {
				return err // I/O / no repo root → exit 1
			}
			variant := hooks.VariantAuto
			switch {
			case noUsage:
				variant = hooks.VariantNoUsage
			case usage:
				variant = hooks.VariantUsage
			default:
				// Variant detection at install time only (SPEC behaviour 9);
				// under -tags nousage the F21 stub resolves
				// (false, "compiled_out") → variant B.
				cfg, err := config.Load(config.LoadOptions{Path: Global.ConfigPath})
				if err != nil {
					return err
				}
				enabled, _ := toggle.ResolveUsageEnabled(false, cfg)
				if enabled {
					variant = hooks.VariantUsage
				} else {
					variant = hooks.VariantNoUsage
				}
			}
			lines, err := hooks.Install(target, hooks.Installed(variant), root)
			if err != nil {
				return err
			}
			for _, l := range lines {
				fmt.Fprintln(c.OutOrStdout(), l)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "claude", "claude|generic (default claude)")
	cmd.Flags().BoolVar(&usage, "usage", false, "force the usage-enabled variant (all four hooks)")
	cmd.Flags().BoolVar(&noUsage, "no-usage", false, "force the usage-disabled variant (spawn-gate + model-audit)")
	cmd.Flags().StringVar(&repo, "repo", "", "repository root (default: nearest .git ancestor)")
	return cmd
}

func newHooksRemoveCmd() *cobra.Command {
	var target, repo string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove installed agent hooks (owned entries only)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if !validHooksTarget(target) {
				return &UsageError{Message: fmt.Sprintf("unknown target %q (known: claude, generic)", target)}
			}
			skills.SetRepoDir(repo)
			root, err := skills.RepoRoot()
			if err != nil {
				return err // I/O / no repo root → exit 1
			}
			lines, err := hooks.Remove(target, root)
			if err != nil {
				return err
			}
			for _, l := range lines {
				fmt.Fprintln(c.OutOrStdout(), l)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "claude", "claude|generic (default claude)")
	cmd.Flags().StringVar(&repo, "repo", "", "repository root (default: nearest .git ancestor)")
	return cmd
}

func newHooksRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <hook> [args...]",
		Short: "Run one agent hook and print the decision envelope",
		// Passthrough: everything after the hook name belongs to the
		// underlying command (installed variants append e.g. --last,
		// --no-usage --profile balanced_implementation --quiet).
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			// DisableFlagParsing also leaves global flags before the command
			// in args. Parse only that prefix; the hook's own args are opaque.
			prefix := pflag.NewFlagSet("hooks run", pflag.ContinueOnError)
			prefix.SetOutput(io.Discard)
			prefix.AddFlagSet(c.InheritedFlags())
			prefix.SetInterspersed(false)
			if err := prefix.Parse(args); err != nil {
				return &UsageError{Message: err.Error()}
			}
			args = prefix.Args()
			if err := Global.Normalize(); err != nil {
				return err
			}
			if err := Global.Validate(); err != nil {
				return err
			}
			if len(args) == 0 {
				return &UsageError{Message: "missing hook name (known: " + hookNames() + ")"}
			}
			name := args[0]
			if _, ok := hooks.Get(name); !ok {
				return &UsageError{Message: fmt.Sprintf("unknown hook %q (known: %s)", name, hookNames())}
			}
			// Non-empty stdin is host context, never command output.
			// The underlying command always runs (SPEC behaviour 4). Resolve the
			// repo root best-effort: it only matters for model-audit
			// evidence, which fails open without it.
			root, _ := skills.RepoRoot()
			in, err := io.ReadAll(c.InOrStdin())
			if err != nil {
				return &UsageError{Message: err.Error()}
			}
			// Cobra has already consumed flags before `hooks`. Forward those
			// explicit settings too; flags after the hook name still win.
			var passthrough []string
			prefix.Visit(func(flag *pflag.Flag) {
				// Underlying output is always JSON for the decision decoder.
				if flag.Name != "json" && flag.Name != "text" {
					passthrough = append(passthrough, "--"+flag.Name+"="+flag.Value.String())
				}
			})
			passthrough = append(passthrough, args[1:]...)
			out, err := hooks.Run(name, passthrough, hooks.Options{
				Runner:   ExecuteCommand,
				Stdin:    in,
				RepoRoot: root,
			})
			if err != nil {
				// Only reachable here: non-empty stdin that is not valid
				// JSON (unknown hooks were rejected above) → exit 2.
				return &UsageError{Message: err.Error()}
			}
			if len(out) > 0 {
				if _, err := c.OutOrStdout().Write(out); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}
