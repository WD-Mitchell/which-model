package pick

// Tier1Axis names one of the 3 fixed ranking axes (annex-b §5.1).
type Tier1Axis string

const (
	AxisIntelligence Tier1Axis = "intelligence"
	AxisCost         Tier1Axis = "cost"
	AxisSpeed        Tier1Axis = "speed"
)

// Tier1ScoreColumn maps the 3 fixed ranking axes to their scores-CSV columns.
var Tier1ScoreColumn = map[Tier1Axis]string{
	AxisIntelligence: "intelligence_index_score",
	AxisCost:         "cost_per_intelligence_index_task_usd_score",
	AxisSpeed:        "median_end_to_end_response_time_seconds_score",
}

// Tier1AxisOrder is the fixed iteration order for missing_tier1 reasons.
var Tier1AxisOrder = []Tier1Axis{AxisIntelligence, AxisCost, AxisSpeed}

// CategoryNames is the canonical 12-category order (annex-b §5.1); also the
// deterministic iteration order for profile.Tier2Weights (F10 SPEC D2).
var CategoryNames = []string{
	"reasoning", "knowledge", "research", "planning_capability",
	"instruction_following", "software_engineering", "ui_visual",
	"agentic_tools", "finance", "evidence_capture", "security", "data_ml",
}
