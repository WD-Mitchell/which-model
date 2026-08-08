package whichmodel

import (
	"time"

	"github.com/spf13/cobra"
)

// GlobalFlags holds every root persistent flag (annex-d §1.2 plus --text,
// F22's explicit inverse of --json). Version is pre-scanned, not bound.
type GlobalFlags struct {
	JSON              bool          // --json
	Text              bool          // --text (F22 addition; inverse of --json)
	MaxAge            time.Duration // --max-age
	Timeout           time.Duration // --timeout; default 10s (annex-d DefaultTimeoutSec)
	Quiet             bool          // --quiet
	Verbose           int           // --verbose (count)
	NoColor           bool          // --no-color
	Offline           bool          // --offline
	ConfigPath        string        // --config; feeds config.LoadOptions.Path
	RefreshUsage      bool          // --refresh-usage
	RefreshBenchmarks bool          // --refresh-benchmarks
	RefreshScores     bool          // --refresh-scores
	Refresh           bool          // --refresh (expanded by Normalize)
	NoUsage           bool          // --no-usage
	ShowIdentity      bool          // --show-identity
	Schema            bool          // --schema (pre-scan; see root.go)
	Version           bool          // --version (pre-scan)
	Normalizer        string        // --normalizer; default "minmax-linear"
	Aggregator        string        // --aggregator; default "weighted-arithmetic-mean"
}

// Global is the package-level singleton every feature reads. NewRootCmd
// re-binds it, resetting it to defaults on every ExecuteArgs call.
var Global GlobalFlags

// Bind registers every persistent flag on cmd; defaults are written into g.
func (g *GlobalFlags) Bind(cmd *cobra.Command) error {
	fs := cmd.PersistentFlags()
	fs.BoolVar(&g.JSON, "json", false, "emit JSON output")
	fs.BoolVar(&g.Text, "text", false, "emit text output (inverse of --json)")
	fs.DurationVar(&g.MaxAge, "max-age", 0, "maximum snapshot age before a refresh is forced")
	fs.DurationVar(&g.Timeout, "timeout", 10*time.Second, "per-request timeout")
	fs.BoolVar(&g.Quiet, "quiet", false, "suppress non-essential output")
	fs.CountVar(&g.Verbose, "verbose", "increase verbosity (repeatable)")
	fs.BoolVar(&g.NoColor, "no-color", false, "disable colored output")
	fs.BoolVar(&g.Offline, "offline", false, "never touch the network")
	fs.StringVar(&g.ConfigPath, "config", "", "explicit config file path")
	fs.BoolVar(&g.RefreshUsage, "refresh-usage", false, "force a usage refresh")
	fs.BoolVar(&g.RefreshBenchmarks, "refresh-benchmarks", false, "force a benchmarks refresh")
	fs.BoolVar(&g.RefreshScores, "refresh-scores", false, "force a scores refresh")
	fs.BoolVar(&g.Refresh, "refresh", false, "force every refresh")
	fs.BoolVar(&g.NoUsage, "no-usage", false, "disable usage for this run")
	fs.BoolVar(&g.ShowIdentity, "show-identity", false, "show identity fields in output")
	fs.BoolVar(&g.Schema, "schema", false, "print the resolved command's JSON schema")
	fs.StringVar(&g.Normalizer, "normalizer", "minmax-linear", "scoring normalizer name")
	fs.StringVar(&g.Aggregator, "aggregator", "weighted-arithmetic-mean", "scoring aggregator name")
	return nil
}

// Normalize expands --refresh into the three targeted refresh flags
// (annex-d §1.6 rule 5).
func (g *GlobalFlags) Normalize() error {
	if g.Refresh {
		g.RefreshUsage = true
		g.RefreshBenchmarks = true
		g.RefreshScores = true
	}
	return nil
}

// Validate rejects contradictory flag sets (annex-d §1.6 rule 4 plus --text).
// --offline with --refresh-scores alone is allowed (Derive is offline-safe).
func (g *GlobalFlags) Validate() error {
	if g.JSON && g.Text {
		return &UsageError{Message: "--json and --text are mutually exclusive"}
	}
	if g.Offline && g.Refresh {
		return &UsageError{Message: "--offline and --refresh are mutually exclusive"}
	}
	if g.Offline && g.RefreshBenchmarks {
		return &UsageError{Message: "--offline and --refresh-benchmarks are mutually exclusive"}
	}
	return nil
}
