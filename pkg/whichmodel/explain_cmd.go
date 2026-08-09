// F26: `explain` command wiring (specs/features/F26-cmd-pick/). Registered
// in all builds (NO build tag) like `pick`.
package whichmodel

import (
	"github.com/spf13/cobra"
)

func init() {
	register(NewExplainCmd)
}

// NewExplainCmd builds the `explain` command (F26 CONTRACTS §3).
func NewExplainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain a previous pick",
		RunE:  runExplainE,
	}
	cmd.Flags().Bool("last", false, "explain the most recent pick record")
	cmd.Flags().String("pick-id", "", "history record ULID to explain")
	return cmd
}

func runExplainE(c *cobra.Command, args []string) error {
	last, _ := c.Flags().GetBool("last")
	pickID, _ := c.Flags().GetString("pick-id")
	out := c.OutOrStdout()
	return RunExplain(ExplainArgs{
		Last:       last,
		PickID:     pickID,
		JSON:       Global.JSON || !stdoutIsTTY(out),
		ConfigPath: Global.ConfigPath,
	}, out, c.ErrOrStderr())
}
