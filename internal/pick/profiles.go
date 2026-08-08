package pick

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
)

func d(n int64) decimal.Decimal { return decimal.NewFromInt(n) }

// mustProfile builds a catalog.Profile from literal integer weights and
// panics when ValidateProfile fails, mirroring the Python import-time crash
// for an invalid built-in profile (annex-b §5.1).
func mustProfile(name string, tier1Share, tier2Share int64, tier1 map[string]int64, tier2 map[string]int64) catalog.Profile {
	p := catalog.Profile{
		Name:         name,
		Tier1Share:   d(tier1Share),
		Tier2Share:   d(tier2Share),
		Tier1Weights: map[string]decimal.Decimal{},
		Tier2Weights: map[string]decimal.Decimal{},
	}
	for k, v := range tier1 {
		p.Tier1Weights[k] = d(v)
	}
	for k, v := range tier2 {
		p.Tier2Weights[k] = d(v)
	}
	if err := ValidateProfile(p); err != nil {
		panic(fmt.Sprintf("built-in profile %q is invalid: %v", name, err))
	}
	return p
}

// Profiles holds the 11 built-ins, constructed via mustProfile which panics
// on ValidateProfile failure (annex-b §5.1, mirrors Python import-time crash).
var Profiles = map[string]catalog.Profile{
	"simple_implementation": mustProfile("simple_implementation", 80, 20,
		map[string]int64{"intelligence": 1, "cost": 5, "speed": 5},
		map[string]int64{"instruction_following": 5}),

	"simple_action_execution": mustProfile("simple_action_execution", 65, 35,
		map[string]int64{"intelligence": 1, "cost": 5, "speed": 5},
		map[string]int64{"instruction_following": 5, "evidence_capture": 5, "agentic_tools": 3, "software_engineering": 2}),

	"balanced_implementation": mustProfile("balanced_implementation", 70, 30,
		map[string]int64{"intelligence": 3, "cost": 3, "speed": 3},
		map[string]int64{"software_engineering": 5, "instruction_following": 3, "agentic_tools": 2}),

	"complex_implementation": mustProfile("complex_implementation", 60, 40,
		map[string]int64{"intelligence": 5, "cost": 1, "speed": 1},
		map[string]int64{"software_engineering": 5, "planning_capability": 4, "instruction_following": 2}),

	"ui_ux": mustProfile("ui_ux", 60, 40,
		map[string]int64{"intelligence": 3, "cost": 2, "speed": 3},
		map[string]int64{"ui_visual": 5, "software_engineering": 4, "instruction_following": 3, "evidence_capture": 2}),

	"complex_action_execution": mustProfile("complex_action_execution", 60, 40,
		map[string]int64{"intelligence": 4, "cost": 2, "speed": 2},
		map[string]int64{"agentic_tools": 5, "instruction_following": 4, "evidence_capture": 2}),

	"financial_work": mustProfile("financial_work", 60, 40,
		map[string]int64{"intelligence": 5, "cost": 1, "speed": 2},
		map[string]int64{"finance": 5, "knowledge": 4, "reasoning": 4, "research": 3, "instruction_following": 3}),

	"research": mustProfile("research", 60, 40,
		map[string]int64{"intelligence": 4, "cost": 2, "speed": 2},
		map[string]int64{"research": 5, "knowledge": 4, "reasoning": 3, "instruction_following": 2, "agentic_tools": 2}),

	"planning": mustProfile("planning", 60, 40,
		map[string]int64{"intelligence": 5, "cost": 1, "speed": 1},
		map[string]int64{"planning_capability": 5}),

	"orchestration": mustProfile("orchestration", 60, 40,
		map[string]int64{"intelligence": 5, "cost": 5, "speed": 4},
		map[string]int64{"planning_capability": 5, "instruction_following": 5}),

	"review": mustProfile("review", 65, 35,
		map[string]int64{"intelligence": 4, "cost": 3, "speed": 2},
		map[string]int64{"instruction_following": 5, "software_engineering": 4, "reasoning": 4, "security": 3, "evidence_capture": 2}),
}
