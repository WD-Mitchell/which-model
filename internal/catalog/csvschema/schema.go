// Package csvschema owns the shared catalog column vocabulary without file or network I/O.
package csvschema

const (
	BenchmarkColumnPrefix = "benchmark:"
	ProvenancePrefix      = "# which-model-scores-provenance"
)

// RawCoreColumns is the fixed core-column order of available_model_raw_values.csv
// (pipeline spec §3.1, model_types.py:10-19). First 8 columns of every raw CSV.
var RawCoreColumns = []string{
	"model",
	"reasoning",
	"intelligence_index",
	"time_per_intelligence_index_task_seconds",
	"cost_per_intelligence_index_task_usd",
	"median_end_to_end_response_time_seconds",
	"artificial_analysis_coding_index",
	"artificial_analysis_agentic_index",
}

// CategoryScoreColumns is the fixed 12-category order of the scores CSV
// (annex-b §4.8; pipeline spec §3.2, generate_scores.py:67-80).
var CategoryScoreColumns = []string{
	"reasoning_score",
	"knowledge_score",
	"research_score",
	"planning_capability_score",
	"instruction_following_score",
	"software_engineering_score",
	"ui_visual_score",
	"agentic_tools_score",
	"finance_score",
	"evidence_capture_score",
	"security_score",
	"data_ml_score",
}

// NonNegativeRawColumns names the raw-CSV metric columns whose cells must be
// >= 0 (csv_store.py:34-38 NONNEGATIVE_RAW_COLUMNS).
var NonNegativeRawColumns = map[string]bool{
	"time_per_intelligence_index_task_seconds": true,
	"cost_per_intelligence_index_task_usd":     true,
	"median_end_to_end_response_time_seconds":  true,
}
