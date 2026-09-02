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
