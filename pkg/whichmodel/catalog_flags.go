package whichmodel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// catalogFlags are the subcommand-local catalog flags
// (specs/features/F23-cmd-catalog/CONTRACTS.md).
type catalogFlags struct {
	Providers      []string
	ProviderConfig string
	Benchmarks     string
	In, Out        string
	Add            []string
	Reasoning      []string
	MinScore       int
	Write, Check   string
}

// Bind registers every catalog flag on cmd.
func (f *catalogFlags) Bind(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringArrayVar(&f.Providers, "provider", nil, "provider id subset (repeatable)")
	fs.StringVar(&f.ProviderConfig, "provider-config", "", "providers.toml override")
	fs.StringVar(&f.Benchmarks, "benchmarks", "", "benchmarks.toml override")
	fs.StringVar(&f.In, "in", "", "raw CSV input override")
	fs.StringVar(&f.Out, "out", "", "output path override")
	fs.StringArrayVar(&f.Add, "add", nil, "additional data source (repeatable; aa_page)")
	fs.StringArrayVar(&f.Reasoning, "reasoning", nil, "reasoning filter (repeatable)")
	fs.IntVar(&f.MinScore, "min-score", 0, "minimum intelligence_index filter")
	fs.StringVar(&f.Write, "write", "", "write the generated workflow to this file")
	fs.StringVar(&f.Check, "check", "", "check the generated workflow against this file")
}

// stageSet unions the global refresh flags with sub, returning stages in the
// canonical Collect-then-Derive order with duplicates removed.
func stageSet(g *GlobalFlags, sub []Stage) []Stage {
	want := make(map[Stage]bool)
	if g.RefreshBenchmarks || g.Refresh {
		want[StageCollect] = true
	}
	if g.RefreshScores || g.Refresh {
		want[StageDerive] = true
	}
	for _, s := range sub {
		want[s] = true
	}
	var out []Stage
	for _, s := range []Stage{StageCollect, StageDerive} {
		if want[s] {
			out = append(out, s)
		}
	}
	return out
}

// validateAdd rejects any --add value other than "aa_page".
func validateAdd(values []string) error {
	for _, v := range values {
		if v != "aa_page" {
			return &UsageError{Message: fmt.Sprintf("unknown --add value %q (supported: aa_page)", v)}
		}
	}
	return nil
}

// validateProviders rejects any --provider id not present in configured.
func validateProviders(ids []string, configured map[string][]string) error {
	for _, id := range ids {
		if _, ok := configured[id]; !ok {
			known := make([]string, 0, len(configured))
			for k := range configured {
				known = append(known, k)
			}
			sort.Strings(known)
			return &UsageError{Message: fmt.Sprintf("unknown provider %q (configured: %s)", id, strings.Join(known, ", "))}
		}
	}
	return nil
}

// validateWorkflowFlags rejects --write and --check used together.
func validateWorkflowFlags(f *catalogFlags) error {
	if f.Write != "" && f.Check != "" {
		return &UsageError{Message: "--write and --check are mutually exclusive"}
	}
	return nil
}
