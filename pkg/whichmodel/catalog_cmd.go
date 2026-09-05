package whichmodel

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/catalog/publish"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/output"
)

func init() { register(NewCatalogCmd) }

// NewCatalogCmd is the catalog command group: refresh, benchmarks, scores,
// list, providers, workflow (subcommands added by later F23 tasks).
func NewCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "catalog refresh and views",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newRefreshCmd())
	cmd.AddCommand(newBenchmarksCmd())
	cmd.AddCommand(newScoresCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newProvidersCmd())
	cmd.AddCommand(newWorkflowCmd())
	return cmd
}

// catalogPreamble resolves config, catalog paths, and validates the shared
// flags used by every real catalog subcommand.
func catalogPreamble(f *catalogFlags) (*config.Config, ResolvedCatalog, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, ResolvedCatalog{}, err
	}
	cc, err := loadCatalogConfig(cfg)
	if err != nil {
		return nil, ResolvedCatalog{}, err
	}
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	paths := config.ResolvePaths(runtime.GOOS, home, os.Getenv)
	res := resolveCatalogPaths(cc, paths, cwd)
	if err := validateAdd(f.Add); err != nil {
		return nil, ResolvedCatalog{}, err
	}
	if len(f.Providers) > 0 {
		providerConfigPath := f.ProviderConfig
		if providerConfigPath == "" {
			providerConfigPath = res.ProviderConfigPath
		}
		providers, err := loadProviderConfig(providerConfigPath)
		if err != nil {
			return nil, ResolvedCatalog{}, err
		}
		if err := validateProviders(f.Providers, providers); err != nil {
			return nil, ResolvedCatalog{}, err
		}
	}
	return cfg, res, nil
}

func newRefreshCmd() *cobra.Command {
	f := &catalogFlags{}
	cmd := &cobra.Command{
		Use:  "refresh",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefresh(f, cmd)
		},
	}
	f.Bind(cmd)
	cmd.Flags().BoolVar(&f.Rebuild, "rebuild", false, "rebuild from fresh source data without cached model lists or old raw values")
	return cmd
}

func runRefresh(f *catalogFlags, cmd *cobra.Command) error {
	if f.Rebuild && len(f.Providers) > 0 {
		return &UsageError{Message: "--rebuild cannot be combined with a --provider subset"}
	}
	cfg, res, err := catalogPreamble(f)
	if err != nil {
		return err
	}

	providerConfigPath := f.ProviderConfig
	if providerConfigPath == "" {
		providerConfigPath = res.ProviderConfigPath
	}
	benchmarksPath := f.Benchmarks
	if benchmarksPath == "" {
		benchmarksPath = res.BenchmarkConfigPath
	}
	outPath := f.Out
	if outPath == "" {
		outPath = res.ScoresCSVPath
	}

	stages := stageSet(&Global, []Stage{StageCollect, StageDerive})

	cc, err := loadCatalogConfig(cfg)
	if err != nil {
		return err
	}
	ttl, err := parseCacheTTL(cc.CacheTTL)
	if err != nil {
		return err
	}

	addAA := false
	for _, v := range f.Add {
		if v == "aa_page" {
			addAA = true
		}
	}

	sc := score.DefaultScoringConfig()
	if err := cfg.UnmarshalKey("scoring", &sc); err != nil {
		return err
	}

	report, err := runStages(cmd.Context(), newRunner(), nil, findRepoRoot(mustGetwd()), &Global, stages,
		CollectOptions{
			Rebuild:            f.Rebuild,
			Providers:          f.Providers,
			ProviderConfigPath: providerConfigPath,
			BenchmarksPath:     benchmarksPath,
			AddAAPage:          addAA,
			OutPath:            res.RawCSVPath,
			Timeout:            Global.Timeout,
			CacheTTL:           ttl,
			CatalogueCachePath: res.CatalogueCachePath,
		},
		DeriveOptions{
			InPath:         res.RawCSVPath,
			OutPath:        outPath,
			BenchmarksPath: benchmarksPath,
			Normalizer:     sc.Normalizer,
			Aggregator:     sc.Aggregator,
		},
	)
	if err != nil {
		return err
	}

	return renderStageReport(cmd, report)
}

func mustGetwd() string {
	cwd, _ := os.Getwd()
	return cwd
}

// renderStageReport writes the text or JSON output for whichever stages ran.
func renderStageReport(cmd *cobra.Command, report stageReport) error {
	if Global.JSON {
		payload := map[string]any{}
		if report.Collect != nil {
			payload["collect"] = map[string]any{
				"providers":    report.Collect.Providers,
				"models":       report.Collect.Models,
				"raw_csv_path": report.Collect.RawCSVPath,
			}
		}
		if report.Derive != nil {
			payload["derive"] = map[string]any{
				"rows":            report.Derive.Rows,
				"scores_csv_path": report.Derive.ScoresCSVPath,
			}
		}
		return output.RenderJSON(cmd.OutOrStdout(), output.OutputEnvelope{}, payload)
	}
	if report.Collect != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "collected %d providers, %d models -> %s\n", report.Collect.Providers, report.Collect.Models, report.Collect.RawCSVPath)
	}
	if report.Derive != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "derived %d rows -> %s\n", report.Derive.Rows, report.Derive.ScoresCSVPath)
	}
	return nil
}

func newBenchmarksCmd() *cobra.Command {
	f := &catalogFlags{}
	cmd := &cobra.Command{
		Use:  "benchmarks",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarks(f, cmd)
		},
	}
	f.Bind(cmd)
	return cmd
}

func runBenchmarks(f *catalogFlags, cmd *cobra.Command) error {
	cfg, res, err := catalogPreamble(f)
	if err != nil {
		return err
	}

	providerConfigPath := f.ProviderConfig
	if providerConfigPath == "" {
		providerConfigPath = res.ProviderConfigPath
	}
	benchmarksPath := f.Benchmarks
	if benchmarksPath == "" {
		benchmarksPath = res.BenchmarkConfigPath
	}

	stages := stageSet(&Global, []Stage{StageCollect})

	cc, err := loadCatalogConfig(cfg)
	if err != nil {
		return err
	}
	ttl, err := parseCacheTTL(cc.CacheTTL)
	if err != nil {
		return err
	}

	addAA := false
	for _, v := range f.Add {
		if v == "aa_page" {
			addAA = true
		}
	}

	report, err := runStages(cmd.Context(), newRunner(), nil, findRepoRoot(mustGetwd()), &Global, stages,
		CollectOptions{
			Providers:          f.Providers,
			ProviderConfigPath: providerConfigPath,
			BenchmarksPath:     benchmarksPath,
			AddAAPage:          addAA,
			OutPath:            res.RawCSVPath,
			Timeout:            Global.Timeout,
			CacheTTL:           ttl,
			CatalogueCachePath: res.CatalogueCachePath,
		},
		DeriveOptions{},
	)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(res.ScoresCSVPath); statErr == nil {
		warnIfStale(res.ScoresCSVPath, res.RawCSVPath, Global.Quiet, cc.WarnOnStaleScores)
	}

	return renderStageReport(cmd, report)
}

func newScoresCmd() *cobra.Command {
	f := &catalogFlags{}
	cmd := &cobra.Command{
		Use:  "scores",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScores(f, cmd)
		},
	}
	f.Bind(cmd)
	return cmd
}

func runScores(f *catalogFlags, cmd *cobra.Command) error {
	cfg, res, err := catalogPreamble(f)
	if err != nil {
		return err
	}

	inPath := f.In
	if inPath == "" {
		inPath = res.RawCSVPath
	}
	outPath := f.Out
	if outPath == "" {
		outPath = res.ScoresCSVPath
	}
	benchmarksPath := f.Benchmarks
	if benchmarksPath == "" {
		benchmarksPath = res.BenchmarkConfigPath
	}

	stages := stageSet(&Global, []Stage{StageDerive})

	sc := score.DefaultScoringConfig()
	if err := cfg.UnmarshalKey("scoring", &sc); err != nil {
		return err
	}

	report, err := runStages(cmd.Context(), newRunner(), nil, findRepoRoot(mustGetwd()), &Global, stages,
		CollectOptions{},
		DeriveOptions{
			InPath:         inPath,
			OutPath:        outPath,
			BenchmarksPath: benchmarksPath,
			Normalizer:     sc.Normalizer,
			Aggregator:     sc.Aggregator,
		},
	)
	if err != nil {
		return err
	}

	return renderStageReport(cmd, report)
}

// workflowDriftErr is the --check drift sentinel: the diff has already been
// printed to stderr, and this plain error makes the root map the failure to
// exit 1 without inventing a new failure code.
var workflowDriftErr = errors.New("catalog workflow: --check found drift (see diff above)")

func newWorkflowCmd() *cobra.Command {
	var write, check bool
	var out string
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Generate or check the refresh-model-data GitHub Action",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if write && check {
				return &UsageError{Message: "catalog workflow: --write and --check are mutually exclusive"}
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			pc, err := publish.Load(cfg)
			if err != nil {
				return err // exit 2 (config validation)
			}
			path := out
			if path == "" {
				root, err := publish.RepoRoot()
				if err != nil {
					return err // exit 1
				}
				path = publish.WorkflowPath(root)
			}
			if check {
				if err := publish.Check(pc, path); err != nil {
					var de *publish.DriftError
					if errors.As(err, &de) {
						fmt.Fprintln(cmd.ErrOrStderr(), de.Error())
						return workflowDriftErr // mapped to exit 1
					}
					return err
				}
				return nil
			}
			summary, err := publish.Write(pc, path)
			if err != nil {
				return err
			}
			if out == "" && pc.Enabled {
				summary = fmt.Sprintf("wrote .github/workflows/%s (schedule=%q, branches=%v, mode=%s)", publish.DefaultWorkflowName, pc.Schedule, pc.Branches, pc.Mode)
			}
			fmt.Fprintln(cmd.OutOrStdout(), summary)
			return nil
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "generate/overwrite the workflow file")
	cmd.Flags().BoolVar(&check, "check", false, "render in-memory and diff against the committed file; exit 1 on drift")
	cmd.Flags().StringVar(&out, "out", "", "output path (default <repoRoot>/.github/workflows/refresh-model-data.yml)")
	return cmd
}
