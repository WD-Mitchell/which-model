package whichmodel

import (
	"time"

	"github.com/spf13/cobra"
)

func init() { register(newServeCmd) }

// newServeCmd is the serve placeholder (SPEC §10): the body refuses until a
// later milestone assigns the usage-cache server. Not in the fixed
// constructor list, so registered via register() in this F22-owned file.
func newServeCmd() *cobra.Command {
	var (
		warm     bool
		interval time.Duration
		listen   string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "serve the usage cache over HTTP (placeholder)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return &CodedError{Code: "serve_unavailable", Message: "serve is not available in this build; it requires the usage cache subsystem (F13) which lands in a later milestone"}
		},
	}
	cmd.Flags().BoolVar(&warm, "warm", false, "warm the cache on start")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "refresh interval")
	cmd.Flags().StringVar(&listen, "listen", ":8099", "listen address")
	return cmd
}
