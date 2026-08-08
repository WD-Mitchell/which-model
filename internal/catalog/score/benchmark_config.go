package score

import (
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
)

// BenchmarkConfig is the parsed benchmarks catalog TOML
// (specs/features/F09-scoring/CONTRACTS.md §1.2).
type BenchmarkConfig struct {
	// CanonicalBenchmarks maps canonical benchmark name -> alias list
	// (incl. itself), first-occurrence order.
	CanonicalBenchmarks map[string][]string
	// EvidenceGroups lists category evidence, file order.
	EvidenceGroups []EvidenceGroup
}

// EvidenceGroup is one category's benchmark evidence list.
type EvidenceGroup struct {
	Category   string // one of the 11 grouped category names (not planning_capability)
	Benchmarks []string
}

// CategoryMinimumEvidence is the per-column evidence gate of
// generate_scores.py:66-81 (security needs 1 populated evidence; every other
// category needs 2).
var CategoryMinimumEvidence = map[string]int{
	"reasoning":             2,
	"knowledge":             2,
	"research":              2,
	"instruction_following": 2,
	"software_engineering":  2,
	"ui_visual":             2,
	"agentic_tools":         2,
	"finance":               2,
	"evidence_capture":      2,
	"security":              1,
	"data_ml":               2,
}

func benchmarkConfigError(format string, args ...any) error {
	return &Error{Code: ErrInvalidBenchmarkConfig, Message: "benchmark config: " + fmt.Sprintf(format, args...)}
}

// ParseBenchmarkConfig parses the benchmarks catalog TOML. Strict: unknown
// top-level and group keys are errors (meta.Undecoded); every listed group
// must have a table; evidence names are canonicalized via the optional
// [benchmark_aliases] table (alias display name -> canonical display name,
// whose identity.BenchmarkKey must match a listed benchmark) plus
// identity.BenchmarkAliases/BenchmarkKey; a dangling alias target is an
// unknown-name error.
func ParseBenchmarkConfig(data []byte) (*BenchmarkConfig, error) {
	var doc struct {
		Selection struct {
			Groups     []string `toml:"groups"`
			Benchmarks []string `toml:"benchmarks"`
		} `toml:"benchmark_selection"`
		Groups map[string]struct {
			Benchmarks []string `toml:"benchmarks"`
		} `toml:"benchmark_groups"`
		Aliases map[string]string `toml:"benchmark_aliases"`
	}
	md, err := toml.Decode(string(data), &doc)
	if err != nil {
		return nil, benchmarkConfigError("%v", err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, benchmarkConfigError("unknown key: %s", undecoded[0].String())
	}
	if doc.Selection.Groups == nil || doc.Groups == nil {
		return nil, benchmarkConfigError("missing benchmark_selection.groups or benchmark_groups")
	}

	// Validate that every selected group has a table.
	for _, group := range doc.Selection.Groups {
		if _, ok := doc.Groups[group]; !ok {
			return nil, benchmarkConfigError("benchmark_selection references unknown group: %s", group)
		}
	}

	// nameSet is every listed benchmark name (group tables in declared group
	// order, then the direct benchmarks array), preserving order.
	var nameSet []string
	for _, group := range doc.Selection.Groups {
		nameSet = append(nameSet, doc.Groups[group].Benchmarks...)
	}
	nameSet = append(nameSet, doc.Selection.Benchmarks...)

	// knownKeys is the dedup key of every listed benchmark; an alias target
	// must share a key with some listed name (e.g. "GDPval" aliases the
	// listed "GDPval-AA" via identity.BenchmarkKey), otherwise the name is
	// unknown.
	knownKeys := make(map[string]bool, len(nameSet))
	for _, name := range nameSet {
		knownKeys[identity.BenchmarkKey(name)] = true
	}

	// resolve applies the TOML alias table: an alias name is rewritten to
	// its target display name.
	resolve := func(name string) (string, error) {
		target, ok := doc.Aliases[name]
		if !ok {
			return name, nil
		}
		if !knownKeys[identity.BenchmarkKey(target)] {
			return "", benchmarkConfigError("unknown benchmark name: %s", target)
		}
		return target, nil
	}

	cfg := &BenchmarkConfig{
		CanonicalBenchmarks: make(map[string][]string),
	}
	for _, name := range nameSet {
		resolved, err := resolve(name)
		if err != nil {
			return nil, err
		}
		cfg.CanonicalBenchmarks[resolved] = append(cfg.CanonicalBenchmarks[resolved], resolved)
	}

	// Evidence groups in file order, canonical names in group-list order.
	for _, group := range doc.Selection.Groups {
		evidence := EvidenceGroup{Category: group}
		for _, name := range doc.Groups[group].Benchmarks {
			resolved, err := resolve(name)
			if err != nil {
				return nil, err
			}
			evidence.Benchmarks = append(evidence.Benchmarks, resolved)
		}
		cfg.EvidenceGroups = append(cfg.EvidenceGroups, evidence)
	}
	return cfg, nil
}
