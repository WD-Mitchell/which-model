// F27: routes logic (specs/features/F27-cmd-routes/). All route state is
// read/written through the F18 seams below; F27 never re-implements merge or
// persistence (routing.SaveTable is the only writer).
package whichmodel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/output"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

// ScoreRow is the F27 view of a scores-CSV row: the identity pair plus the
// provider that owns the row. The F06 canonical row does not carry a
// provider; --auto needs one to build the route (F27 TASKS T5 pin).
type ScoreRow struct {
	Model     string
	Reasoning string
	Provider  string
}

// RouteAddArgs is the validated input of `routes add`.
type RouteAddArgs struct {
	Provider   string   // registry id
	ModelID    string   // provider-side model id
	Model      string   // scored catalog model name
	Reasoning  string   // score-row reasoning key; default "default"
	Windows    []string // --window, repeatable
	ConfigPath string
}

// RouteRemoveArgs is the validated input of `routes remove`.
type RouteRemoveArgs struct {
	Provider, ModelID, ConfigPath string
}

// RouteListArgs is the validated input of `routes list`.
type RouteListArgs struct {
	Provider   string // "" = all
	JSON       bool
	ConfigPath string
}

// RouteRefreshArgs is the validated input of `routes refresh`.
type RouteRefreshArgs struct {
	Auto       string // "" = none
	ConfigPath string
}

// RouteVerifyArgs is the validated input of `routes verify`.
type RouteVerifyArgs struct {
	JSON       bool
	ConfigPath string
}

// RouteList is the list --json document root (F27 CONTRACTS §5).
type RouteList struct {
	SchemaVersion string          `json:"schema_version"` // "2.0"
	Routes        []routing.Route `json:"routes"`         // canonical routing.Route JSON tags
}

// VerifyReport is the verify --json document root (F27 SPEC §2.7).
type VerifyReport struct {
	SchemaVersion       string         `json:"schema_version"`    // "2.0"
	StaleRoutes         []string       `json:"stale_routes"`      // "<provider>:<model-id>"
	Unrouted            []ScoreRef     `json:"unrouted"`          // score rows without routes
	ProvenanceCounts    map[string]int `json:"provenance_counts"` // user_declared|provider_live|models_dev
	ScoresSHA256Matches bool           `json:"scores_sha256_matches"`
}

// ScoreRef is one unrouted score-row identity (F27 CONTRACTS §2).
type ScoreRef struct {
	Model     string `json:"model"`
	Reasoning string `json:"reasoning"`
}

// F18/F06/F21 seams, injectable in tests. Defaults are the landed upstream
// funcs (via the local adapters below).
var (
	loadRoutesFunc    func(path string) (routing.Table, error)                  // default: LoadTable, missing → empty
	saveRoutesFunc    func(path string, t routing.Table) error                  // default: routing.SaveTable
	routesPathFunc    func(cfg *config.Config) (string, error)                  // default: <cache_dir>/routes.json
	produceRoutesFunc func(cfg *config.Config) ([]routing.Route, error)         // default: F18 pipeline adapter
	readScoresFunc    func(path string) ([]ScoreRow, error)                     // default: csvstore reader
	scoresSHA256Func  func(cfg *config.Config) (string, error)                  // default: csvstore.ProvenanceHash
	toggleResolveFunc func(flagNoUsage bool, cfg *config.Config) (bool, string) // default: toggle.ResolveUsageEnabled
)

func init() {
	loadRoutesFunc = loadRoutes
	saveRoutesFunc = routing.SaveTable
	routesPathFunc = routesPath
	produceRoutesFunc = produceRoutes
	readScoresFunc = readScores
	scoresSHA256Func = scoresSHA256
	toggleResolveFunc = toggle.ResolveUsageEnabled
}

// defaultPaths resolves the platform cache/state directories (same
// convention as the catalog pipeline, F23).
func defaultPaths() config.Paths {
	home, _ := os.UserHomeDir()
	return config.ResolvePaths(runtime.GOOS, home, os.Getenv)
}

// loadRoutes wraps routing.LoadTable with F27's empty-table semantics: a
// missing file is the first run, not an error (F27 TASKS T4/T6).
func loadRoutes(path string) (routing.Table, error) {
	t, err := routing.LoadTable(path)
	if os.IsNotExist(err) {
		return routing.Table{}, nil
	}
	return t, err
}

// routesPath resolves the route table location: <cache_dir>/routes.json
// (F18 SPEC §2.11; annex-d §4.5 cache directory).
func routesPath(cfg *config.Config) (string, error) {
	return filepath.Join(defaultPaths().CacheDir, "routes.json"), nil
}

// scoresCSVPath resolves the scores CSV from [catalog] scores_csv_path,
// defaulting to the cache tree like the catalog pipeline.
func scoresCSVPath(cfg *config.Config) (string, error) {
	p := ""
	if err := cfg.UnmarshalKey("catalog.scores_csv_path", &p); err != nil {
		return "", err
	}
	if p == "" {
		p = filepath.Join(defaultPaths().CacheDir, "catalog", "available_model_scores.csv")
	}
	return p, nil
}

// readScores reads the scores CSV via csvstore and extracts the model /
// reasoning identity columns (header-driven).
func readScores(path string) ([]ScoreRow, error) {
	rows, _, err := csvstore.Read(path)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []ScoreRow{}, nil
	}
	modelIdx, reasoningIdx := -1, -1
	for i, h := range rows[0].Header {
		switch h {
		case "model":
			modelIdx = i
		case "reasoning":
			reasoningIdx = i
		}
	}
	if modelIdx < 0 || reasoningIdx < 0 {
		return nil, fmt.Errorf("scores CSV %s: missing model/reasoning columns", path)
	}
	out := make([]ScoreRow, 0, len(rows))
	for _, row := range rows {
		if modelIdx >= len(row.Values) || reasoningIdx >= len(row.Values) {
			continue
		}
		out = append(out, ScoreRow{Model: row.Values[modelIdx], Reasoning: row.Values[reasoningIdx]})
	}
	return out, nil
}

// scoresSHA256 hashes the current scores CSV bytes (F18's ScoresHash source).
func scoresSHA256(cfg *config.Config) (string, error) {
	p, err := scoresCSVPath(cfg)
	if err != nil {
		return "", err
	}
	return csvstore.ProvenanceHash(p)
}

// produceRoutes adapts the F18 production pipeline to the F27 seam: it
// assembles the routing.Input from config (catalog identities, the current
// table's user-declared routes, registry provider descriptors) and returns
// exactly the routes ProduceRoutes built (F27 never re-implements merge).
func produceRoutes(cfg *config.Config) ([]routing.Route, error) {
	input := routing.Input{
		Providers: make([]routing.ProviderInput, 0, len(routeProviders())),
	}
	for _, p := range routeProviders() {
		input.Providers = append(input.Providers, routing.ProviderInput{
			Provider: p.ID,
			Kind:     p.Kind,
			Windows:  p.Windows,
		})
	}
	enabled, _ := toggle.ResolveUsageEnabled(Global.NoUsage, cfg)
	input.Degraded = !enabled

	csvPath, err := scoresCSVPath(cfg)
	if err != nil {
		return nil, err
	}
	rows, err := readScoresFunc(csvPath)
	if err != nil && !errors.Is(err, csvstore.ErrMissingFile) {
		return nil, err
	}
	input.CatalogRows = make([]identity.Identity, 0, len(rows))
	for _, row := range rows {
		input.CatalogRows = append(input.CatalogRows, identity.IdentityKey(row.Model, row.Reasoning))
	}

	path, err := routesPathFunc(cfg)
	if err != nil {
		return nil, err
	}
	table, err := loadRoutesFunc(path)
	if err != nil {
		return nil, err
	}
	for _, r := range table.Routes {
		if r.Provenance == routing.ProvenanceUserDeclared {
			input.Providers = append(input.Providers, routing.ProviderInput{
				Provider: r.Provider,
				UserDeclared: []routing.UserDeclaredRoute{{
					Provider:  r.Provider,
					ModelID:   r.ModelID,
					Model:     r.Model,
					Reasoning: r.Reasoning,
					WindowIDs: r.WindowIDs,
				}},
			})
		}
	}

	result, err := routing.ProduceRoutes(input)
	if err != nil {
		return nil, err
	}
	return result.Routes, nil
}

// RunRouteAdd implements `routes add` (F27 SPEC §2.2): validate, load, reject
// duplicates, append a user_declared route, persist.
func RunRouteAdd(args RouteAddArgs, stdout, stderr io.Writer) error {
	if args.Provider == "" {
		return &UsageError{Message: "--provider is required"}
	}
	if !providerExists(args.Provider) {
		return &UsageError{Message: fmt.Sprintf("unknown provider %q; valid providers: %s", args.Provider, strings.Join(providerIDs(), ", "))}
	}
	if args.ModelID == "" || args.Model == "" {
		return &UsageError{Message: "--model-id and --model are required"}
	}
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	path, err := routesPathFunc(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	table, err := loadRoutesFunc(path)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	for _, r := range table.Routes {
		if r.Provider == args.Provider && r.ModelID == args.ModelID {
			return &UsageError{Message: fmt.Sprintf("route %q already exists; remove it first", args.Provider+":"+args.ModelID)}
		}
	}
	reasoning := args.Reasoning
	if reasoning == "" {
		reasoning = "default"
	}
	table.Routes = append(table.Routes, routing.Route{
		Provider:   args.Provider,
		ModelID:    args.ModelID,
		Model:      args.Model,
		Reasoning:  reasoning,
		WindowIDs:  args.Windows,
		Provenance: routing.ProvenanceUserDeclared,
	})
	if err := saveRoutesFunc(path, table); err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	return nil
}

// RunRouteRemove implements `routes remove` (F27 SPEC §2.3): exact
// (provider, model-id) match, silent success, no_route when absent.
func RunRouteRemove(args RouteRemoveArgs, stdout, stderr io.Writer) error {
	if args.Provider == "" {
		return &UsageError{Message: "--provider is required"}
	}
	if args.ModelID == "" {
		return &UsageError{Message: "--model-id is required"}
	}
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	path, err := routesPathFunc(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	table, err := loadRoutesFunc(path)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	idx := -1
	for i, r := range table.Routes {
		if r.Provider == args.Provider && r.ModelID == args.ModelID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return &CodedError{Code: "no_route", Message: fmt.Sprintf("no route %q", args.Provider+":"+args.ModelID)}
	}
	table.Routes = append(table.Routes[:idx], table.Routes[idx+1:]...)
	if err := saveRoutesFunc(path, table); err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	return nil
}

// RunRouteList implements `routes list` (F27 SPEC §2.4): table or JSON
// document; missing routes file is an empty table.
func RunRouteList(args RouteListArgs, stdout, stderr io.Writer) error {
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	path, err := routesPathFunc(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	table, err := loadRoutesFunc(path)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	routes := table.Routes
	if args.Provider != "" {
		if !providerExists(args.Provider) {
			return &UsageError{Message: fmt.Sprintf("unknown provider %q", args.Provider)}
		}
		filtered := make([]routing.Route, 0, len(routes))
		for _, r := range routes {
			if r.Provider == args.Provider {
				filtered = append(filtered, r)
			}
		}
		routes = filtered
	}
	if args.JSON {
		if routes == nil {
			routes = []routing.Route{}
		}
		return writeJSONDoc(stdout, RouteList{SchemaVersion: "2.0", Routes: routes})
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "provider\tmodel_id\tmodel\treasoning\twindows\tprovenance")
	for _, r := range routes {
		windows := strings.Join(r.WindowIDs, ",")
		if len(r.WindowIDs) == 0 {
			windows = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Provider, r.ModelID, r.Model, r.Reasoning, windows, r.Provenance)
	}
	return w.Flush()
}

// RunRouteRefresh implements `routes refresh` (F27 SPEC §2.5, §2.6):
// F18 production + compare-and-skip persistence; --auto adds one
// user_declared route from a decisive fuzzy score-row match.
func RunRouteRefresh(args RouteRefreshArgs, stdout, stderr io.Writer) error {
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	enabled, _ := toggleResolveFunc(Global.NoUsage, cfg)
	if !enabled {
		_ = output.WriteWarning(stderr, "usage is disabled; refresh uses static sources only")
	}
	routes, err := produceRoutesFunc(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	path, err := routesPathFunc(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	if args.Auto != "" {
		csvPath, err := scoresCSVPath(cfg)
		if err != nil {
			return &CodedError{Code: "runtime", Message: err.Error()}
		}
		rows, err := readScoresFunc(csvPath)
		if err != nil {
			return &CodedError{Code: "runtime", Message: err.Error()}
		}
		want := strings.ToLower(args.Auto)
		var matches []ScoreRow
		for _, row := range rows {
			if strings.Contains(strings.ToLower(row.Model), want) {
				matches = append(matches, row)
			}
		}
		switch len(matches) {
		case 0:
			return &UsageError{Message: fmt.Sprintf("no score row matching %q", args.Auto)}
		case 1:
			routes = append(routes, routing.Route{
				Provider:   matches[0].Provider,
				ModelID:    matches[0].Model,
				Model:      matches[0].Model,
				Reasoning:  "default",
				Provenance: routing.ProvenanceUserDeclared,
			})
		default:
			models := make([]string, len(matches))
			for i, m := range matches {
				models[i] = m.Model
			}
			return &UsageError{Message: fmt.Sprintf("no score row matching %q (ambiguous: %s)", args.Auto, strings.Join(models, ", "))}
		}
	}
	hash, _ := scoresSHA256Func(cfg) // unreadable CSV → no recorded hash, never fatal
	current, err := loadRoutesFunc(path)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	table := routing.Table{
		SchemaVersion: routing.TableSchemaVersion,
		ScoresHash:    hash,
		RefreshedAt:   current.RefreshedAt,
		Routes:        routes,
	}
	produced, err := json.Marshal(table)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	existing, err := json.Marshal(current)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	if bytes.Equal(produced, existing) {
		return nil
	}
	if err := saveRoutesFunc(path, table); err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	return nil
}

// RunRouteVerify implements `routes verify` (F27 SPEC §2.7): stale routes on
// stdout (or the VerifyReport document under --json), warnings and the
// provenance summary on stderr; stale → ReportedError(stale_routes).
func RunRouteVerify(args RouteVerifyArgs, stdout, stderr io.Writer) error {
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	path, err := routesPathFunc(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	table, err := loadRoutesFunc(path)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	csvPath, err := scoresCSVPath(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	rows, err := readScoresFunc(csvPath)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}

	scoreSet := make(map[identity.Identity]bool, len(rows))
	for _, row := range rows {
		scoreSet[identity.IdentityKey(row.Model, row.Reasoning)] = true
	}
	routeSet := make(map[identity.Identity]bool, len(table.Routes))
	for _, r := range table.Routes {
		routeSet[identity.IdentityKey(r.Model, r.Reasoning)] = true
	}

	var stale []routing.Route
	for _, r := range table.Routes {
		if !scoreSet[identity.IdentityKey(r.Model, r.Reasoning)] {
			stale = append(stale, r)
		}
	}
	var unrouted []ScoreRef
	for _, row := range rows {
		if !routeSet[identity.IdentityKey(row.Model, row.Reasoning)] {
			unrouted = append(unrouted, ScoreRef{Model: row.Model, Reasoning: row.Reasoning})
		}
	}

	counts := map[string]int{"user_declared": 0, "provider_live": 0, "models_dev": 0}
	for _, r := range table.Routes {
		switch r.Provenance {
		case routing.ProvenanceUserDeclared:
			counts["user_declared"]++
		case routing.ProvenanceProviderLive:
			counts["provider_live"]++
		case routing.ProvenanceModelsDev:
			counts["models_dev"]++
		}
	}

	liveHash, _ := scoresSHA256Func(cfg) // unreadable CSV → no live hash, no warning
	storedHash := table.ScoresHash
	matches := liveHash != "" && storedHash != "" && liveHash == storedHash
	if liveHash != "" && storedHash != "" && liveHash != storedHash {
		_ = output.WriteWarning(stderr, "scores CSV changed since routes were produced; run which-model routes refresh")
	}
	for _, u := range unrouted {
		_ = output.WriteWarning(stderr, fmt.Sprintf("score row %s/%s has no route; it cannot be picked", u.Model, u.Reasoning))
	}
	fmt.Fprintf(stderr, "routes: %d total (%d user_declared, %d provider_live, %d models_dev)\n",
		len(table.Routes), counts["user_declared"], counts["provider_live"], counts["models_dev"])

	if args.JSON {
		report := VerifyReport{
			SchemaVersion:       "2.0",
			StaleRoutes:         []string{},
			Unrouted:            []ScoreRef{},
			ProvenanceCounts:    counts,
			ScoresSHA256Matches: matches,
		}
		for _, r := range stale {
			report.StaleRoutes = append(report.StaleRoutes, r.Provider+":"+r.ModelID)
		}
		report.Unrouted = append(report.Unrouted, unrouted...)
		if err := writeJSONDoc(stdout, report); err != nil {
			return &CodedError{Code: "runtime", Message: err.Error()}
		}
	} else {
		for _, r := range stale {
			fmt.Fprintf(stdout, "stale route %s:%s (%s/%s)\n", r.Provider, r.ModelID, r.Model, r.Reasoning)
		}
	}
	if len(stale) > 0 {
		return &ReportedError{Err: &CodedError{
			Code:    "stale_routes",
			Message: fmt.Sprintf("%d stale route(s); run which-model routes refresh", len(stale)),
		}}
	}
	return nil
}

// writeJSONDoc renders one indent-2 JSON document plus a trailing newline
// (F22 envelope; F27 CONTRACTS §5).
func writeJSONDoc(w io.Writer, doc any) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}
