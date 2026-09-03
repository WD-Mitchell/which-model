package service

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdecimal "github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/config"
)

func catCtx() context.Context { return context.Background() }

// rawValuesCSV is the raw-values CSV written for re-derive tests. It carries
// three eligible rows and >=2 distinct raw values per benchmark column so
// score.Derive emits benchmark scores (a single-valued column derives to a
// blank score, which would leave the custom-group overlay without evidence).
const rawValuesCSV = `model,reasoning,intelligence_index,time_per_intelligence_index_task_seconds,cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,artificial_analysis_coding_index,artificial_analysis_agentic_index,benchmark:SWE-Bench Verified,benchmark:Terminal-Bench,benchmark:SWE-Bench Pro
Claude Opus 5,max,63.1,465,2.34,61,78.0,59.2,96.0,,79.2
GPT-5.6 Sol,medium,55.6,81,0.37,15,76.3,47.9,80.0,88.8,64.6
Kimi K2.7 Code,high,43.0,100,0.22,67,60.8,30.3,70.0,76.0,
`

// writeRawValues writes the re-derive raw-values CSV into the fixture cache
// dir (the pipeline reads <CacheDir>/catalog/available_model_raw_values.csv).
func writeRawValues(t *testing.T, svc *Services) {
	t.Helper()
	dir := filepath.Join(svc.paths.CacheDir, "catalog")
	if err := os.WriteFile(filepath.Join(dir, "available_model_raw_values.csv"), []byte(rawValuesCSV), 0o600); err != nil {
		t.Fatalf("write raw values: %v", err)
	}
}

// diskCfg reloads config.toml from disk, mirroring how the fixture loads it.
func diskCfg(t *testing.T, svc *Services) *config.Config {
	t.Helper()
	cfg, err := config.Load(config.LoadOptions{Path: svc.paths.UserConfigFile, Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatalf("config.Load from disk: %v", err)
	}
	return cfg
}

// countEvents returns how many recorded events carry the given event name.
func countEvents(rec *emitRecorder, name string) int {
	n := 0
	for _, e := range rec.Events() {
		if e.Event == name {
			n++
		}
	}
	return n
}

func TestCatalogBenchmarks(t *testing.T) {
	svc, _ := newTestServices(t)
	got, err := svc.Catalog().Benchmarks(catCtx())
	if err != nil {
		t.Fatalf("Benchmarks: %v", err)
	}
	want := []string{
		"AutomationBench",
		"DeepSWE",
		"Finance Agent",
		"FinanceAgent",
		"FrontierCode",
		"GDPval",
		"GDPval-AA",
		"MCP Atlas",
		"Program Bench",
		"SWE-Bench Multilingual",
		"SWE-Bench Multimodal",
		"SWE-Bench Pro",
		"SWE-Bench Verified",
		"Terminal-Bench",
		"Toolathlon",
		"τ3 Banking",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Benchmarks = %v, want %v", got, want)
	}
}

func TestCatalogCoverageGolden(t *testing.T) {
	svc, _ := newTestServices(t)
	// CoverageTotal = every (model, reasoning) row in the cached scores CSV.
	cov := map[string]int{
		"SWE-Bench Verified":     1,
		"SWE-Bench Pro":          2,
		"SWE-Bench Multilingual": 1,
		"SWE-Bench Multimodal":   1,
		"DeepSWE":                2,
		"Terminal-Bench":         1,
		"AutomationBench":        1,
		"FrontierCode":           0,
		"Program Bench":          0,
		"MCP Atlas":              1,
		"Toolathlon":             1,
		"Finance Agent":          1,
		"FinanceAgent":           0,
		"τ3 Banking":             0,
		"GDPval":                 0,
		"GDPval-AA":              0,
	}
	for _, slug := range []string{"software_engineering", "finance"} {
		detail, err := svc.Catalog().GroupDetail(catCtx(), slug)
		if err != nil {
			t.Fatalf("GroupDetail(%q): %v", slug, err)
		}
		for _, b := range detail.Benchmarks {
			if b.CoverageTotal != len(svc.scores) {
				t.Errorf("%s: CoverageTotal = %d, want %d", b.Name, b.CoverageTotal, len(svc.scores))
			}
			if want, ok := cov[b.Name]; ok && b.Covered != want {
				t.Errorf("%s: Covered = %d, want %d", b.Name, b.Covered, want)
			}
		}
	}
}

func TestCatalogBenchmarkDetail(t *testing.T) {
	svc, _ := newTestServices(t)
	c := svc.Catalog()

	// Unknown name -> not_found.
	if _, err := c.BenchmarkDetail(catCtx(), "No Such Benchmark"); !errors.Is(err, errNotFound) {
		t.Fatalf("BenchmarkDetail unknown: err = %v, want errNotFound", err)
	}

	// Single-raw benchmark: one tested row, max => Norm 100.
	d, err := c.BenchmarkDetail(catCtx(), "SWE-Bench Verified")
	if err != nil {
		t.Fatalf("BenchmarkDetail: %v", err)
	}
	if d.Name != "SWE-Bench Verified" {
		t.Errorf("Name = %q", d.Name)
	}
	const note = "Carried in the model data export. No description recorded for this benchmark yet."
	if d.Note != note {
		t.Errorf("Note = %q, want %q", d.Note, note)
	}
	if !reflect.DeepEqual(d.Groups, []string{"software_engineering"}) {
		t.Errorf("Groups = %v, want [software_engineering]", d.Groups)
	}
	wantRows := []BenchRow{{Model: "Claude Opus 5", Reasoning: "max", Value: 96.0, Norm: 100}}
	if !reflect.DeepEqual(d.Rows, wantRows) {
		t.Errorf("Rows = %+v, want %+v", d.Rows, wantRows)
	}

	// Two-raw benchmark: Norm = value/max*100 (half-up), sorted Norm desc.
	d, err = c.BenchmarkDetail(catCtx(), "SWE-Bench Pro")
	if err != nil {
		t.Fatalf("BenchmarkDetail: %v", err)
	}
	// GPT-5.6 Sol 64.6 / 79.2 * 100 = 81.57 -> 82 (RoundHalfUp, 0dp).
	wantRows = []BenchRow{
		{Model: "Claude Opus 5", Reasoning: "max", Value: 79.2, Norm: 100},
		{Model: "GPT-5.6 Sol", Reasoning: "medium", Value: 64.6, Norm: 82},
	}
	if !reflect.DeepEqual(d.Rows, wantRows) {
		t.Errorf("Rows = %+v, want %+v", d.Rows, wantRows)
	}
}

func TestCatalogModelDetail(t *testing.T) {
	svc, _ := newTestServices(t)
	c := svc.Catalog()

	empty, err := c.ModelDetail(catCtx(), "No Such Model", "max")
	if err != nil {
		t.Fatalf("unknown model: %v", err)
	}
	if len(empty.Rows) != 0 {
		t.Fatalf("unknown model rows = %+v, want empty", empty.Rows)
	}

	d, err := c.ModelDetail(catCtx(), "Claude Opus 5", "max")
	if err != nil {
		t.Fatalf("ModelDetail: %v", err)
	}
	if d.Model != "Claude Opus 5" || d.Reasoning != "max" {
		t.Fatalf("identity = %s/%s", d.Model, d.Reasoning)
	}
	if len(d.Rows) == 0 {
		t.Fatal("ModelDetail rows empty, want SWE-Bench scores")
	}
	byName := map[string]ModelBenchRow{}
	for _, row := range d.Rows {
		byName[row.Name] = row
	}
	verified, ok := byName["SWE-Bench Verified"]
	if !ok {
		t.Fatalf("missing SWE-Bench Verified in %+v", d.Rows)
	}
	if verified.Value != 96 || verified.Norm != 100 {
		t.Errorf("SWE-Bench Verified = %+v, want value 96 norm 100", verified)
	}
	if !reflect.DeepEqual(verified.Groups, []string{"software_engineering"}) {
		t.Errorf("groups = %v, want [software_engineering]", verified.Groups)
	}
}

func TestCatalogModels(t *testing.T) {
	svc, _ := newTestServices(t)
	got, err := svc.Catalog().Models(catCtx())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(Models) = %d, want 3 (%v)", len(got), got)
	}
	if got[0].ModelName != "Claude Opus 5" || got[1].ModelName != "GPT-5.6 Sol" || got[2].ModelName != "Kimi K2.7 Code" {
		t.Fatalf("order = %s, %s, %s", got[0].ModelName, got[1].ModelName, got[2].ModelName)
	}
	opus := got[0]
	if opus.ModelID != "claude-opus-5" {
		t.Errorf("Opus ModelID = %q, want claude-opus-5", opus.ModelID)
	}
	if !reflect.DeepEqual(opus.Reasoning, []string{"high", "max"}) {
		t.Errorf("Opus reasoning = %v, want [high max]", opus.Reasoning)
	}
	if opus.Intelligence == nil || *opus.Intelligence != 100 {
		t.Errorf("Opus intelligence = %v, want 100", opus.Intelligence)
	}
	if opus.Cost == nil || *opus.Cost != 0 {
		t.Errorf("Opus cost = %v, want 0", opus.Cost)
	}
	if opus.Speed == nil || *opus.Speed != 12 {
		t.Errorf("Opus speed = %v, want 12", opus.Speed)
	}
	if opus.ProviderCount != 1 {
		t.Errorf("Opus ProviderCount = %d, want 1", opus.ProviderCount)
	}
	sol := got[1]
	if sol.ModelID != "gpt-5.6" {
		t.Errorf("Sol ModelID = %q, want gpt-5.6", sol.ModelID)
	}
	if sol.Intelligence == nil || *sol.Intelligence != 63 {
		t.Errorf("Sol intelligence = %v, want 63", sol.Intelligence)
	}
	if sol.ProviderCount != 1 {
		t.Errorf("Sol ProviderCount = %d, want 1", sol.ProviderCount)
	}
	kimi := got[2]
	if kimi.ModelID != "" {
		t.Errorf("Kimi ModelID = %q, want empty", kimi.ModelID)
	}
	if kimi.ProviderCount != 0 {
		t.Errorf("Kimi ProviderCount = %d, want 0", kimi.ProviderCount)
	}
	if kimi.Intelligence == nil || *kimi.Intelligence != 0 {
		t.Errorf("Kimi intelligence = %v, want 0", kimi.Intelligence)
	}
}
func TestCatalogModels_OnlyEnabledProvidersFilter(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[gui]
only_enabled_providers = true
`))
	got, err := svc.Catalog().Models(catCtx())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(Models) with only_enabled_providers=true = %d, want 2 (%v)", len(got), got)
	}
	if got[0].ModelName != "Claude Opus 5" || got[1].ModelName != "GPT-5.6 Sol" {
		t.Fatalf("models = %s, %s, want Opus and Sol (excluding Kimi)", got[0].ModelName, got[1].ModelName)
	}
}


func TestCatalogModelsTopReasoningScores(t *testing.T) {
	header := "model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score"
	csv := header + "\n" +
		"Claude Opus 5,high,40,10,20,30,0,0\n" +
		"Claude Opus 5,max,90,10,20,30,0,0\n"
	svc, _ := newTestServices(t, WithScoresCSV(csv))
	got, err := svc.Catalog().Models(catCtx())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0].Reasoning, []string{"high", "max"}) {
		t.Errorf("reasoning = %v, want [high max]", got[0].Reasoning)
	}
	if got[0].Intelligence == nil || *got[0].Intelligence != 90 {
		t.Errorf("intelligence = %v, want 90 (max row)", got[0].Intelligence)
	}
}

func TestCatalogModelCard(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[providers.claude]
enabled = true
`))
	in, out := 15.0, 75.0
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:          "anthropic",
		ModelID:           "claude-opus-5",
		Name:              "Claude Opus 5",
		InputCostUSDPerM:  &in,
		OutputCostUSDPerM: &out,
	}})

	if _, err := svc.Catalog().Model(catCtx(), "No Such Model"); !errors.Is(err, errNotFound) {
		t.Fatalf("unknown: %v, want errNotFound", err)
	}

	got, err := svc.Catalog().Model(catCtx(), "Claude Opus 5")
	if err != nil {
		t.Fatalf("Model: %v", err)
	}
	if got.ModelName != "Claude Opus 5" || got.ModelID != "claude-opus-5" {
		t.Fatalf("identity = %s/%s", got.ModelName, got.ModelID)
	}
	if !got.InCatalog {
		t.Errorf("InCatalog = false, want true for scored model")
	}
	if got.Intelligence == nil || *got.Intelligence != 100 {
		t.Errorf("intelligence = %v, want 100", got.Intelligence)
	}
	if len(got.Providers) != 1 {
		t.Fatalf("providers = %+v, want 1 (claude enabled)", got.Providers)
	}
	row := got.Providers[0]
	if row.Provider != "claude" || row.ModelID != "claude-opus-5" {
		t.Errorf("provider row = %+v", row)
	}
	if !reflect.DeepEqual(row.Reasoning, []string{"high", "max"}) {
		t.Errorf("reasoning = %v, want [high max]", row.Reasoning)
	}
	if !reflect.DeepEqual(row.RouteKeys, []string{"claude/claude-opus-5@high", "claude/claude-opus-5@max"}) {
		t.Errorf("route keys = %v", row.RouteKeys)
	}
	if row.InputCostUSDPerM == nil || *row.InputCostUSDPerM != 15 {
		t.Errorf("input cost = %v, want 15", row.InputCostUSDPerM)
	}
	if row.OutputCostUSDPerM == nil || *row.OutputCostUSDPerM != 75 {
		t.Errorf("output cost = %v, want 75", row.OutputCostUSDPerM)
	}

	sol, err := svc.Catalog().Model(catCtx(), "GPT-5.6 Sol")
	if err != nil {
		t.Fatalf("Sol: %v", err)
	}
	if len(sol.Providers) != 0 {
		t.Errorf("Sol providers = %+v, want none (codex not enabled)", sol.Providers)
	}
}

func TestCatalogModelCardNoListedPrice(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[providers.claude]
enabled = true
`))
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider: "anthropic",
		ModelID:  "claude-opus-5",
		Name:     "Claude Opus 5",
	}})
	got, err := svc.Catalog().Model(catCtx(), "Claude Opus 5")
	if err != nil {
		t.Fatalf("Model: %v", err)
	}
	if len(got.Providers) != 1 {
		t.Fatalf("providers = %+v", got.Providers)
	}
	if got.Providers[0].InputCostUSDPerM != nil || got.Providers[0].OutputCostUSDPerM != nil {
		t.Errorf("costs = %v/%v, want nil (no listed price)", got.Providers[0].InputCostUSDPerM, got.Providers[0].OutputCostUSDPerM)
	}
}

func TestCatalogModelCardDisabledOmitted(t *testing.T) {
	svc, _ := newTestServices(t)
	in := 15.0
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:         "anthropic",
		ModelID:          "claude-opus-5",
		InputCostUSDPerM: &in,
	}})
	got, err := svc.Catalog().Model(catCtx(), "Claude Opus 5")
	if err != nil {
		t.Fatalf("Model: %v", err)
	}
	if len(got.Providers) != 0 {
		t.Errorf("providers = %+v, want empty when claude is not enabled", got.Providers)
	}
}
func TestCatalogModelCardUnscoredProviderModel(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[providers.cursor]
enabled = true
`))
	in, out := 2.5, 10.0
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:          "cursor",
		ModelID:           "fable-5.1",
		Name:              "Fable 5.1",
		EffortLevels:      []string{"default", "high"},
		InputCostUSDPerM:  &in,
		OutputCostUSDPerM: &out,
	}})

	got, err := svc.Catalog().Model(catCtx(), "Fable 5.1")
	if err != nil {
		t.Fatalf("Model(Fable 5.1): %v", err)
	}
	if got.ModelName != "Fable 5.1" || got.ModelID != "fable-5.1" {
		t.Fatalf("identity = %s/%s, want Fable 5.1/fable-5.1", got.ModelName, got.ModelID)
	}
	if got.Intelligence != nil || got.Cost != nil || got.Speed != nil {
		t.Errorf("expected nil scores for unscored model, got intel=%v, cost=%v, speed=%v", got.Intelligence, got.Cost, got.Speed)
	}
	if got.InCatalog {
		t.Errorf("InCatalog = true, want false for unscored model")
	}
	if got.ProviderCount != 1 {
		t.Errorf("ProviderCount = %d, want 1", got.ProviderCount)
	}
	if len(got.Providers) != 1 {
		t.Fatalf("providers = %+v, want 1 (cursor enabled)", got.Providers)
	}
	p := got.Providers[0]
	if p.Provider != "cursor" || p.ModelID != "fable-5.1" {
		t.Errorf("provider row = %+v, want cursor/fable-5.1", p)
	}
	if p.InputCostUSDPerM == nil || *p.InputCostUSDPerM != 2.5 {
		t.Errorf("input cost = %v, want 2.5", p.InputCostUSDPerM)
	}
	if p.OutputCostUSDPerM == nil || *p.OutputCostUSDPerM != 10.0 {
		t.Errorf("output cost = %v, want 10.0", p.OutputCostUSDPerM)
	}

	// Also verify lookup by ModelID works for unscored model
	gotByID, err := svc.Catalog().Model(catCtx(), "fable-5.1")
	if err != nil {
		t.Fatalf("Model(fable-5.1): %v", err)
	}
	if gotByID.ModelName != "Fable 5.1" {
		t.Errorf("gotByID.ModelName = %q, want Fable 5.1", gotByID.ModelName)
	}
}

func TestCatalogModelCardUnscoredDisabledProvider(t *testing.T) {
	svc, _ := newTestServices(t) // cursor not enabled
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:     "cursor",
		ModelID:      "fable-5.1",
		Name:         "Fable 5.1",
		EffortLevels: []string{"default"},
	}})

	got, err := svc.Catalog().Model(catCtx(), "Fable 5.1")
	if err != nil {
		t.Fatalf("Model(Fable 5.1): %v", err)
	}
	if got.ModelName != "Fable 5.1" {
		t.Fatalf("modelName = %s, want Fable 5.1", got.ModelName)
	}
	if got.ProviderCount != 0 {
		t.Errorf("ProviderCount = %d, want 0 (provider disabled)", got.ProviderCount)
	}
	if len(got.Providers) != 0 {
		t.Errorf("providers = %+v, want empty when provider is disabled", got.Providers)
	}
}

func TestCatalogModelCardLookupByModelID(t *testing.T) {
	svc, _ := newTestServices(t)
	// Claude Opus 5 is in scores CSV; lookup by its model ID "claude-opus-5"
	got, err := svc.Catalog().Model(catCtx(), "claude-opus-5")
	if err != nil {
		t.Fatalf("Model(claude-opus-5): %v", err)
	}
	if got.ModelName != "Claude Opus 5" {
		t.Errorf("ModelName = %q, want Claude Opus 5", got.ModelName)
	}
	if got.Intelligence == nil || *got.Intelligence != 100 {
		t.Errorf("intelligence = %v, want 100", got.Intelligence)
	}
}
func TestCatalogModelCardProviderIDResolvesScoredModel(t *testing.T) {
	svc, _ := newTestServices(t)
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:     "github-copilot",
		ModelID:      "custom-opus",
		Name:         "Claude Opus 5",
		EffortLevels: []string{"high"},
	}})

	got, err := svc.Catalog().Model(catCtx(), "custom-opus")
	if err != nil {
		t.Fatalf("Model(custom-opus): %v", err)
	}
	if got.ModelName != "Claude Opus 5" {
		t.Errorf("ModelName = %q, want Claude Opus 5", got.ModelName)
	}
	if !got.InCatalog {
		t.Errorf("InCatalog = false, want true for scored model resolved via provider ID")
	}
	if got.Intelligence == nil || *got.Intelligence != 100 {
		t.Errorf("intelligence = %v, want 100", got.Intelligence)
	}
}

// TestCatalogModelCardDisabledLevelStillExposed pins B05 SPEC §2.15: the model
// card lists every effort level the provider EXPOSES, including levels switched
// off under [routes.disabled]. The card is a navigation surface into per-combo
// benchmarks and the Providers page owns the toggles, so a disabled level must
// still be reachable here. Provider-level enablement IS filtered (§2.15) and is
// covered by TestCatalogModelCardDisabledOmitted.
func TestCatalogModelCardDisabledLevelStillExposed(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[providers.claude]
enabled = true

[routes.disabled]
claude = ["claude-opus-5@max"]
`))

	got, err := svc.Catalog().Model(catCtx(), "Claude Opus 5")
	if err != nil {
		t.Fatalf("Model(Claude Opus 5): %v", err)
	}
	if len(got.Providers) != 1 {
		t.Fatalf("providers = %+v, want 1 (claude enabled)", got.Providers)
	}
	row := got.Providers[0]
	if !reflect.DeepEqual(row.Reasoning, []string{"high", "max"}) {
		t.Errorf("reasoning = %v, want [high max] (max disabled but still exposed)", row.Reasoning)
	}
	if !stringInSlice(row.RouteKeys, "claude/claude-opus-5@max") {
		t.Errorf("route keys = %v, want the disabled max combo still addressable", row.RouteKeys)
	}
}

// TestCatalogModelsProviderListingContributes pins B05 SPEC §2.14: ProviderCount
// and ModelID are a UNION of (a) every route carrying the name, regardless of
// provider enablement, and (b) the model listings of ENABLED providers, which
// include models.dev catalogue entries that have no route yet. Kimi K2.7 Code
// has no route in the fixture, so it isolates contribution (b): without it a
// user who enables a provider but has not run `routes refresh` would see a
// scored model reported as reachable from nowhere.
func TestCatalogModelsProviderListingContributes(t *testing.T) {
	base, _ := newTestServices(t)
	before, err := base.Catalog().Models(catCtx())
	if err != nil {
		t.Fatalf("Models (baseline): %v", err)
	}
	var baseKimi *CatalogModel
	for i := range before {
		if before[i].ModelName == "Kimi K2.7 Code" {
			baseKimi = &before[i]
		}
	}
	if baseKimi == nil {
		t.Fatal("Kimi K2.7 Code missing from baseline catalog")
	}
	if baseKimi.ProviderCount != 0 || baseKimi.ModelID != "" {
		t.Fatalf("baseline Kimi = count %d id %q, want 0 and empty (no routes)", baseKimi.ProviderCount, baseKimi.ModelID)
	}

	svc, _ := newTestServices(t, WithConfigTOML(`
[providers.cursor]
enabled = true
`))
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:     "cursor",
		ModelID:      "kimi-k2.7-code",
		Name:         "Kimi K2.7 Code",
		EffortLevels: []string{"high"},
	}})
	got, err := svc.Catalog().Models(catCtx())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	var kimi *CatalogModel
	for i := range got {
		if got[i].ModelName == "Kimi K2.7 Code" {
			kimi = &got[i]
		}
	}
	if kimi == nil {
		t.Fatal("Kimi K2.7 Code missing after seeding the provider listing")
	}
	if kimi.ProviderCount != 1 {
		t.Errorf("ProviderCount = %d, want 1 from the enabled provider listing alone", kimi.ProviderCount)
	}
	if kimi.ModelID != "kimi-k2.7-code" {
		t.Errorf("ModelID = %q, want kimi-k2.7-code from the provider listing", kimi.ModelID)
	}
	if !reflect.DeepEqual(kimi.Providers, []string{"cursor"}) {
		t.Errorf("Providers = %v, want [cursor]", kimi.Providers)
	}
}

// TestCatalogModelsDisabledProviderListingIgnored is the negative half of
// §2.14(b): the listing contribution is gated on provider enablement, so the
// same seed with cursor disabled leaves Kimi at zero.
func TestCatalogModelsDisabledProviderListingIgnored(t *testing.T) {
	svc, _ := newTestServices(t) // cursor not enabled
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:     "cursor",
		ModelID:      "kimi-k2.7-code",
		Name:         "Kimi K2.7 Code",
		EffortLevels: []string{"high"},
	}})
	got, err := svc.Catalog().Models(catCtx())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	for _, m := range got {
		if m.ModelName != "Kimi K2.7 Code" {
			continue
		}
		if m.ProviderCount != 0 || m.ModelID != "" {
			t.Errorf("Kimi = count %d id %q, want 0 and empty (cursor disabled)", m.ProviderCount, m.ModelID)
		}
		return
	}
	t.Fatal("Kimi K2.7 Code missing from catalog")
}

// TestCatalogModelCardSharedModelIDNoCrossProviderLeak is the aggregation-phase
// counterpart to the provider-scoped lookup fix. Provider-native ids are
// provider-scoped, so two unrelated models can share a generic id such as
// "default". Before the guard, synthesising the unscored "Alpha 1.0" card
// (base.ModelID "default") matched anthropic's "Claude Opus 5" purely on that
// bare id, leaking a second provider row and a "max" chip that drilled into the
// wrong benchmarks. The display name is authoritative: an id-only match whose
// name is a DIFFERENT catalog identity must be rejected.
func TestCatalogModelCardSharedModelIDNoCrossProviderLeak(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[providers.cursor]
enabled = true

[providers.claude]
enabled = true
`))
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{
		{Provider: "cursor", ModelID: "default", Name: "Alpha 1.0", EffortLevels: []string{"high"}},
		{Provider: "anthropic", ModelID: "default", Name: "Claude Opus 5", EffortLevels: []string{"max"}},
	})

	got, err := svc.Catalog().Model(catCtx(), "Alpha 1.0")
	if err != nil {
		t.Fatalf("Model(Alpha 1.0): %v", err)
	}
	if got.ModelName != "Alpha 1.0" || got.InCatalog {
		t.Fatalf("identity = %q inCatalog=%v, want Alpha 1.0 and false", got.ModelName, got.InCatalog)
	}
	if len(got.Providers) != 1 || got.Providers[0].Provider != "cursor" {
		t.Fatalf("providers = %+v, want only the cursor row", got.Providers)
	}
	if got.ProviderCount != 1 {
		t.Errorf("ProviderCount = %d, want 1", got.ProviderCount)
	}
	if stringInSlice(got.Reasoning, "max") {
		t.Errorf("reasoning = %v, leaked Claude Opus 5's max level via the shared id", got.Reasoning)
	}

	// The scored neighbour must remain intact and unaffected by the guard.
	opus, err := svc.Catalog().Model(catCtx(), "Claude Opus 5")
	if err != nil {
		t.Fatalf("Model(Claude Opus 5): %v", err)
	}
	if !opus.InCatalog {
		t.Errorf("Claude Opus 5 InCatalog = false, want true")
	}
	for _, p := range opus.Providers {
		if p.Provider == "cursor" {
			t.Errorf("Alpha 1.0's cursor row leaked onto the Claude Opus 5 card: %+v", p)
		}
	}
}

// TestCatalogModelCardTwoUnscoredShareModelID is the case a scored-name
// heuristic cannot catch: BOTH models are unscored, so neither display name
// appears in the scores CSV and there is no catalog identity to arbitrate.
// Only provider scoping — an id is meaningful solely within the provider that
// serves the model by name — keeps the two cards apart.
func TestCatalogModelCardTwoUnscoredShareModelID(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[providers.cursor]
enabled = true

[providers.antigravity]
enabled = true
`))
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{
		{Provider: "cursor", ModelID: "default", Name: "Alpha 1.0", EffortLevels: []string{"high"}},
		{Provider: "antigravity", ModelID: "default", Name: "Beta 9.9", EffortLevels: []string{"low"}},
	})

	for _, tc := range []struct {
		name     string
		provider string
		level    string
		otherLvl string
	}{
		{"Alpha 1.0", "cursor", "high", "low"},
		{"Beta 9.9", "antigravity", "low", "high"},
	} {
		got, err := svc.Catalog().Model(catCtx(), tc.name)
		if err != nil {
			t.Fatalf("Model(%s): %v", tc.name, err)
		}
		if got.InCatalog {
			t.Errorf("%s InCatalog = true, want false (unscored)", tc.name)
		}
		if len(got.Providers) != 1 || got.Providers[0].Provider != tc.provider {
			t.Fatalf("%s providers = %+v, want only %s", tc.name, got.Providers, tc.provider)
		}
		if got.ProviderCount != 1 {
			t.Errorf("%s ProviderCount = %d, want 1", tc.name, got.ProviderCount)
		}
		if !stringInSlice(got.Reasoning, tc.level) {
			t.Errorf("%s reasoning = %v, want its own %s level", tc.name, got.Reasoning, tc.level)
		}
		if stringInSlice(got.Reasoning, tc.otherLvl) {
			t.Errorf("%s reasoning = %v, leaked the other model's %s level", tc.name, got.Reasoning, tc.otherLvl)
		}
	}
}

func TestCatalogGroupsList(t *testing.T) {
	svc, _ := newTestServices(t)
	got, err := svc.Catalog().Groups(catCtx())
	if err != nil {
		t.Fatalf("Groups: %v", err)
	}
	want := []GroupSummary{
		{Slug: "software_engineering", Builtin: true, BenchmarkCount: 11, InProfiles: 5},
		{Slug: "finance", Builtin: true, BenchmarkCount: 5, InProfiles: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Groups = %+v, want %+v", got, want)
	}
}

func TestCatalogGroupsCustoms(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[groups.zz_group]
benchmarks = ["SWE-Bench Verified"]

[groups.bb_group]
benchmarks = ["Terminal-Bench", "MCP Atlas"]

[profiles.custom_p]
core_share = 60
tier1 = { intelligence = 3, cost = 3, speed = 3 }
tier2 = { bb_group = 4 }
`))
	got, err := svc.Catalog().Groups(catCtx())
	if err != nil {
		t.Fatalf("Groups: %v", err)
	}
	want := []GroupSummary{
		{Slug: "software_engineering", Builtin: true, BenchmarkCount: 11, InProfiles: 5},
		{Slug: "finance", Builtin: true, BenchmarkCount: 5, InProfiles: 1},
		{Slug: "bb_group", BenchmarkCount: 2, InProfiles: 1},
		{Slug: "zz_group", BenchmarkCount: 1, InProfiles: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Groups = %+v, want %+v", got, want)
	}
}

func TestCatalogGroupDetail(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[groups.my_group]
benchmarks = ["SWE-Bench Verified", "Terminal-Bench"]
`))
	c := svc.Catalog()

	// Unknown slug -> not_found.
	if _, err := c.GroupDetail(catCtx(), "nope"); !errors.Is(err, errNotFound) {
		t.Fatalf("GroupDetail unknown: err = %v, want errNotFound", err)
	}

	d, err := c.GroupDetail(catCtx(), "my_group")
	if err != nil {
		t.Fatalf("GroupDetail: %v", err)
	}
	if d.Slug != "my_group" || d.Builtin {
		t.Errorf("detail = %+v, want custom slug my_group", d)
	}
	if len(d.Benchmarks) != 16 {
		t.Fatalf("len(Benchmarks) = %d, want 16", len(d.Benchmarks))
	}
	on := map[string]bool{}
	cov := map[string]int{}
	for _, b := range d.Benchmarks {
		on[b.Name] = b.On
		cov[b.Name] = b.Covered
	}
	if !on["SWE-Bench Verified"] || !on["Terminal-Bench"] {
		t.Errorf("On membership wrong: %v", on)
	}
	if on["MCP Atlas"] || on["Finance Agent"] {
		t.Errorf("non-members marked On: %v", on)
	}
	// Coverage mirrors the golden raw cells (SWE-Bench Verified 1, Terminal-Bench 1).
	if cov["SWE-Bench Verified"] != 1 || cov["Terminal-Bench"] != 1 {
		t.Errorf("coverage = %v", cov)
	}
}

func TestCatalogSaveGroup(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[groups.my_group]
benchmarks = ["SWE-Bench Verified"]
`))
	writeRawValues(t, svc)
	c := svc.Catalog()

	if err := c.SaveGroup(catCtx(), "my_group", []string{"SWE-Bench Verified", "Terminal-Bench"}, ""); err != nil {
		t.Fatalf("SaveGroup: %v", err)
	}

	// Persisted: group member list updated under the same slug.
	cfg := diskCfg(t, svc)
	groups, err := cfg.LoadGroups()
	if err != nil {
		t.Fatalf("LoadGroups: %v", err)
	}
	got := groups["my_group"]
	if !reflect.DeepEqual(got.Benchmarks, []string{"SWE-Bench Verified", "Terminal-Bench"}) {
		t.Fatalf("group members = %v", got.Benchmarks)
	}

	// Re-derived: scores CSV on disk rewritten, and the custom category
	// overlay applied to the live cache rows.
	if st, err := os.Stat(filepath.Join(svc.paths.CacheDir, "catalog", "available_model_scores.csv")); err != nil {
		t.Fatalf("stat scores: %v", err)
	} else if st.Size() == 0 {
		t.Errorf("scores CSV empty after re-derive")
	}
	found := 0
	for _, row := range svc.scores {
		if v, ok := row.Categories["my_group"]; ok && v.GreaterThan(sdecimal.Zero) {
			found++
		}
	}
	if found == 0 {
		t.Errorf("no row overlaid Categories[my_group] after save")
	}

	// Exactly one event: catalog:changed.
	evs := rec.Events()
	if len(evs) != 1 || evs[0].Event != EventCatalogChanged {
		t.Fatalf("events = %+v, want exactly one catalog:changed", evs)
	}
}

func TestCatalogSaveGroupValidationOrder(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[groups.my_group]
benchmarks = ["SWE-Bench Verified"]
`))
	c := svc.Catalog()
	ctx := catCtx()

	// 1. builtin slug -> builtin_readonly.
	if err := c.SaveGroup(ctx, "software_engineering", []string{"SWE-Bench Verified"}, ""); !errors.Is(err, errBuiltinReadonly) {
		t.Fatalf("SaveGroup builtin: err = %v, want errBuiltinReadonly", err)
	}
	// 2. unknown slug -> not_found.
	if err := c.SaveGroup(ctx, "ghost", []string{"SWE-Bench Verified"}, ""); !errors.Is(err, errNotFound) {
		t.Fatalf("SaveGroup unknown slug: err = %v, want errNotFound", err)
	}
	// 3. unknown benchmark (first offender) -> validation_failed.
	if err := c.SaveGroup(ctx, "my_group", []string{"Not A Benchmark"}, ""); !errors.Is(err, errValidation) {
		t.Fatalf("SaveGroup unknown bench: err = %v, want errValidation", err)
	} else if !strings.Contains(err.Error(), `unknown benchmark "Not A Benchmark" in group "my_group"`) {
		t.Fatalf("SaveGroup unknown bench msg = %q", err)
	}
	// 4. renameTo collides with an existing group -> conflict.
	if err := c.SaveGroup(ctx, "my_group", []string{"SWE-Bench Verified"}, "software_engineering"); !errors.Is(err, errConflict) {
		t.Fatalf("SaveGroup rename collision: err = %v, want errConflict", err)
	}

	// All validation failures were read-only: no events, and a read path emits
	// nothing too.
	if len(rec.Events()) != 0 {
		t.Fatalf("events after validation-only path = %+v, want none", rec.Events())
	}
	if _, err := c.Benchmarks(ctx); err != nil {
		t.Fatal(err)
	}
	if len(rec.Events()) != 0 {
		t.Fatalf("read path emitted events: %+v", rec.Events())
	}
}

func TestCatalogRenameRewritesProfileWeights(t *testing.T) {
	// sanitizeGroupSlug ("My Group! " -> "my_group") matches SPEC §2.7.
	if got := sanitizeGroupSlug("My Group! "); got != "my_group" {
		t.Fatalf("sanitizeGroupSlug(\"My Group! \") = %q, want my_group", got)
	}
	if got := sanitizeGroupSlug("___"); got != "" {
		t.Fatalf("sanitizeGroupSlug(\"___\") = %q, want \"\"", got)
	}

	svc, rec := newTestServices(t, WithConfigTOML(`
[groups.my_group]
benchmarks = ["SWE-Bench Verified"]

[profiles.deep_work]
core_share = 65
tier1 = { intelligence = 4, cost = 3, speed = 2 }
tier2 = { software_engineering = 5, my_group = 4 }
`))
	writeRawValues(t, svc)
	c := svc.Catalog()

	if err := c.SaveGroup(catCtx(), "my_group", []string{"SWE-Bench Verified", "Terminal-Bench"}, "Renamed"); err != nil {
		t.Fatalf("SaveGroup rename: %v", err)
	}

	cfg := diskCfg(t, svc)
	groups, err := cfg.LoadGroups()
	if err != nil {
		t.Fatalf("LoadGroups: %v", err)
	}
	if _, ok := groups["my_group"]; ok {
		t.Errorf("old slug my_group still present after rename")
	}
	renamed, ok := groups["renamed"]
	if !ok {
		t.Fatalf("renamed group not found: %v", groups)
	}
	if !reflect.DeepEqual(renamed.Benchmarks, []string{"SWE-Bench Verified", "Terminal-Bench"}) {
		t.Errorf("renamed members = %v", renamed.Benchmarks)
	}

	// tier2 weight key moved my_group -> renamed in the custom profile.
	profiles, err := cfg.LoadProfiles(categoryNamesForTest())
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	p := profiles["deep_work"]
	if _, ok := p.Tier2["my_group"]; ok {
		t.Errorf("tier2 my_group not stripped from profile")
	}
	if p.Tier2["renamed"] != 4 {
		t.Errorf("tier2 renamed = %d, want 4", p.Tier2["renamed"])
	}
	if len(rec.Events()) != 1 || rec.Events()[0].Event != EventCatalogChanged {
		t.Fatalf("events = %+v, want one catalog:changed", rec.Events())
	}
}

func TestCatalogDuplicateDelete(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[groups.my_group]
benchmarks = ["SWE-Bench Verified", "Terminal-Bench"]

[profiles.deep_work]
core_share = 65
tier1 = { intelligence = 4, cost = 3, speed = 2 }
tier2 = { software_engineering = 5, my_group = 4 }
`))
	writeRawValues(t, svc)
	c := svc.Catalog()
	ctx := catCtx()

	// Duplicate a custom group -> my_group_copy with the same members.
	d, err := c.DuplicateGroup(ctx, "my_group")
	if err != nil {
		t.Fatalf("DuplicateGroup: %v", err)
	}
	if d.Slug != "my_group_copy" || d.Builtin {
		t.Fatalf("duplicate detail = %+v", d)
	}
	var copied []string
	for _, b := range d.Benchmarks {
		if b.On {
			copied = append(copied, b.Name)
		}
	}
	if !reflect.DeepEqual(copied, []string{"SWE-Bench Verified", "Terminal-Bench"}) {
		t.Fatalf("copy members = %v", copied)
	}
	// Duplicate again -> my_group_copy_2 (first free).
	d2, err := c.DuplicateGroup(ctx, "my_group")
	if err != nil {
		t.Fatalf("DuplicateGroup 2: %v", err)
	}
	if d2.Slug != "my_group_copy_2" {
		t.Fatalf("second duplicate slug = %q, want my_group_copy_2", d2.Slug)
	}

	// Duplicate spanning builtin groups is allowed.
	// (software_engineering is built-in; a copy is fully custom.)
	d3, err := c.DuplicateGroup(ctx, "software_engineering")
	if err != nil {
		t.Fatalf("DuplicateGroup builtin: %v", err)
	}
	if d3.Slug != "software_engineering_copy" {
		t.Fatalf("builtin duplicate slug = %q", d3.Slug)
	}

	// The three successful duplicates above each emitted catalog:changed.
	if countEvents(rec, EventCatalogChanged) != 3 {
		t.Fatalf("after 3 duplicates, catalog:changed count = %d, want 3: %+v", countEvents(rec, EventCatalogChanged), rec.Events())
	}
	base := len(rec.Events())

	// Delete a builtin -> builtin_readonly, no new events.
	if err := c.DeleteGroup(ctx, "software_engineering"); !errors.Is(err, errBuiltinReadonly) {
		t.Fatalf("DeleteGroup builtin: err = %v, want errBuiltinReadonly", err)
	}
	if len(rec.Events()) != base {
		t.Fatalf("builtin delete emitted events: %+v", rec.Events())
	}
	// Delete unknown -> not_found, no new events.
	if err := c.DeleteGroup(ctx, "ghost"); !errors.Is(err, errNotFound) {
		t.Fatalf("DeleteGroup unknown: err = %v, want errNotFound", err)
	}
	if len(rec.Events()) != base {
		t.Fatalf("unknown delete emitted events: %+v", rec.Events())
	}

	// Delete a custom group -> removed + tier2 stripped + one catalog:changed.
	if err := c.DeleteGroup(ctx, "my_group"); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	cfg := diskCfg(t, svc)
	groups, err := cfg.LoadGroups()
	if err != nil {
		t.Fatalf("LoadGroups: %v", err)
	}
	if _, ok := groups["my_group"]; ok {
		t.Errorf("my_group still present after delete")
	}
	profiles, err := cfg.LoadProfiles(categoryNamesForTest())
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if _, ok := profiles["deep_work"].Tier2["my_group"]; ok {
		t.Errorf("tier2 my_group not stripped on delete")
	}
	if len(rec.Events()) != base+1 || rec.Events()[base].Event != EventCatalogChanged {
		t.Fatalf("events = %+v, want exactly one catalog:changed after custom delete", rec.Events())
	}
}

func TestCatalogDeliverableBuiltinMutationRejected(t *testing.T) {
	svc, rec := newTestServices(t)
	c := svc.Catalog()
	ctx := catCtx()

	if err := c.SaveGroup(ctx, "finance", []string{"SWE-Bench Verified"}, ""); !errors.Is(err, errBuiltinReadonly) {
		t.Fatalf("SaveGroup builtin: err = %v, want errBuiltinReadonly", err)
	}
	if err := c.DeleteGroup(ctx, "finance"); !errors.Is(err, errBuiltinReadonly) {
		t.Fatalf("DeleteGroup builtin: err = %v, want errBuiltinReadonly", err)
	}
	if len(rec.Events()) != 0 {
		t.Fatalf("builtin mutations emitted events: %+v", rec.Events())
	}

	// Config untouched on disk.
	cfg := diskCfg(t, svc)
	groups, err := cfg.LoadGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("builtin mutations changed [groups.*]: %v", groups)
	}
}

func TestCatalogDeriveFailurePersistsGroup(t *testing.T) {
	// No raw CSV written here, so the re-derive step fails on the missing raw
	// file — but the group edit is already persisted (SPEC §2.12).
	svc, rec := newTestServices(t, WithConfigTOML(`
[groups.my_group]
benchmarks = ["SWE-Bench Verified"]
`))
	c := svc.Catalog()
	ctx := catCtx()

	beforeScores := append([]catalog.ScoreRow(nil), svc.scores...)
	beforeRaw := shallowCopyRawValues(svc.rawValues)

	err := c.SaveGroup(ctx, "my_group", []string{"SWE-Bench Verified", "Terminal-Bench"}, "")
	if err == nil {
		t.Fatal("SaveGroup unexpectedly succeeded without a raw CSV")
	}
	if !strings.Contains(err.Error(), "available_model_raw_values.csv") {
		t.Fatalf("derive failure err = %q, want it to name the raw CSV path", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("derive failure err = %v, want fs.ErrNotExist", err)
	}

	// Group persist stayed durable even though derive failed.
	cfg := diskCfg(t, svc)
	groups, err := cfg.LoadGroups()
	if err != nil {
		t.Fatalf("LoadGroups: %v", err)
	}
	got := groups["my_group"]
	if !reflect.DeepEqual(got.Benchmarks, []string{"SWE-Bench Verified", "Terminal-Bench"}) {
		t.Fatalf("persisted group members = %v, want the saved list", got.Benchmarks)
	}

	// Catalog cache unchanged.
	if !reflect.DeepEqual(svc.scores, beforeScores) {
		t.Errorf("scores cache changed after derive failure")
	}
	if !reflect.DeepEqual(svc.rawValues, beforeRaw) {
		t.Errorf("rawValues cache changed after derive failure")
	}

	// Events: config:changed + the success-path catalog:changed is suppressed.
	if countEvents(rec, EventCatalogChanged) != 0 {
		t.Errorf("catalog:changed emitted on derive failure: %+v", rec.Events())
	}
	if countEvents(rec, EventConfigChanged) != 1 {
		t.Errorf("config:changed count = %d, want 1: %+v", countEvents(rec, EventConfigChanged), rec.Events())
	}
}

// shallowCopyRawValues copies the benchmark -> key -> value map deeply enough
// to snapshot a catalog cache for change detection.
func shallowCopyRawValues(m map[string]map[modelKey]sdecimal.Decimal) map[string]map[modelKey]sdecimal.Decimal {
	out := make(map[string]map[modelKey]sdecimal.Decimal, len(m))
	for name, inner := range m {
		innerCopy := make(map[modelKey]sdecimal.Decimal, len(inner))
		for k, v := range inner {
			innerCopy[k] = v
		}
		out[name] = innerCopy
	}
	return out
}

// categoryNamesForTest mirrors the canonical tier2 vocabulary used by the
// fixture (pick.CategoryNames), enough for LoadProfiles validation on disk.
func categoryNamesForTest() []string {
	return []string{
		"intelligence", "cost", "speed",
		"reasoning", "knowledge", "research", "planning_capability",
		"instruction_following", "software_engineering", "ui_visual",
		"agentic_tools", "finance", "evidence_capture", "security", "data_ml",
	}
}
