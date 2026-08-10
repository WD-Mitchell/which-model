//go:build !nousage

package whichmodel

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func init() {
	RegisterExitCode("unsupported", 2)
	register(NewAuthCmd)
}

type AuthStatusArgs struct {
	Providers    []string
	All          bool
	ShowIdentity bool
	JSON         bool
	ConfigPath   string
	NoUsage      bool
}

func NewAuthCmd() *cobra.Command {
	cobra.EnableCommandSorting = false
	cmd := &cobra.Command{
		Use:   "auth status|login|logout",
		Short: "Manage provider credentials",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return &UsageError{Message: "requires a subcommand"}
			}
			return nil
		},
	}
	cmd.AddCommand(newAuthStatusCmd(), newAuthLoginCmd(), newAuthLogoutCmd())
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "status [provider...]",
		Args: cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if err := validateAuthProviders(args); err != nil {
				return err
			}
			return RunAuthStatus(AuthStatusArgs{
				Providers:    args,
				ShowIdentity: Global.ShowIdentity,
				JSON:         Global.JSON,
				ConfigPath:   Global.ConfigPath,
				NoUsage:      Global.NoUsage,
			}, c.OutOrStdout(), c.ErrOrStderr())
		},
	}
}

func newAuthLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "login <provider>",
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if err := validateAuthProviders(args); err != nil {
				return err
			}
			return RunAuthLogin(args[0], c.OutOrStdout(), c.ErrOrStderr(), c.InOrStdin())
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "logout <provider>",
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if err := validateAuthProviders(args); err != nil {
				return err
			}
			yes, err := c.Flags().GetBool("yes")
			if err != nil {
				return &UsageError{Message: err.Error()}
			}
			return RunAuthLogout(args[0], yes, c.OutOrStdout(), c.ErrOrStderr(), c.InOrStdin())
		},
	}
	cmd.Flags().Bool("yes", false, "skip confirmation")
	return cmd
}

func validateAuthProviders(providers []string) error {
	for _, id := range providers {
		if _, err := usage.Get(id); err != nil {
			return &UsageError{Message: fmt.Sprintf("unknown provider %q; valid providers: %s", id, strings.Join(usage.IDs(), ", "))}
		}
	}
	return nil
}

// Temporary T1 logic placeholders; replaced by auth.go in T2.
