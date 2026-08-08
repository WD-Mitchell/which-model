package whichmodel

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/output"
)

// ldflags targets: -X github.com/WD-Mitchell/which-model/pkg/whichmodel.Version, etc.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func init() {
	register(NewVersionCmd)
	RegisterSchema("version", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"schema_version": map[string]any{"const": "2.0"},
			"version":        map[string]any{"type": "string"},
			"commit":         map[string]any{"type": "string"},
			"built_at":       map[string]any{"type": "string"},
		},
	})
}

// VersionLine renders the canonical one-line version string.
func VersionLine() string {
	return fmt.Sprintf("which-model %s (commit %s, built %s)", Version, Commit, BuildDate)
}

// VersionJSON renders the --json payload.
func VersionJSON() map[string]string {
	return map[string]string{"version": Version, "commit": Commit, "built_at": BuildDate}
}

// NewVersionCmd prints version information (SPEC §7); --version and version
// are equivalent (--version is pre-scanned in ExecuteArgs).
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if Global.JSON {
				return output.RenderJSON(Stdout, output.OutputEnvelope{}, VersionJSON())
			}
			fmt.Fprintln(Stdout, VersionLine())
			return nil
		},
	}
}
