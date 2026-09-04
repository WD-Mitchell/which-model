// F26: pick pipeline (specs/features/F26-cmd-pick/). Profile resolution,
// filtering, ranking, usage/bands join, strategy, result assembly, text
// renderer and history append live here. NO build tag: the feature runs
// degraded under -tags nousage (SPEC §2.4) with the F21 toggle stub.
package whichmodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/WD-Mitchell/which-model/internal/pick/band"
	"github.com/WD-Mitchell/which-model/internal/pick/strategy"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
)

// validProfiles are the eleven annex-c §2.1 profile names, verbatim and in
// order (SPEC §2.1.1; CONTRACTS §2 — the list is load-bearing).
var validProfiles = []string{
	"simple_implementation",
	"simple_action_execution",
	"balanced_implementation",
	"complex_implementation",
	"ui_ux",
	"complex_action_execution",
	"financial_work",
	"research",
	"planning",
	"orchestration",
	"review",
}

// validCategories are the --task-category selector values (SPEC §2.1.2).
var validCategories = []string{
	"implementation",
	"action_execution",
	"ui_ux",
	"financial_work",
	"research",
	"planning",
	"orchestration",
	"review",
}

// f20StrategyNames are the canonical F20 registry strategy strings.
var f20StrategyNames = []string{
	string(pick.StrategyPriority),
	string(pick.StrategyRoundRobin),
	string(pick.StrategyLeastUsed),
	string(pick.StrategyMostUsed),
	string(pick.StrategyClosestToReset),
}

// strategyNamesFunc is the F20 registry-name seam (SPEC §2.1.3; D-3): F26
// validates --strategy against this list at runtime and never hardcodes the
// registry. Tests inject a fake list.
var strategyNamesFunc = func() []string { return f20StrategyNames }

// resolveProfile maps PickArgs to a profile id: a validated --profile, or
// the --task-category/--complexity mapping table (SPEC §2.1.1–§2.1.2).
func resolveProfile(args PickArgs) (string, error) {
	if args.Profile != "" {
		for _, p := range validProfiles {
			if p == args.Profile {
				return p, nil
			}
		}
		return "", &UsageError{Message: fmt.Sprintf("unknown profile %q; valid: %s", args.Profile, strings.Join(validProfiles, ", "))}
	}
	cat := args.TaskCategory
	known := false
	for _, c := range validCategories {
		if c == cat {
			known = true
			break
		}
	}
	if !known {
		return "", &UsageError{Message: fmt.Sprintf("unknown task category %q", cat)}
	}
	switch cat {
	case "implementation", "action_execution":
		mapped := ""
		switch args.Complexity {
		case "simple":
			if cat == "implementation" {
				mapped = "simple_implementation"
			} else {
				mapped = "simple_action_execution"
			}
		case "medium":
			mapped = "balanced_implementation"
		case "complex":
			if cat == "implementation" {
				mapped = "complex_implementation"
			} else {
				mapped = "complex_action_execution"
			}
		default:
			return "", &UsageError{Message: fmt.Sprintf("unknown complexity %q", args.Complexity)}
		}
		return mapped, nil
	default:
		// 1:1 categories reject a non-empty --complexity (SPEC §2.1.2).
		if args.Complexity != "" {
			return "", &UsageError{Message: fmt.Sprintf("--complexity is not valid for task category %q", cat)}
		}
		return cat, nil
	}
}

// validateStrategy checks name against the F20 registry names (SPEC §2.1.3).
func validateStrategy(name string, names []string) error {
	for _, n := range names {
		if n == name {
			return nil
		}
	}
	return &UsageError{Message: fmt.Sprintf("unknown strategy %q; valid: %s", name, strings.Join(names, ", "))}
}

// isUsageRequiredStrategy reports whether selection requires live or cached
// usage metadata.
func isUsageRequiredStrategy(name string) bool {
	return name == string(pick.StrategyLeastUsed) ||
		name == string(pick.StrategyMostUsed) ||
		name == string(pick.StrategyClosestToReset)
}

// RouteRef is the JSON form of a route inside Candidate/ExcludedCandidate —
// annex-c §4.2 "$defs/Route" VERBATIM: required [provider, model_id, model,
// reasoning, window_ids], additionalProperties false (CONTRACTS §2).
type RouteRef struct {
	Provider  string   `json:"provider"`
	ModelID   string   `json:"model_id"`
	Model     string   `json:"model"`
	Reasoning string   `json:"reasoning"`
	WindowIDs []string `json:"window_ids"`
}

// Candidate mirrors annex-c §4.2 candidates[] VERBATIM (CONTRACTS §2).
type Candidate struct {
	CandidateID    string   `json:"candidate_id"`          // "<provider>:<model_id>"
	Route          RouteRef `json:"route"`                 // full Route object, not a bare id
	ModelScore     float64  `json:"model_score"`           // decimal.Round(0) → float64
	Band           string   `json:"band,omitempty"`        // omitted when usage disabled
	BandWeight     float64  `json:"band_weight,omitempty"` // omitted when usage disabled
	ProviderWeight float64  `json:"provider_weight"`
	FinalScore     float64  `json:"final_score"`
	Warnings       []string `json:"warnings"`
}

// ExcludedCandidate mirrors annex-c §4.2 excluded_candidates[] VERBATIM
// (CONTRACTS §2).
type ExcludedCandidate struct {
	Route      RouteRef `json:"route"`
	ReasonCode string   `json:"reason_code"` // band_gated|no_score_row|auth_required|provider_error|not_in_availability_list
	Reason     string   `json:"reason"`      // human-readable detail
}

// PickResult is the pick --json document root (CONTRACTS §5; SPEC D-17/D-18).
type PickResult struct {
	SchemaVersion       string              `json:"schema_version"` // "2.0"
	UsageEnabled        bool                `json:"usage_enabled"`
	UsageDisabledReason *string             `json:"usage_disabled_reason"` // null when enabled
	Profile             string              `json:"profile"`
	Strategy            string              `json:"strategy"`
	Normalizer          string              `json:"normalizer"`
	Aggregator          string              `json:"aggregator"`
	Candidates          []Candidate         `json:"candidates"`
	ExcludedCandidates  []ExcludedCandidate `json:"excluded_candidates"`

	// bandUsedPercent carries the picked candidate's band used-percent for
	// the text renderer (CONTRACTS §7); never serialized.
	bandUsedPercent map[string]float64
}

// pickFetchOptions is the F26-owned fetch seam input (CONTRACTS §8.3).
type pickFetchOptions struct {
	Backend   config.UsageBackend
	Offline   bool
	Refresh   bool
	MaxAge    time.Duration
	Timeout   time.Duration
	Providers []string
}

// bandResult is the F26-owned band evaluation result (CONTRACTS §8.4).
type bandResult struct {
	Name        string
	UsedPercent float64
	Weight      float64
	Gated       bool
	Warning     string
}

// strategyOptions is reserved for strategy-wide execution options.
type strategyOptions struct{}

// HistoryEntry is one append-only line of <state_dir>/pick/history.jsonl
// (CONTRACTS §2; SPEC D-13).
type HistoryEntry struct {
	ULID          string   `json:"ulid"` // github.com/oklog/ulid/v2, 26 chars
	TS            string   `json:"ts"`   // RFC3339
	Profile       string   `json:"profile"`
	Strategy      string   `json:"strategy"`
	CandidateID   string   `json:"candidate_id"` // "" when no pick
	FinalScore    float64  `json:"final_score"`  // 0 when no pick
	ExcludedCount int      `json:"excluded_count"`
	Evidence      Evidence `json:"evidence"` // full annex-c §4.3 record (SPEC D-13)
}

// Evidence mirrors annex-c §4.3 "$defs/Evidence" VERBATIM: required
// [profile, score_inputs, band, snapshot_age_seconds, confidence,
// route_provenance, excluded_candidates, last_verified],
// additionalProperties false. Degraded mode omits band/snapshot_age_seconds/
// confidence/last_verified (annex-c §5.1).
type Evidence struct {
	Profile            string              `json:"profile"`
	ScoreInputs        map[string]float64  `json:"score_inputs"` // tier1 + category composite values (numbers)
	Band               *BandEvidence       `json:"band,omitempty"`
	SnapshotAgeSeconds *int64              `json:"snapshot_age_seconds,omitempty"`
	Confidence         string              `json:"confidence,omitempty"`    // live|cached; omitted in degraded mode
	RouteProvenance    string              `json:"route_provenance"`        // provider_live|models_dev|user_declared
	ExcludedCandidates []ExcludedCandidate `json:"excluded_candidates"`     // full §4.2 objects
	LastVerified       string              `json:"last_verified,omitempty"` // single RFC3339 date-time
}

// BandEvidence is the evidence band object (annex-c §4.3).
type BandEvidence struct {
	Name        string  `json:"name"`
	UsedPercent float64 `json:"used_percent"`
	Weight      float64 `json:"weight"`
}

// ExplainResult is the explain --json document root — annex-c §4.3 VERBATIM:
// required [schema_version, candidate, evidence].
type ExplainResult struct {
	SchemaVersion string   `json:"schema_version"` // "2.0"
	Candidate     string   `json:"candidate"`      // candidate_id echoed back
	Evidence      Evidence `json:"evidence"`
}

// ExplainArgs selects the history record (CONTRACTS §2).
type ExplainArgs struct {
	Last       bool
	PickID     string // "" unless --pick-id
	JSON       bool   // Global.JSON
	ConfigPath string
}

// usageSnapshot is the F26 view of a provider's usage snapshot (T4). Alias
// of the F19 type so the seam can live in pick.go without a build tag.
type usageSnapshot = usage.Snapshot

// timeValue is the last-verified timestamp alias (T4).
type timeValue = time.Time

// runState is the per-run pipeline state shared by the default seam
// adapters (score rows, snapshots, evidence inputs). Single-threaded: set
// at RunPick start, restored on exit.
type runState struct {
	fetchOptions   pickFetchOptions
	cfg            *config.Config
	strategyConfig strategy.Config
	profile        string
	dataDir        string
	dryRun         bool               // feeds strategy.State.DryRun: round-robin reads the cursor without advancing (F20 CONTRACTS §6)
	scores         []catalog.ScoreRow // scores CSV, loaded once per run
	scoreInputs    map[string]map[string]float64
	routesByKey    map[string]routing.Route
	excluded       []ExcludedCandidate
	snapshots      map[string]*usageSnapshot
	lastVerified   map[string]timeValue
	usageEnabled   bool
	usageReason    string
	// pressureByProvider feeds least-used and most-used; resetAtByProvider
	// feeds closest-to-reset.
	pressureByProvider map[string]float64
	resetAtByProvider  map[string]time.Time
	// bandUsedPercent carries each surviving candidate's band used-percent
	// for the text renderer and evidence (never serialized).
	bandUsedPercent map[string]float64
}

// pickRun is the current run's state, read by the default seam adapters.
var pickRun *runState

// scoreFunc is the F10 scoring seam (SPEC §2.2a; CONTRACTS §8.7): returns
// the rounded model score, whether a score row exists, and the row's
// tier1+category composite inputs (fed to Evidence score_inputs, T9).
var scoreFunc = func(profile, model, reasoning string) (decimal.Decimal, bool, map[string]float64) {
	st := pickRun
	if st == nil || st.cfg == nil {
		return decimal.Zero, false, nil
	}
	p, ok := pick.Profiles[profile]
	if !ok {
		return decimal.Zero, false, nil
	}
	for _, row := range st.scores {
		if row.Model == model && row.Reasoning == reasoning {
			ms := pick.ScoreModel(row, p)
			return ms.Total, true, scoreInputsFor(ms)
		}
	}
	return decimal.Zero, false, nil
}

// scoreInputsFor extracts the F10 tier1+category composite inputs from a
// scored model (annex-c §4.3 score_inputs; SPEC D-13). Missing composites
// (nil pointers) map to 0.
func scoreInputsFor(ms pick.ModelScore) map[string]float64 {
	inputs := map[string]float64{"tier1": ms.Tier1.InexactFloat64(), "category": 0}
	if ms.Tier2 != nil {
		inputs["category"] = ms.Tier2.InexactFloat64()
	}
	return inputs
}

// pickFetchAllFunc is the F14 fetch seam (SPEC §2.2e; CONTRACTS §8.3).
// Returns per-provider snapshots plus the last-verified map. Named
// pickFetchAllFunc because F24's usage.go owns fetchAllFunc in the
// default build; this F26-owned seam works in both build tags.
var pickUsageFetchAll = fetch.FetchAll

var pickFetchAllFunc = func(ctx context.Context, providers []string, opts pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
	enabled := make(map[string]bool, len(providers))
	for _, p := range providers {
		enabled[p] = true
	}
	auth := config.DefaultAuthConfig()
	if st := pickRun; st != nil && st.cfg != nil {
		var err error
		auth, err = st.cfg.LoadAuth()
		if err != nil {
			return nil, nil, err
		}
	}
	snaps, _, err := pickUsageFetchAll(ctx, providers, fetch.Options{
		Backend:                opts.Backend,
		Offline:                opts.Offline,
		Refresh:                opts.Refresh,
		MaxAge:                 opts.MaxAge,
		Timeout:                opts.Timeout,
		Enabled:                enabled,
		StateDir:               stateDirFunc(),
		DisableManagedKeychain: !auth.UseKeychain,
	})
	if err != nil {
		return nil, nil, err
	}
	byProvider := make(map[string]*usageSnapshot, len(snaps))
	verified := make(map[string]timeValue, len(snaps))
	for i := range snaps {
		s := &snaps[i]
		byProvider[s.Provider] = s
		verified[s.Provider] = s.FetchedAt
	}
	return byProvider, verified, nil
}

// bandEvaluateFunc is the F19 band seam (SPEC §2.2f; CONTRACTS §8.4):
// route is the candidate's canonical route key; the default adapter
// resolves the route's window IDs from the run state.
var bandEvaluateFunc = func(snap *usage.Snapshot, route string, cfg *config.Config) (bandResult, error) {
	st := pickRun
	if st == nil {
		return bandResult{}, errors.New("no active pick run")
	}
	var raw band.TOMLConfig
	if err := cfg.UnmarshalKey("bands", &raw); err != nil {
		return bandResult{}, err
	}
	bcfg, err := band.FromTOML(raw)
	if err != nil {
		return bandResult{}, err
	}
	r, ok := st.routesByKey[route]
	if !ok {
		return bandResult{}, fmt.Errorf("no route for %s", route)
	}
	if snap == nil {
		return bandResult{}, fmt.Errorf("no usage snapshot for provider %s", r.Provider)
	}
	p := band.NewPressure(*snap, r.WindowIDs)
	res := band.EvaluateBand(p, bcfg)
	used := 0.0
	if p.Known {
		used = p.Percent.InexactFloat64()
	}
	return bandResult{Name: res.Name, UsedPercent: used, Weight: res.Weight.InexactFloat64(), Gated: res.Gated, Warning: res.Warning}, nil
}

// strategyApplyFunc is the F20 strategy seam (SPEC §2.2g; CONTRACTS §8.5):
// returns the survivors in strategy order with survivors[0] as the pick.
var strategyApplyFunc = func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
	st := pickRun
	if st == nil {
		return nil, errors.New("no active pick run")
	}
	s, err := strategy.New(pick.Strategy(name))
	if err != nil {
		return nil, err
	}
	state := &strategy.State{
		Profile:             st.profile,
		DataDir:             st.dataDir,
		DryRun:              st.dryRun,
		ProviderPriority:    providerOrder(st.cfg),
		Config:              st.strategyConfig,
		UsageEnabled:        st.usageEnabled,
		UsageDisabledReason: st.usageReason,
		PressureByProvider:  st.pressureByProvider,
		ResetAtByProvider:   st.resetAtByProvider,
	}
	pcands := make([]pick.Candidate, len(cands))
	for i, c := range cands {
		pcands[i] = pick.Candidate{
			Route:          st.routesByKey[candidateRouteKey(c)],
			ModelScore:     decimal.NewFromFloat(c.ModelScore),
			Band:           c.Band,
			BandWeight:     decimal.NewFromFloat(c.BandWeight),
			ProviderWeight: decimal.NewFromFloat(c.ProviderWeight),
			FinalScore:     decimal.NewFromFloat(c.FinalScore),
			Warnings:       c.Warnings,
		}
	}
	picked, rest, err := s.Pick(pcands, state)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(rest)+1)
	out = append(out, mapStrategyCandidate(picked, st))
	for _, p := range rest {
		out = append(out, mapStrategyCandidate(p, st))
	}
	return out, nil
}

// mapStrategyCandidate converts an internal/pick candidate back to the
// F26 JSON form (the strategy may reorder or replace candidates).
func mapStrategyCandidate(p pick.Candidate, st *runState) Candidate {
	return Candidate{
		CandidateID:    p.Route.Provider + ":" + p.Route.ModelID,
		Route:          routeRef(p.Route),
		ModelScore:     p.ModelScore.InexactFloat64(),
		Band:           p.Band,
		BandWeight:     p.BandWeight.InexactFloat64(),
		ProviderWeight: p.ProviderWeight.InexactFloat64(),
		FinalScore:     p.FinalScore.InexactFloat64(),
		Warnings:       p.Warnings,
	}
}

// candidateRouteKey is the canonical route key for a F26 candidate
// (mirrors strategy.RouteKey; "<provider>/<model_id>/<reasoning>").
func candidateRouteKey(c Candidate) string {
	return c.Route.Provider + "/" + c.Route.ModelID + "/" + c.Route.Reasoning
}

// stateDirFunc resolves the platform state directory (history lives under
// <state_dir>/pick). Injectable so tests can pin a temp dir.
var stateDirFunc = func() string { return defaultPaths().StateDir }

// pickAuthCode reports whether a fetch failure code is in F24's auth-class
// list (F24 CONTRACTS §5; copied verbatim into this build-tag-free file).
func pickAuthCode(code string) bool {
	switch code {
	case "unauthorized", "login_required", "expired_credential", "credential_file",
		"credential_json", "unsafe_credential", "access_denied", "device_expired",
		"cookie_unavailable", "signing_failed":
		return true
	}
	return false
}

// multiplyRound2 multiplies float64s as decimals and rounds to 2 places
// (global CONTRACTS §5 precision rules): FinalScore = ModelScore ×
// BandWeight × ProviderWeight (SPEC §2.2f).
func multiplyRound2(vals ...float64) float64 {
	d := decimal.NewFromFloat(vals[0])
	for _, v := range vals[1:] {
		d = d.Mul(decimal.NewFromFloat(v))
	}
	return d.Round(2).InexactFloat64()
}

// classifyNoPick maps a zero-survivor run to the SPEC §2.5 exit class
// (precedence high → low; Decision D-15): any auth_required → 5, else any
// band_gated/provider_error → 4, else 3. Messages are the SPEC §3.16
// strings (issue #50: the auth message used to hardcode a CodexBar
// credential check that is wrong for native-backend users; provider_error
// kept the shared exit-4 class per D-15 but now reports its own message so
// provider failures are not described as gating).
func classifyNoPick(ex []ExcludedCandidate) *CodedError {
	for _, x := range ex {
		if x.ReasonCode == "auth_required" {
			return &CodedError{Code: "auth_required", Message: "auth required; run which-model auth status"}
		}
	}
	for _, x := range ex {
		if x.ReasonCode == "band_gated" {
			return &CodedError{Code: "usage_gated", Message: "usage gating excluded every candidate"}
		}
	}
	for _, x := range ex {
		if x.ReasonCode == "provider_error" {
			return &CodedError{Code: "usage_gated", Message: "one or more candidates failed with a provider error"}
		}
	}
	return &CodedError{Code: "no_pick", Message: "no candidate matched the request"}
}

// applyUsageStage runs the T4 usage stage over the ranked survivors:
// fetch, per-provider failure classification, per-candidate band
// evaluation and FinalScore math. Mutates cands/excluded in place.
func applyUsageStage(cands *[]Candidate, excluded *[]ExcludedCandidate, st *runState) error {
	providers := make([]string, 0, len(*cands))
	seen := make(map[string]bool, len(*cands))
	for _, c := range *cands {
		if !seen[c.Route.Provider] {
			seen[c.Route.Provider] = true
			providers = append(providers, c.Route.Provider)
		}
	}
	snaps, lastVerified, err := pickFetchAllFunc(context.Background(), providers, st.fetchOptions)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	st.snapshots = snaps
	st.lastVerified = lastVerified
	st.resetAtByProvider = make(map[string]time.Time, len(snaps))
	for provider, snap := range snaps {
		if snap == nil {
			continue
		}
		for _, window := range snap.Windows {
			if window.ResetsAt == nil || window.ResetsAt.IsZero() {
				continue
			}
			if current, ok := st.resetAtByProvider[provider]; !ok || window.ResetsAt.Before(current) {
				st.resetAtByProvider[provider] = *window.ResetsAt
			}
		}
	}
	kept := make([]Candidate, 0, len(*cands))
	for _, cand := range *cands {
		snap, ok := snaps[cand.Route.Provider]
		if !ok {
			*excluded = append(*excluded, ExcludedCandidate{
				Route: cand.Route, ReasonCode: "provider_error",
				Reason: fmt.Sprintf("no usage snapshot for provider %s", cand.Route.Provider),
			})
			continue
		}
		if snap.Failure != nil {
			if pickAuthCode(snap.Failure.Code) {
				*excluded = append(*excluded, ExcludedCandidate{
					Route: cand.Route, ReasonCode: "auth_required",
					Reason: fmt.Sprintf("provider %s: %s", cand.Route.Provider, snap.Failure.Message),
				})
			} else {
				*excluded = append(*excluded, ExcludedCandidate{
					Route: cand.Route, ReasonCode: "provider_error", Reason: snap.Failure.Message,
				})
			}
			continue
		}
		br, err := bandEvaluateFunc(snap, candidateRouteKey(cand), st.cfg)
		if err != nil {
			*excluded = append(*excluded, ExcludedCandidate{
				Route: cand.Route, ReasonCode: "provider_error", Reason: err.Error(),
			})
			continue
		}
		if br.Gated {
			*excluded = append(*excluded, ExcludedCandidate{
				Route: cand.Route, ReasonCode: band.ReasonCodeBandGated,
				Reason: fmt.Sprintf("band usage %s%% > gate", formatPickNumber(br.UsedPercent)),
			})
			continue
		}
		cand.Band = br.Name
		cand.BandWeight = br.Weight
		cand.FinalScore = multiplyRound2(cand.ModelScore, br.Weight, cand.ProviderWeight)
		if br.Warning != "" {
			cand.Warnings = append(cand.Warnings, br.Warning)
		}
		kept = append(kept, cand)
		if st.bandUsedPercent == nil {
			st.bandUsedPercent = make(map[string]float64)
		}
		st.bandUsedPercent[cand.CandidateID] = br.UsedPercent
		if st.pressureByProvider == nil {
			st.pressureByProvider = make(map[string]float64)
		}
		if cur, ok := st.pressureByProvider[cand.Route.Provider]; !ok || br.UsedPercent > cur {
			st.pressureByProvider[cand.Route.Provider] = br.UsedPercent
		}
	}
	*cands = kept
	return nil
}

// historyPath is <state_dir>/pick/history.jsonl (CONTRACTS §2; SPEC §2.10).
func historyPath(cfg *config.Config) (string, error) {
	return filepath.Join(stateDirFunc(), "pick", "history.jsonl"), nil
}

// appendHistory records the run as one JSONL line (SPEC D-13). Write
// failures are warned and swallowed — history must never fail a pick
// (D-12).
func appendHistory(stderr io.Writer, st *runState, profile, strategy string, top *Candidate, excluded []ExcludedCandidate) {
	if st == nil || st.cfg == nil {
		return
	}
	path, err := historyPath(st.cfg)
	if err != nil {
		warnHistory(stderr, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		warnHistory(stderr, err)
		return
	}
	entry := HistoryEntry{
		ULID:          ulid.Make().String(),
		TS:            time.Now().UTC().Format(time.RFC3339),
		Profile:       profile,
		Strategy:      strategy,
		ExcludedCount: len(excluded),
		Evidence:      buildEvidence(st, top, excluded),
	}
	if top != nil {
		entry.CandidateID = top.CandidateID
		entry.FinalScore = top.FinalScore
	}
	line, err := json.Marshal(entry)
	if err != nil {
		warnHistory(stderr, err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		warnHistory(stderr, err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		warnHistory(stderr, err)
		return
	}
}

func warnHistory(stderr io.Writer, err error) {
	if stderr != nil {
		fmt.Fprintf(stderr, "warning: could not write pick history: %v\n", err)
	}
}

// routeProvenanceForEvidence maps a route provenance for the evidence
// record. Degraded mode must never emit provider_live (SPEC §2.15); a
// stale live route is recorded as models_dev in that case.
func routeProvenanceForEvidence(p routing.Provenance, usageEnabled bool) string {
	if !usageEnabled && p == routing.ProvenanceProviderLive {
		return string(routing.ProvenanceModelsDev)
	}
	return string(p)
}

// buildEvidence assembles the annex-c §4.3 record from the run state.
// top == nil marks a zero-survivor run (empty score_inputs, no band).
func buildEvidence(st *runState, top *Candidate, excluded []ExcludedCandidate) Evidence {
	ev := Evidence{
		Profile:            st.profile,
		ScoreInputs:        map[string]float64{},
		ExcludedCandidates: excluded,
	}
	if top == nil {
		return ev
	}
	if inputs := st.scoreInputs[top.CandidateID]; inputs != nil {
		ev.ScoreInputs = inputs
	}
	ev.RouteProvenance = routeProvenanceForEvidence(st.routesByKey[candidateRouteKey(*top)].Provenance, st.usageEnabled)
	if !st.usageEnabled {
		return ev
	}
	ev.Band = &BandEvidence{Name: top.Band, UsedPercent: st.bandUsedPercent[top.CandidateID], Weight: top.BandWeight}
	if snap, ok := st.snapshots[top.Route.Provider]; ok {
		age := int64(time.Since(snap.FetchedAt).Seconds())
		ev.SnapshotAgeSeconds = &age
	}
	verified, ok := st.lastVerified[top.Route.Provider]
	if ok {
		ev.Confidence = "live"
		ev.LastVerified = verified.UTC().Format(time.RFC3339)
	} else {
		ev.Confidence = "cached"
	}
	return ev
}

// loadScoreRows reads the scores CSV once per run for the default score
// seam. Any read/parse failure yields nil — the pipeline surfaces missing
// rows per-route as no_score_row exclusions instead of failing the run.
func loadScoreRows(cfg *config.Config) []catalog.ScoreRow {
	path, err := scoresCSVPath(cfg)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	rows, err := score.ParseScoresCSV(data)
	if err != nil {
		return nil
	}
	return rows
}

// routeRef builds the JSON Route object from an F18 route (SPEC D-17;
// reasoning falls back to the F18 default "default").
func routeRef(r routing.Route) RouteRef {
	reasoning := r.Reasoning
	if reasoning == "" {
		reasoning = "default"
	}
	return RouteRef{
		Provider:  r.Provider,
		ModelID:   r.ModelID,
		Model:     r.Model,
		Reasoning: reasoning,
		WindowIDs: r.WindowIDs,
	}
}

// readAllowlists unions the model ids of every --available file (one id per
// line, '#' comments, trimmed; SPEC §2.1.5). A missing file is a UsageError.
func readAllowlists(paths []string) (map[string]bool, error) {
	allow := make(map[string]bool)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, &UsageError{Message: fmt.Sprintf("allowlist file %q not found", p)}
			}
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			allow[line] = true
		}
	}
	return allow, nil
}

// providerOrder returns the config-ordered provider ids (most preferred
// first): [providers.<id>].priority descending, ties by id ascending — the
// F20 PriorityOrder semantics (F20 CONTRACTS §3).
func providerOrder(cfg *config.Config) []string {
	priorities := make(map[string]int, len(cfg.Providers))
	for id, p := range cfg.Providers {
		priorities[id] = p.Priority
	}
	return strategy.PriorityOrder(priorities)
}

// providerWeight resolves [providers.<id>].weight, default 1.0 (SPEC §2.2d).
func providerWeight(cfg *config.Config, provider string) float64 {
	p, ok := cfg.Providers[provider]
	if !ok || p.Weight.IsZero() {
		return 1.0
	}
	return p.Weight.InexactFloat64()
}

// rankCandidates sorts survivors by model_score descending; ties break by
// provider order (config order), then model_id lexical (SPEC §2.2c).
func rankCandidates(cands []Candidate, cfg *config.Config) {
	order := providerOrder(cfg)
	rank := make(map[string]int, len(order))
	for i, id := range order {
		rank[id] = i
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.ModelScore != b.ModelScore {
			return a.ModelScore > b.ModelScore
		}
		ai, aok := rank[a.Route.Provider]
		bi, bok := rank[b.Route.Provider]
		switch {
		case aok && bok:
			if ai != bi {
				return ai < bi
			}
		case aok != bok:
			return aok
		}
		return a.Route.ModelID < b.Route.ModelID
	})
}

// warnUnrouted emits the SPEC §3 warning for score rows that have no route
// (SPEC §2.2d; D-6): warning only, never an excluded_candidates entry.
func warnUnrouted(stderr io.Writer, args PickArgs, routes []routing.Route) {
	if stderr == nil {
		return
	}
	covered := make(map[string]bool, len(routes))
	for _, r := range routes {
		covered[r.Model+"/"+r.Reasoning] = true
	}
	st := pickRun
	if st == nil {
		return
	}
	path, err := scoresCSVPath(st.cfg)
	if err != nil {
		return
	}
	rows, err := readScoresFunc(path)
	if err != nil {
		return
	}
	for _, row := range rows {
		if covered[row.Model+"/"+row.Reasoning] {
			continue
		}
		if _, ok, _ := scoreFunc(args.Profile, row.Model, row.Reasoning); ok {
			fmt.Fprintf(stderr, "warning: no route for score row %s/%s; ignored\n", row.Model, row.Reasoning)
		}
	}
}

// FormatPickText renders the text result (CONTRACTS §7). Empty candidates
// render nothing — the failure line is F22's job (SPEC §2.3.9).
func FormatPickText(res *PickResult) string {
	if res == nil || len(res.Candidates) == 0 {
		return ""
	}
	top := res.Candidates[0]
	var b strings.Builder
	fmt.Fprintf(&b, "picked %s via %s (score %s)\n", top.Route.ModelID, top.Route.Provider, formatPickNumber(top.FinalScore))
	fmt.Fprintf(&b, "  profile: %s\n", res.Profile)
	fmt.Fprintf(&b, "  strategy: %s\n", res.Strategy)
	if res.UsageEnabled && top.Band != "" {
		used := res.bandUsedPercent[top.CandidateID]
		fmt.Fprintf(&b, "  band: %s (%s%% used, weight %s)\n", top.Band, formatPickNumber(used), formatPickNumber(top.BandWeight))
	}
	if len(top.Warnings) > 0 {
		fmt.Fprintf(&b, "  warnings: %d\n", len(top.Warnings))
	}
	return b.String()
}

// formatPickNumber renders numbers per CONTRACTS §7.
func formatPickNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// RunPick executes the full pick pipeline (SPEC §2.2). Returns nil on a
// pick, UsageError on argument errors, CodedError for exit classes
// 1/3/4/5/2 (F26 CONTRACTS §2, §4).
func RunPick(args PickArgs, stdout, stderr io.Writer) error {
	// Selector validation (SPEC §2.1; T1).
	switch {
	case args.Profile == "" && args.TaskCategory == "":
		return &UsageError{Message: "--profile or --task-category is required"}
	case args.Profile != "" && args.TaskCategory != "":
		return &UsageError{Message: "--profile and --task-category are mutually exclusive"}
	case (args.TaskCategory == "") != (args.Complexity == ""):
		return &UsageError{Message: "--task-category and --complexity must be given together"}
	}
	profile, err := resolveProfile(args)
	if err != nil {
		return err
	}
	if args.Strategy != "" {
		if err := validateStrategy(args.Strategy, strategyNamesFunc()); err != nil {
			return err
		}
	}
	args.Profile = profile

	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	var strategyConfig strategy.Config
	if err := cfg.UnmarshalKey("strategy", &strategyConfig); err != nil {
		return &UsageError{Message: err.Error()}
	}
	enabled, reason := toggleResolveFunc(args.NoUsage, cfg)
	if args.Strategy == "" {
		args.Strategy = strategyConfig.Default
		if args.Strategy == "" {
			if enabled {
				args.Strategy = string(pick.StrategyClosestToReset)
			} else {
				args.Strategy = string(pick.StrategyPriority)
			}
		}
		if err := validateStrategy(args.Strategy, strategyNamesFunc()); err != nil {
			return err
		}
	}
	st := &runState{cfg: cfg, strategyConfig: strategyConfig, profile: profile, dryRun: args.DryRun, scores: loadScoreRows(cfg), dataDir: stateDirFunc()}
	st.fetchOptions = pickFetchOptions{Backend: cfg.Usage.Backend, Offline: args.Offline, Refresh: args.Refresh, MaxAge: args.MaxAge, Timeout: args.Timeout}
	prev := pickRun
	pickRun = st
	defer func() { pickRun = prev }()

	routesPath, err := routesPathFunc(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	table, err := loadRoutesFunc(routesPath)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	routes := table.Routes
	st.routesByKey = make(map[string]routing.Route, len(routes))
	for _, r := range routes {
		st.routesByKey[strategy.RouteKeyFromRoute(r)] = r
	}
	allow, err := readAllowlists(args.Allowlists)
	if err != nil {
		if _, ok := err.(*UsageError); ok {
			return err
		}
		return &CodedError{Code: "runtime", Message: err.Error()}
	}

	excluded := make([]ExcludedCandidate, 0)
	cands := make([]Candidate, 0)
	for _, r := range routes {
		ref := routeRef(r)
		if len(args.Allowlists) > 0 && !allow[r.ModelID] {
			excluded = append(excluded, ExcludedCandidate{Route: ref, ReasonCode: "not_in_availability_list", Reason: "model not in allowlist"})
			continue
		}
		scoreVal, ok, inputs := scoreFunc(profile, r.Model, r.Reasoning)
		if !ok {
			excluded = append(excluded, ExcludedCandidate{
				Route:      ref,
				ReasonCode: "no_score_row",
				Reason:     fmt.Sprintf("no score row for %s/%s", r.Model, r.Reasoning),
			})
			if stderr != nil {
				fmt.Fprintf(stderr, "warning: no score row for %s/%s; excluded\n", r.Model, r.Reasoning)
			}
			continue
		}
		modelScore := scoreVal.Round(0).InexactFloat64()
		cand := Candidate{
			CandidateID: r.Provider + ":" + r.ModelID,
			Route:       ref,
			ModelScore:  modelScore,
			FinalScore:  modelScore,
			Warnings:    []string{},
		}
		cands = append(cands, cand)
		if st.scoreInputs == nil {
			st.scoreInputs = make(map[string]map[string]float64)
		}
		st.scoreInputs[cand.CandidateID] = inputs
	}
	warnUnrouted(stderr, args, routes)
	rankCandidates(cands, cfg)
	for i := range cands {
		cands[i].ProviderWeight = providerWeight(cfg, cands[i].Route.Provider)
	}
	if len(cands) == 0 {
		appendHistory(stderr, st, profile, args.Strategy, nil, excluded)
		return classifyNoPick(excluded)
	}

	st.usageEnabled = enabled
	st.usageReason = reason
	if !enabled {
		// Strict no_providers misconfiguration: usage explicitly on but no
		// provider enabled — surface, never silently degrade (SPEC §2.14).
		if reason == "no_providers_enabled" && st.cfg.Usage.Enabled == config.UsageTrue {
			return &CodedError{Code: "usage_config", Message: `usage is enabled but no providers are enabled; set [providers.<id>] enabled = true or [usage] enabled = "auto"`}
		}
		// Usage-aware strategies need usage data and never fall back.
		if isUsageRequiredStrategy(args.Strategy) {
			return &CodedError{Code: "usage_disabled", Message: fmt.Sprintf("strategy %q requires usage data", args.Strategy)}
		}
	} else if err := applyUsageStage(&cands, &excluded, st); err != nil {
		return err
	}
	if len(cands) == 0 {
		if args.JSON {
			res := &PickResult{
				SchemaVersion:      "2.0",
				UsageEnabled:       enabled,
				Profile:            profile,
				Strategy:           args.Strategy,
				Normalizer:         Global.Normalizer,
				Aggregator:         Global.Aggregator,
				Candidates:         make([]Candidate, 0),
				ExcludedCandidates: excluded,
				bandUsedPercent:    st.bandUsedPercent,
			}
			if err := emitPick(res, true, stdout); err != nil {
				return &CodedError{Code: "runtime", Message: err.Error()}
			}
		}
		appendHistory(stderr, st, profile, args.Strategy, nil, excluded)
		noPickErr := classifyNoPick(excluded)
		if args.JSON {
			return &ReportedError{Err: noPickErr}
		}
		return noPickErr
	}

	survivors, err := strategyApplyFunc(args.Strategy, cands, strategyOptions{})
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	if len(survivors) == 0 {
		appendHistory(stderr, st, profile, args.Strategy, nil, excluded)
		return classifyNoPick(excluded)
	}
	cands = survivors

	res := &PickResult{
		SchemaVersion:      "2.0",
		UsageEnabled:       enabled,
		Profile:            profile,
		Strategy:           args.Strategy,
		Normalizer:         Global.Normalizer,
		Aggregator:         Global.Aggregator,
		Candidates:         cands,
		ExcludedCandidates: excluded,
		bandUsedPercent:    st.bandUsedPercent,
	}
	if !enabled {
		r := reason
		res.UsageDisabledReason = &r
	}
	if err := emitPick(res, args.JSON, stdout); err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	appendHistory(stderr, st, profile, args.Strategy, &cands[0], excluded)
	return nil
}

// emitPick renders the result document: JSON (indent 2 + newline) or text.
func emitPick(res *PickResult, jsonOut bool, stdout io.Writer) error {
	if stdout == nil {
		return nil
	}
	if jsonOut {
		return writeJSONDoc(stdout, res)
	}
	_, err := io.WriteString(stdout, FormatPickText(res))
	return err
}
