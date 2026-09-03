package routing

import (
	"fmt"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// ModelEntry is one provider-native model, from the models.dev catalogue
// (F08) or a live provider enumeration.
type ModelEntry struct {
	ModelID   string   // provider-native id, e.g. "claude-opus-4-5-20251101"
	Name      string   // display name BEFORE cleaning (F07 CleanModelName applied by routing)
	Reasoning []string // declared effort levels (models.dev reasoning_options[].values); empty = non-reasoning model
}

// UserDeclaredRoute is a hand-authored route (routes.toml or `routes add`).
// Model/Reasoning are trusted operator input; no catalog match is required.
type UserDeclaredRoute struct {
	Provider  string
	ModelID   string
	Model     string   // catalog display name, already cleaned
	Reasoning string   // may be "default"
	WindowIDs []string // explicit gating windows; empty = derive via BindWindowIDs
}

type ProviderInput struct {
	Provider         string
	Kind             usage.Kind   // from the F11 descriptor
	ModelsDev        []ModelEntry // models.dev catalogue records (F08)
	LiveModels       []ModelEntry // live enumeration; nil = no credentialed enumeration exists for this provider
	UserDeclared     []UserDeclaredRoute
	ExcludedModelIDs []string           // providers.toml excluded_models
	Windows          []usage.WindowSpec // descriptor windows (F11) for BindWindowIDs
}

type Input struct {
	Providers   []ProviderInput
	CatalogRows []identity.Identity // every scores-CSV identity (Model already cleaned, Reasoning already collapsed)
	Degraded    bool                // usage disabled at any level: live source skipped, one warning (SPEC §2.13)
}

// UnroutedModel is one provider-native model (or level) skipped because no
// catalog row matched. Surfaced, never silent (SPEC §2.7).
type UnroutedModel struct {
	Provider  string
	ModelID   string
	Name      string // cleaned catalog name
	Reasoning string // level that failed; "" when the whole model failed
	Reason    string // always "no_catalog_row"
}

type BuildResult struct {
	Routes   []Route
	Unrouted []UnroutedModel
	Warnings []string         // exact strings below
	Errors   map[string]error // provider id -> hard error; nil when no provider errored
}

// AmbiguityError is the fail-loud result of an unresolvable catalog match
// (SPEC §2.8). Candidates lists EVERY matched catalog identity, catalog order.
type AmbiguityError struct {
	Provider   string
	ModelID    string
	Name       string // cleaned catalog name that matched
	Candidates []identity.Identity
}

type joinedLevel struct {
	level string
	row   identity.Identity
}

// joinModel classifies one provider-native model against the catalog.
func joinModel(entry ModelEntry, rows []identity.Identity) (levels []joinedLevel, candidates []identity.Identity, unmatched []string) {
	declared := entry.Reasoning
	if len(declared) == 0 {
		declared = []string{"default"}
	}
	clean := identity.CleanModelName(entry.Name)
	for _, row := range rows {
		if identity.CleanModelName(row.Model) == clean {
			candidates = append(candidates, row)
		}
	}
	for _, level := range declared {
		collapsed := identity.CollapseReasoning(level)
		matched := false
		for _, row := range candidates {
			if identity.CollapseReasoning(row.Reasoning) == collapsed {
				levels = append(levels, joinedLevel{level: level, row: row})
				matched = true
				break
			}
		}
		if !matched {
			unmatched = append(unmatched, level)
		}
	}
	// An entry that declares NO effort levels binds a single catalog identity
	// under that identity's own reasoning: with one candidate and no claim from
	// the source, there is nothing to disambiguate. Without this, a models.dev
	// record with no declared efforts (treated as "default" → collapsed "high")
	// could not bind a model whose only catalog row is e.g. xhigh, and that one
	// model failed the WHOLE provider's refresh via AmbiguityError (measured
	// live: copilot/gpt-5.4-nano vs the sole identity (GPT-5.4 nano, xhigh)).
	// Entries that DID declare levels keep the strict behaviour (a declared
	// level absent from the catalog stays unmatched), and the fail-loud
	// AmbiguityError is unchanged for its designed case — multiple candidates
	// the declared levels cannot tell apart (F18 golden: three Opus rows).
	if len(entry.Reasoning) == 0 && len(levels) == 0 && len(candidates) == 1 {
		levels = append(levels, joinedLevel{level: candidates[0].Reasoning, row: candidates[0]})
		unmatched = nil
	}
	return levels, candidates, unmatched
}

func (e *AmbiguityError) Error() string {
	candidates := make([]string, len(e.Candidates))
	for i, candidate := range e.Candidates {
		candidates[i] = fmt.Sprintf("(%s, %s)", candidate.Model, candidate.Reasoning)
	}
	return fmt.Sprintf(
		"ambiguous route for %s/%s: %s matches catalog identities [%s] that declared effort levels cannot disambiguate; add a manual override in routes.toml",
		e.Provider,
		e.ModelID,
		e.Name,
		strings.Join(candidates, ", "),
	)
}

// ProduceRoutes derives the route table for every configured provider.
func ProduceRoutes(in Input) (BuildResult, error) {
	result := BuildResult{}
	if in.Degraded && len(in.Providers) > 0 {
		result.Warnings = append(result.Warnings, "live provider model lists unavailable; routes built from models-dev and user-declared sources only")
	}

	var firstErr error
	for _, provider := range in.Providers {
		declaredIDs := make(map[string]struct{}, len(provider.UserDeclared))
		for _, declared := range provider.UserDeclared {
			if _, ok := declaredIDs[declared.ModelID]; !ok {
				declaredIDs[declared.ModelID] = struct{}{}
			}
		}

		declaredRoutes := make([]Route, 0, len(provider.UserDeclared))
		declaredSeen := make(map[string]struct{}, len(provider.UserDeclared))

		excluded := make(map[string]struct{}, len(provider.ExcludedModelIDs))
		for _, modelID := range provider.ExcludedModelIDs {
			excluded[modelID] = struct{}{}
		}

		type sourceEntry struct {
			entry ModelEntry
			src   Provenance
		}
		seen := make(map[string]sourceEntry)
		order := make([]string, 0, len(provider.ModelsDev)+len(provider.LiveModels))
		providerUnrouted := make([]UnroutedModel, 0)
		eligible := provider.Kind == usage.KindSubscription || provider.Kind == usage.KindAPIKeyBilling
		if eligible {
			for _, entry := range provider.ModelsDev {
				if _, ok := excluded[entry.ModelID]; ok {
					continue
				}
				if _, ok := declaredIDs[entry.ModelID]; ok {
					continue
				}
				if _, ok := seen[entry.ModelID]; ok {
					continue
				}
				seen[entry.ModelID] = sourceEntry{entry: entry, src: ProvenanceModelsDev}
				order = append(order, entry.ModelID)
			}
			if !in.Degraded {
				for _, entry := range provider.LiveModels {
					if _, ok := excluded[entry.ModelID]; ok {
						continue
					}
					if _, ok := declaredIDs[entry.ModelID]; ok {
						continue
					}
					if existing, ok := seen[entry.ModelID]; ok {
						if existing.src == ProvenanceModelsDev {
							seen[entry.ModelID] = sourceEntry{entry: entry, src: ProvenanceProviderLive}
						}
						continue
					}
					seen[entry.ModelID] = sourceEntry{entry: entry, src: ProvenanceProviderLive}
					order = append(order, entry.ModelID)
				}
			} else {
				// Degraded: the live source is skipped entirely — no live-derived
				// routes, no live-occupied positions. A model present ONLY in
				// LiveModels falls through to the absent case: unrouted with a
				// warning, never an ambiguity check (SPEC §2.13, D-table row
				// "Degraded-source marking"; F18-T8).
				for _, entry := range provider.LiveModels {
					if _, ok := excluded[entry.ModelID]; ok {
						continue
					}
					if _, ok := declaredIDs[entry.ModelID]; ok {
						continue
					}
					if _, ok := seen[entry.ModelID]; ok {
						continue
					}
					clean := identity.CleanModelName(entry.Name)
					providerUnrouted = append(providerUnrouted, UnroutedModel{
						Provider: provider.Provider,
						ModelID:  entry.ModelID,
						Name:     clean,
						Reason:   "no_catalog_row",
					})
					result.Warnings = append(result.Warnings, fmt.Sprintf(
						"unrouted provider model %s/%s (%s): no catalog row matches",
						provider.Provider, entry.ModelID, clean,
					))
				}
			}
		}

		providerErr := error(nil)
		autoRoutes := make([]Route, 0, len(order))
		for _, modelID := range order {
			source := seen[modelID]
			levels, candidates, unmatched := joinModel(source.entry, in.CatalogRows)
			clean := identity.CleanModelName(source.entry.Name)
			if len(candidates) == 0 {
				providerUnrouted = append(providerUnrouted, UnroutedModel{
					Provider: provider.Provider,
					ModelID:  modelID,
					Name:     clean,
					Reason:   "no_catalog_row",
				})
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"unrouted provider model %s/%s (%s): no catalog row matches",
					provider.Provider, modelID, clean,
				))
				continue
			}
			// Explicit effort levels disambiguate the identities the provider
			// actually serves; score rows for other efforts are not candidates
			// for that provider-native model. Effort-less entries retain the
			// fail-loud all-candidates rule because the source cannot choose.
			if len(source.entry.Reasoning) == 0 && !coversAllCandidates(levels, candidates) {
				providerErr = &AmbiguityError{
					Provider:   provider.Provider,
					ModelID:    modelID,
					Name:       clean,
					Candidates: candidates,
				}
				break
			}
			for _, level := range levels {
				autoRoutes = append(autoRoutes, Route{
					Provider:   provider.Provider,
					ModelID:    modelID,
					Model:      clean,
					Reasoning:  level.level,
					WindowIDs:  BindWindowIDs(provider.Windows, modelID, clean),
					Provenance: source.src,
				})
			}
			for _, level := range unmatched {
				providerUnrouted = append(providerUnrouted, UnroutedModel{
					Provider:  provider.Provider,
					ModelID:   modelID,
					Name:      clean,
					Reasoning: level,
					Reason:    "no_catalog_row",
				})
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"unrouted provider model %s/%s (%s, %s): no catalog row matches",
					provider.Provider, modelID, clean, level,
				))
			}
		}

		for _, declared := range provider.UserDeclared {
			if _, ok := declaredSeen[declared.ModelID]; ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"duplicate user-declared route for %s/%s; keeping first",
					provider.Provider, declared.ModelID,
				))
				continue
			}
			declaredSeen[declared.ModelID] = struct{}{}
			windows := declared.WindowIDs
			if len(windows) == 0 {
				windows = BindWindowIDs(provider.Windows, declared.ModelID, declared.Model)
			}
			declaredRoutes = append(declaredRoutes, Route{
				Provider:   provider.Provider,
				ModelID:    declared.ModelID,
				Model:      declared.Model,
				Reasoning:  declared.Reasoning,
				WindowIDs:  windows,
				Provenance: ProvenanceUserDeclared,
			})
		}

		if providerErr != nil {
			if result.Errors == nil {
				result.Errors = make(map[string]error)
			}
			result.Errors[provider.Provider] = providerErr
			if firstErr == nil {
				firstErr = providerErr
			}
		} else {
			result.Routes = append(result.Routes, autoRoutes...)
		}
		result.Routes = append(result.Routes, declaredRoutes...)
		result.Unrouted = append(result.Unrouted, providerUnrouted...)
	}
	return result, firstErr
}

func coversAllCandidates(levels []joinedLevel, candidates []identity.Identity) bool {
	covered := make([]bool, len(candidates))
	for _, level := range levels {
		for i, candidate := range candidates {
			if !covered[i] && candidate == level.row {
				covered[i] = true
				break
			}
		}
	}
	for _, value := range covered {
		if !value {
			return false
		}
	}
	return true
}
