//go:build !nousage

package whichmodel

import (
	"fmt"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/output"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"github.com/spf13/cobra"
)

func init() { register(newPrivacyCmd) }

func newPrivacyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "privacy", Short: "Inspect company retention or clean up owned product data"}
	for _, operation := range []string{"status", "cleanup", "purge"} {
		var project string
		var selected []string
		child := &cobra.Command{Use: operation, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			policy, err := readCompanyPolicy()
			if err != nil {
				return err
			}
			ctl := privacy.Controller{Policy: policy}
			if operation == "status" {
				var settings any
				if policy.Managed {
					settings = map[string]any{"identity_free": policy.Policy.IdentityFree, "retention": policy.Policy.Retention}
				}
				return output.RenderJSON(cmd.OutOrStdout(), output.OutputEnvelope{}, map[string]any{"managed": policy.Managed, "settings": settings, "offline_deletion": "requires scheduled cleanup while the application is stopped"})
			}
			layout, err := privacy.DefaultLayout()
			if err != nil {
				return err
			}
			if project != "" {
				layout.ProjectRoot = project
			}
			categories := []privacy.Category{}
			for _, value := range selected {
				switch value {
				case "usage":
					categories = append(categories, privacy.Usage)
				case "history":
					categories = append(categories, privacy.History)
				case "audit":
					categories = append(categories, privacy.Audit)
				case "launch":
					categories = append(categories, privacy.Launch)
				default:
					return &UsageError{Message: "category must be usage, history, audit or launch"}
				}
			}
			if operation == "purge" && !ctl.Enabled() {
				// Explicit deletion is available to personal users without enrolling them
				// or enabling automatic retention. The duration values are unused by purge.
				p := company.Defaults()
				ctl.Policy = company.Snapshot{Managed: true, Policy: &p}
			}
			summary, err := ctl.Maintain(layout, operation == "purge", categories)
			summary.Managed = policy.Managed
			if writeErr := output.RenderJSON(cmd.OutOrStdout(), output.OutputEnvelope{}, summary); writeErr != nil {
				return writeErr
			}
			if err != nil {
				return &ReportedError{Err: err}
			}
			return nil
		}}
		if operation != "status" {
			child.Flags().StringVar(&project, "project-root", "", "also clean the two owned legacy audit files in this project")
			child.Flags().StringSliceVar(&selected, "category", nil, "select usage, history, audit or launch (default: all)")
		}
		cmd.AddCommand(child)
	}
	return cmd
}

func startupPrivacyMaintenance(c *cobra.Command) {
	if Global.NoUsage || c.Name() == "version" {
		return
	}
	for parent := c; parent != nil; parent = parent.Parent() {
		if parent.Name() == "privacy" || parent.Name() == "config" {
			return
		}
	}
	policy, err := readCompanyPolicy()
	if err != nil || !policy.Managed {
		return
	}
	layout, err := privacy.DefaultLayout()
	if err == nil {
		_, err = (privacy.Controller{Policy: policy}).Maintain(layout, false, nil)
	}
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(), "warning: company privacy maintenance incomplete; run privacy cleanup for category results")
	}
}
