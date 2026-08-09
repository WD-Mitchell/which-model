package whichmodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/decimal"
	"github.com/WD-Mitchell/which-model/internal/output"
	sdecimal "github.com/shopspring/decimal"
)

var listColumns = []string{"model", "reasoning", "intelligence_index", "cost_per_intelligence_index_task_usd"}

func newListCmd() *cobra.Command {
	f := &catalogFlags{}
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(f, cmd)
		},
	}
	f.Bind(cmd)
	return cmd
}

func runList(f *catalogFlags, cmd *cobra.Command) error {
	cfg, res, err := catalogPreamble(f)
	if err != nil {
		return err
	}

	cc, err := loadCatalogConfig(cfg)
	if err != nil {
		return err
	}

	stages := stageSet(&Global, nil)
	if len(stages) > 0 {
		ttl, err := parseCacheTTL(cc.CacheTTL)
		if err != nil {
			return err
		}
		sc := score.DefaultScoringConfig()
		if err := cfg.UnmarshalKey("scoring", &sc); err != nil {
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
		if _, err := runStages(cmd.Context(), newRunner(), nil, findRepoRoot(mustGetwd()), &Global, stages,
			CollectOptions{
				Providers:          f.Providers,
				ProviderConfigPath: providerConfigPath,
				BenchmarksPath:     benchmarksPath,
				OutPath:            res.RawCSVPath,
				Timeout:            Global.Timeout,
				CacheTTL:           ttl,
				CatalogueCachePath: res.CatalogueCachePath,
			},
			DeriveOptions{
				InPath:         res.RawCSVPath,
				OutPath:        res.ScoresCSVPath,
				BenchmarksPath: benchmarksPath,
				Normalizer:     sc.Normalizer,
				Aggregator:     sc.Aggregator,
			},
		); err != nil {
			return err
		}
	}

	rows, _, err := csvstore.Read(res.ScoresCSVPath)
	if err != nil {
		if errors.Is(err, csvstore.ErrMissingFile) {
			return fmt.Errorf("scores CSV not found at %s; run 'which-model catalog refresh' (or '--refresh-scores') to generate it", res.ScoresCSVPath)
		}
		return err
	}

	warnIfStale(res.ScoresCSVPath, res.RawCSVPath, Global.Quiet, cc.WarnOnStaleScores)

	listed := listRows(rows, f.Reasoning, f.MinScore)

	if Global.JSON {
		docs := make([]map[string]string, len(listed))
		for i, lr := range listed {
			doc := make(map[string]string, len(listColumns))
			for _, col := range listColumns {
				if col == "intelligence_index" && !lr.hasScore {
					continue
				}
				if v, ok := lr.values[col]; ok {
					doc[col] = v
				}
			}
			docs[i] = doc
		}
		out, err := json.Marshal(docs)
		if err != nil {
			return err
		}
		if _, err := cmd.OutOrStdout().Write(append(out, '\n')); err != nil {
			return err
		}
		return nil
	}

	table := make([][]string, len(listed))
	for i, lr := range listed {
		row := make([]string, len(listColumns))
		for j, col := range listColumns {
			v, ok := lr.values[col]
			if !ok || v == "" {
				row[j] = "-"
			} else {
				row[j] = v
			}
		}
		table[i] = row
	}
	return output.RenderTable(cmd.OutOrStdout(), listColumns, table)
}

type listedRow struct {
	values   map[string]string
	score    sdecimal.Decimal
	hasScore bool
}

// listRows filters and sorts rows per SPEC §10/D2/D18.
func listRows(rows []csvstore.Row, reasoningFilter []string, minScore int) []listedRow {
	wantReasoning := make(map[string]bool, len(reasoningFilter))
	for _, r := range reasoningFilter {
		wantReasoning[r] = true
	}

	var out []listedRow
	for _, row := range rows {
		values := make(map[string]string, len(row.Header))
		for i, name := range row.Header {
			if i < len(row.Values) {
				values[name] = row.Values[i]
			}
		}
		if len(wantReasoning) > 0 && !wantReasoning[values["reasoning"]] {
			continue
		}
		score, err := decimal.Parse(values["intelligence_index"])
		hasScore := err == nil
		if minScore != 0 {
			if !hasScore {
				continue
			}
			intPart := score.IntPart()
			if intPart < int64(minScore) {
				continue
			}
		}
		out = append(out, listedRow{values: values, score: score, hasScore: hasScore})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].hasScore != out[j].hasScore {
			return out[i].hasScore
		}
		if !out[i].hasScore {
			return false
		}
		cmp := out[i].score.Cmp(out[j].score)
		if cmp != 0 {
			return cmp > 0
		}
		return out[i].values["model"] < out[j].values["model"]
	})
	return out
}
