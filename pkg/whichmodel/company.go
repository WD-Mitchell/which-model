package whichmodel

import (
	"encoding/json"
	"fmt"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/output"
	"github.com/spf13/cobra"
)

var readCompanyPolicy = company.Load

func requireCompanyCapability(capability string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	return policy.RequireCapability(capability)
}

func requireCompanyProvider(provider string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	return policy.RequireProvider(provider)
}

func newConfigPolicyCmd() *cobra.Command {
	return &cobra.Command{Use: "policy", Short: "inspect administrator policy and enrollment", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := readCompanyPolicy()
			if err != nil {
				return err
			}
			if Global.JSON {
				return output.RenderJSON(Stdout, output.OutputEnvelope{}, state)
			}
			data, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(Stdout, string(data))
			return err
		}}
}
