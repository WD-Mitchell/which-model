package whichmodel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/aa"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
	sdecimal "github.com/shopspring/decimal"
)


// ensureBootstrapFile creates path with minimal non-empty placeholder
// content when it does not yet exist (F06's WriteAtomic/WriteAtomicBytes are
// "replace" primitives that CAS-verify against a pre-existing, non-empty
// original — a repo's very first catalog refresh has no such file). Never
// touches an existing file.
func ensureBootstrapFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("# bootstrap\n"), 0o644)
}
// modelsDevProvidersURL / modelsDevBenchmarksURL / aaV2Fetch / aaPageFetch /
// aaClientForTest are test seams: production defaults to the pinned F08
// constants/wrappers; tests redirect them to httptest servers so Collect's
// full orchestration (cache, merge, backup, atomic write) is exercised
// without touching the real network.
var (
	modelsDevProvidersURL  = modelsdev.ProvidersURL
	modelsDevBenchmarksURL = modelsdev.BenchmarksURL
	aaV2Fetch              = aa.FetchAAv2
	aaPageFetch            = aa.FetchAAPage
	aaClientForTest        = aa.AAV2Client
)

// benchmarkEvidenceIndex is canonicalID -> benchmark name -> effort -> score.
type benchmarkEvidenceIndex map[string]map[string]map[string]sdecimal.Decimal

func buildEvidenceIndex(benchmarks []modelsdev.BenchmarkRecord) benchmarkEvidenceIndex {
	idx := make(benchmarkEvidenceIndex)
	for _, rec := range benchmarks {
		if rec.CanonicalID == "" {
			continue
		}
		byName := idx[rec.CanonicalID]
		if byName == nil {
			byName = make(map[string]map[string]sdecimal.Decimal)
			idx[rec.CanonicalID] = byName
		}
		for _, ev := range rec.Benchmarks {
			byEffort := byName[ev.Name]
			if byEffort == nil {
				byEffort = make(map[string]sdecimal.Decimal)
				byName[ev.Name] = byEffort
			}
			byEffort[ev.Effort] = ev.Score
		}
	}
	return idx
}

// lookupEvidence resolves one (name, level) cell: an exact-level entry wins,
// else the effort-agnostic "" entry, else blank.
func (idx benchmarkEvidenceIndex) lookupEvidence(canonicalKeys []string, name, level string) string {
	for _, key := range canonicalKeys {
		byName, ok := idx[key]
		if !ok {
			continue
		}
		byEffort, ok := byName[name]
		if !ok {
			continue
		}
		if v, ok := byEffort[level]; ok {
			return v.String()
		}
		if v, ok := byEffort[""]; ok {
			return v.String()
		}
		return ""
	}
	return ""
}

func decimalCell(d *sdecimal.Decimal) string {
	if d == nil {
		return ""
	}
	return d.String()
}

// buildFreshRows merges the models.dev catalogue, AA v2 (+ optional page)
// data, and models.dev benchmark evidence into fresh raw rows per (model,
// effort level) (specs/features/F23-cmd-catalog/CONTRACTS.md).
func buildFreshRows(catalogue []modelsdev.ProviderModel, benchmarks []modelsdev.BenchmarkRecord, aaModels []aa.AAModel, pages map[string]aa.PageMetrics, expandedNames []string) ([]csvstore.Row, error) {
	aaByBase := make(map[string]aa.AAModel, len(aaModels))
	for _, m := range aaModels {
		aaByBase[path.Base(m.Slug)] = m
	}
	evidence := buildEvidenceIndex(benchmarks)
	expandedSet := make(map[string]bool, len(expandedNames))
	for _, n := range expandedNames {
		expandedSet[n] = true
	}

	colSet := make(map[string]bool)
	for _, m := range aaModels {
		for col := range m.Benchmarks {
			colSet[col] = true
		}
	}
	for _, rec := range benchmarks {
		for _, ev := range rec.Benchmarks {
			if expandedSet[ev.Name] {
				colSet[csvstore.BenchmarkColumnPrefix+ev.Name] = true
			}
		}
	}
	dynamic := make([]string, 0, len(colSet))
	for c := range colSet {
		dynamic = append(dynamic, c)
	}
	sort.Strings(dynamic)

	header := make([]string, 0, len(csvstore.RawCoreColumns)+len(dynamic))
	header = append(header, csvstore.RawCoreColumns...)
	header = append(header, dynamic...)

	var rows []csvstore.Row
	for _, cm := range catalogue {
		if cm.Status == "deprecated" {
			continue
		}
		levels := cm.EffortLevels
		if len(levels) == 0 {
			levels = []string{"high"}
		}
		aaModel, hasAA := aaByBase[cm.ModelID]
		var page *aa.PageMetrics
		if hasAA {
			if p, ok := pages[aaModel.Slug]; ok {
				page = &p
			}
		}
		canonicalKeys := []string{cm.Provider + "/" + cm.ModelID, cm.ModelID}
		modelName := cm.Name
		if modelName == "" {
			modelName = cm.ModelID
		}

		for _, level := range levels {
			values := make([]string, len(header))
			values[0] = modelName
			values[1] = level

			if hasAA {
				values[2] = decimalCell(aaModel.IntelligenceIndex)
			}
			timeVal := ""
			if page != nil {
				timeVal = decimalCell(page.TimePerIntelligenceTaskSeconds)
			}
			values[3] = timeVal
			costVal := ""
			if hasAA {
				costVal = decimalCell(aaModel.CostPerTaskUSD)
			}
			if costVal == "" && page != nil {
				costVal = decimalCell(page.FallbackCostUSD)
			}
			values[4] = costVal
			if hasAA {
				values[5] = decimalCell(aaModel.MedianResponseSeconds)
				values[6] = decimalCell(aaModel.CodingIndex)
				values[7] = decimalCell(aaModel.AgenticIndex)
			}

			for i, col := range dynamic {
				idx := len(csvstore.RawCoreColumns) + i
				if hasAA {
					if v, ok := aaModel.Benchmarks[col]; ok {
						values[idx] = v.String()
						continue
					}
				}
				name := strings.TrimPrefix(col, csvstore.BenchmarkColumnPrefix)
				values[idx] = evidence.lookupEvidence(canonicalKeys, name, level)
			}

			rows = append(rows, csvstore.Row{Header: header, Values: values})
		}
	}

	if len(rows) > 0 {
		if err := csvstore.ValidateRawRows(rows); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// mergeWithExisting applies F06's merge semantics: full refresh -> MergeRows;
// subset refresh -> MergePartialRefresh.
func mergeWithExisting(existing []csvstore.Row, fresh []csvstore.Row, refreshedModelIDs []string) ([]csvstore.Row, error) {
	if len(refreshedModelIDs) == 0 {
		return csvstore.MergeRows(existing, fresh)
	}
	return csvstore.MergePartialRefresh(existing, fresh, refreshedModelIDs, true)
}

// readCache reads the cached models.dev provider catalogue; a missing file
// is reported via ok=false, not an error.
func readCache(cachePath string) ([]modelsdev.ProviderModel, bool, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var catalogue []modelsdev.ProviderModel
	if err := json.Unmarshal(data, &catalogue); err != nil {
		return nil, false, err
	}
	return catalogue, true, nil
}

// writeCache atomically writes catalogue as JSON to cachePath.
func writeCache(cachePath string, catalogue []modelsdev.ProviderModel) error {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(catalogue)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".modelsdev-cache-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// cacheFresh reports whether cachePath's mtime is within ttl of now.
func cacheFresh(cachePath string, ttl time.Duration) bool {
	info, err := os.Stat(cachePath)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < ttl
}

// Collect implements the Collect stage: models.dev catalogue (cached) +
// benchmarks + AA v2 (+ optional page) -> merged raw CSV.
func (defaultRunner) Collect(ctx context.Context, o CollectOptions) (CollectResult, error) {
	providers, err := loadProviderConfig(o.ProviderConfigPath)
	if err != nil {
		return CollectResult{}, err
	}
	names, err := loadBenchmarkConfig(o.BenchmarksPath)
	if err != nil {
		return CollectResult{}, err
	}

	client := httpkit.NewClient(httpkit.WithTimeout(o.Timeout))

	var catalogue []modelsdev.ProviderModel
	if cacheFresh(o.CatalogueCachePath, o.CacheTTL) {
		if cached, ok, err := readCache(o.CatalogueCachePath); err == nil && ok {
			catalogue = cached
		}
	}
	if catalogue == nil {
		catalogue, err = modelsdev.FetchModelsDevProvidersFrom(client, modelsDevProvidersURL)
		if err != nil {
			return CollectResult{}, err
		}
		if err := writeCache(o.CatalogueCachePath, catalogue); err != nil {
			return CollectResult{}, err
		}
	}

	benchRecords, err := modelsdev.FetchModelsDevBenchmarksFrom(client, modelsDevBenchmarksURL, names)
	if err != nil {
		return CollectResult{}, err
	}

	aaModels, err := aaV2Fetch(aaClientForTest(), o.AAKey)
	if err != nil {
		return CollectResult{}, err
	}

	pages := make(map[string]aa.PageMetrics)
	if o.AddAAPage {
		for _, m := range aaModels {
			p, err := aaPageFetch(aaClientForTest(), m.Slug, false)
			if err != nil || p == nil {
				continue
			}
			pages[m.Slug] = *p
		}
	}


	fresh, err := buildFreshRows(catalogue, benchRecords, aaModels, pages, names)
	if err != nil {
		return CollectResult{}, err
	}

	var existing []csvstore.Row
	if _, statErr := os.Stat(o.OutPath); statErr == nil {
		existing, _, err = csvstore.Read(o.OutPath)
		if err != nil {
			return CollectResult{}, err
		}
	}

	var refreshedModelIDs []string
	if len(o.Providers) > 0 {
		want := make(map[string]bool, len(o.Providers))
		for _, p := range o.Providers {
			want[p] = true
		}
		for _, cm := range catalogue {
			if want[cm.Provider] {
				refreshedModelIDs = append(refreshedModelIDs, cm.ModelID)
			}
		}
	}

	merged, err := mergeWithExisting(existing, fresh, refreshedModelIDs)
	if err != nil {
		return CollectResult{}, err
	}

	if _, statErr := os.Stat(o.OutPath); statErr == nil {
		if _, err := csvstore.Backup(o.OutPath, csvstore.DefaultBackupKeep); err != nil {
			return CollectResult{}, err
		}
	} else if err := ensureBootstrapFile(o.OutPath); err != nil {
		return CollectResult{}, err
	}
	if err := csvstore.WriteAtomic(o.OutPath, merged, nil); err != nil {
		return CollectResult{}, err
	}

	providerCount := len(o.Providers)
	if providerCount == 0 {
		providerCount = len(providers)
	}
	return CollectResult{Providers: providerCount, Models: len(merged), RawCSVPath: o.OutPath}, nil
}
