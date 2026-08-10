//go:build !nousage

package whichmodel

import (
	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func init() { register(NewUsageCmd) }

func NewUsageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage [provider...]",
		Short: "Report provider usage allowances",
		RunE:  runUsageE,
	}
	cmd.Flags().Bool("all", false, "report every enabled provider")
	cmd.Flags().String("source", "", "force a usage source")
	return cmd
}

func runUsageE(c *cobra.Command, args []string) error {
	all, err := c.Flags().GetBool("all")
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	source, err := c.Flags().GetString("source")
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	return RunUsage(UsageArgs{
		Providers:    args,
		All:          all,
		Source:       usage.Source(source),
		MaxAge:       Global.MaxAge,
		ForceRefresh: Global.RefreshUsage,
		Timeout:      Global.Timeout,
		Offline:      Global.Offline,
		ShowIdentity: Global.ShowIdentity,
		JSON:         Global.JSON,
		ConfigPath:   Global.ConfigPath,
		NoUsage:      Global.NoUsage,
	}, c.OutOrStdout(), c.ErrOrStderr())
}
