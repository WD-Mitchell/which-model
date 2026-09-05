package whichmodel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/aa"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/output"
	"github.com/WD-Mitchell/which-model/internal/security"
)

// CollectOptions configures the Collect stage
// (specs/features/F23-cmd-catalog/CONTRACTS.md).
type CollectOptions struct {
	Rebuild            bool // fetch model data anew and derive from fresh raw observations only
	Providers          []string
	ProviderConfigPath string
	BenchmarksPath     string
	AddAAPage          bool
	OutPath            string
	Timeout            time.Duration
	CacheTTL           time.Duration
	AAKey              string
	CatalogueCachePath string
}

// CollectResult is Collect's outcome summary.
type CollectResult struct {
	Providers  int
	Models     int
	RawCSVPath string
}

// DeriveOptions configures the Derive stage.
type DeriveOptions struct {
	InPath, OutPath, BenchmarksPath, Normalizer, Aggregator string
}

// DeriveResult is Derive's outcome summary.
type DeriveResult struct {
	Rows          int
	ScoresCSVPath string
}

// StageRunner performs one catalog pipeline stage.
type StageRunner interface {
	Collect(ctx context.Context, o CollectOptions) (CollectResult, error)
	Derive(ctx context.Context, o DeriveOptions) (DeriveResult, error)
}

// AAKeyResolver resolves the Artificial Analysis API key.
type AAKeyResolver func(repoRoot string) (string, error)

// stageReport records which stages ran and their results.
type stageReport struct {
	Collect *CollectResult
	Derive  *DeriveResult
}

// newRunner is the StageRunner test seam.
var newRunner = func() StageRunner { return &defaultRunner{} }

// runStages executes stages in fixed Collect-then-Derive order (deduplicated
// regardless of input order); Collect failure aborts before Derive.
func runStages(ctx context.Context, r StageRunner, resolveKey AAKeyResolver, repoRoot string, g *GlobalFlags, stages []Stage, co CollectOptions, do DeriveOptions) (stageReport, error) {
	var report stageReport
	want := make(map[Stage]bool, len(stages))
	for _, s := range stages {
		want[s] = true
	}

	if want[StageCollect] {
		if g.Offline {
			return report, &UsageError{Message: "Collect requires network access; incompatible with --offline"}
		}
		resolve := resolveKey
		if resolve == nil {
			resolve = aa.LoadAAAPIKey
		}
		key, err := resolve(repoRoot)
		if err != nil {
			return report, &UsageError{Message: "ARTIFICIAL_ANALYSIS_API is not set; the Collect stage requires an Artificial Analysis API key"}
		}
		co.AAKey = key
		res, err := r.Collect(ctx, co)
		if err != nil {
			return report, err
		}
		report.Collect = &res
	}

	if want[StageDerive] {
		res, err := r.Derive(ctx, do)
		if err != nil {
			return report, err
		}
		report.Derive = &res
	}

	return report, nil
}

// defaultRunner is the production StageRunner.
type defaultRunner struct{}

const maxConfigBytes = 1 << 20

// readFileOrMissing reads path bounded by maxBytes, reporting a "missing"
// sentinel distinct from other read failures.
func readFileOrMissing(path string, maxBytes int64) (data []byte, missing bool, err error) {
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return nil, true, statErr
		}
	}
	data, _, err = security.ReadBoundedFile(path, maxBytes)
	if err != nil {
		var se *security.Error
		if errors.As(err, &se) && se.Message == "The credential file was not found." {
			return nil, true, err
		}
		return nil, false, err
	}
	return data, false, nil
}

// Derive implements the Derive stage: raw CSV + benchmarks.toml + F09
// score.Derive -> atomic write with rotation backup.
func (defaultRunner) Derive(ctx context.Context, o DeriveOptions) (DeriveResult, error) {
	raw, missing, err := readFileOrMissing(o.InPath, csvstore.MaxCsvBytes)
	if missing {
		return DeriveResult{}, fmt.Errorf("raw CSV not found at %s; run 'which-model catalog benchmarks' (or '--refresh-benchmarks') to collect it", o.InPath)
	}
	if err != nil {
		return DeriveResult{}, err
	}

	bench, missing, err := readFileOrMissing(o.BenchmarksPath, maxConfigBytes)
	if missing {
		return DeriveResult{}, fmt.Errorf("benchmarks config not found at %s; provide benchmarks.toml or set catalog.benchmark_config_path", o.BenchmarksPath)
	}
	if err != nil {
		return DeriveResult{}, err
	}

	norm, err := resolveNormalizerName(o.Normalizer)
	if err != nil {
		return DeriveResult{}, err
	}
	agg, err := resolveAggregatorName(o.Aggregator)
	if err != nil {
		return DeriveResult{}, err
	}

	derived, err := score.Derive(raw, bench, norm, agg)
	if err != nil {
		return DeriveResult{}, err
	}

	if _, statErr := os.Stat(o.OutPath); statErr == nil {
		if _, err := csvstore.Backup(o.OutPath, csvstore.DefaultBackupKeep); err != nil {
			return DeriveResult{}, err
		}
	} else if err := ensureBootstrapFile(o.OutPath); err != nil {
		return DeriveResult{}, err
	}

	if err := csvstore.WriteAtomicBytes(o.OutPath, derived); err != nil {
		return DeriveResult{}, err
	}

	rows, err := countRows(derived)
	if err != nil {
		return DeriveResult{}, err
	}

	return DeriveResult{Rows: rows, ScoresCSVPath: o.OutPath}, nil
}

// countRows counts non-comment, non-empty lines minus the header line.
func countRows(derived []byte) (int, error) {
	lines := strings.Split(string(derived), "\n")
	n := 0
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		n++
	}
	n--
	if n < 0 {
		return 0, fmt.Errorf("scores CSV output has no header line")
	}
	return n, nil
}

// resolveNormalizerName wraps score.ResolveNormalizer, mapping unknown names
// to a UsageError.
func resolveNormalizerName(name string) (score.Normalizer, error) {
	n, err := score.ResolveNormalizer(name)
	if err != nil {
		return nil, &UsageError{Message: err.Error()}
	}
	return n, nil
}

// resolveAggregatorName wraps score.ResolveAggregator, mapping unknown names
// to a UsageError.
func resolveAggregatorName(name string) (score.Aggregator, error) {
	a, err := score.ResolveAggregator(name)
	if err != nil {
		return nil, &UsageError{Message: err.Error()}
	}
	return a, nil
}

// warnIfStale writes the staleness warning to stderr when the scores CSV is
// stale relative to the raw CSV; StaleCheck errors and quiet/config
// suppression are silent (never change the exit code).
func warnIfStale(scoresPath, rawPath string, quiet bool, warnOnStale bool) {
	if quiet || !warnOnStale {
		return
	}
	stale, err := csvstore.StaleCheck(scoresPath, rawPath)
	if err != nil {
		return
	}
	if stale {
		output.WriteWarning(Stderr, csvstore.StaleWarning(scoresPath, rawPath))
	}
}
