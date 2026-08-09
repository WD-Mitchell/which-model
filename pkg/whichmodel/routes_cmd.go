// F27: the `routes` command tree (specs/features/F27-cmd-routes/SPEC.md §2.1).
// Registered in ALL builds (no build tag) — the command is not usage-touching
// in the toggle sense and must run under -tags nousage (SPEC §2.6).
package whichmodel

import (
	"github.com/spf13/cobra"
)

func init() { register(NewRoutesCmd) }

// NewRoutesCmd builds the routes command with its five subcommands
// list|add|remove|refresh|verify (SPEC §2.1).
func NewRoutesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes list|add|remove|refresh|verify",
		Short: "Manage the provider route table",
	}

	list := &cobra.Command{
		Use:   "list [--provider <id>]",
		Short: "List the provider route table",
		Args:  cobra.NoArgs,
		RunE:  runRouteListE,
	}
	list.Flags().String("provider", "", "filter by provider id")

	add := &cobra.Command{
		Use:   "add --provider <id> --model-id <model_id> --model <name> [--reasoning <level>] [--window <id>...]",
		Short: "Add a user-declared route",
		Args:  cobra.NoArgs,
		RunE:  runRouteAddE,
	}
	add.Flags().String("provider", "", "registry provider id")
	add.Flags().String("model-id", "", "provider-side model id")
	add.Flags().String("model", "", "scored catalog model name")
	add.Flags().String("reasoning", "default", "score-row reasoning key")
	add.Flags().StringSlice("window", []string{}, "usage window id gating this route (repeatable)")

	remove := &cobra.Command{
		Use:   "remove --provider <id> --model-id <model_id>",
		Short: "Remove a route",
		Args:  cobra.NoArgs,
		RunE:  runRouteRemoveE,
	}
	remove.Flags().String("provider", "", "registry provider id")
	remove.Flags().String("model-id", "", "provider-side model id")

	refresh := &cobra.Command{
		Use:   "refresh [--auto <fuzzy-name>]",
		Short: "Regenerate the route table from production sources",
		Args:  cobra.NoArgs,
		RunE:  runRouteRefreshE,
	}
	refresh.Flags().String("auto", "", "fuzzy model name; adds one user-declared route")

	verify := &cobra.Command{
		Use:   "verify",
		Short: "Verify routes against the current scores CSV",
		Args:  cobra.NoArgs,
		RunE:  runRouteVerifyE,
	}

	cmd.AddCommand(list, add, remove, refresh, verify)
	return cmd
}

func runRouteAddE(c *cobra.Command, _ []string) error {
	provider, _ := c.Flags().GetString("provider")
	modelID, _ := c.Flags().GetString("model-id")
	model, _ := c.Flags().GetString("model")
	reasoning, _ := c.Flags().GetString("reasoning")
	windows, _ := c.Flags().GetStringSlice("window")
	return RunRouteAdd(RouteAddArgs{
		Provider:   provider,
		ModelID:    modelID,
		Model:      model,
		Reasoning:  reasoning,
		Windows:    windows,
		ConfigPath: Global.ConfigPath,
	}, c.OutOrStdout(), c.ErrOrStderr())
}

func runRouteRemoveE(c *cobra.Command, _ []string) error {
	provider, _ := c.Flags().GetString("provider")
	modelID, _ := c.Flags().GetString("model-id")
	return RunRouteRemove(RouteRemoveArgs{
		Provider:   provider,
		ModelID:    modelID,
		ConfigPath: Global.ConfigPath,
	}, c.OutOrStdout(), c.ErrOrStderr())
}

func runRouteListE(c *cobra.Command, _ []string) error {
	provider, _ := c.Flags().GetString("provider")
	return RunRouteList(RouteListArgs{
		Provider:   provider,
		JSON:       Global.JSON,
		ConfigPath: Global.ConfigPath,
	}, c.OutOrStdout(), c.ErrOrStderr())
}

func runRouteRefreshE(c *cobra.Command, _ []string) error {
	auto, _ := c.Flags().GetString("auto")
	return RunRouteRefresh(RouteRefreshArgs{
		Auto:       auto,
		ConfigPath: Global.ConfigPath,
	}, c.OutOrStdout(), c.ErrOrStderr())
}

func runRouteVerifyE(c *cobra.Command, _ []string) error {
	return RunRouteVerify(RouteVerifyArgs{
		JSON:       Global.JSON,
		ConfigPath: Global.ConfigPath,
	}, c.OutOrStdout(), c.ErrOrStderr())
}
