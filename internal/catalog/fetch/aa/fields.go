package aa

// AABenchmarkField names an evaluations.* field mapped to a generated
// benchmark column. Fields and column names are VERBATIM from
// docs/plan/annex-b-catalog-port.md §2.4.
type AABenchmarkField struct {
	Field  string // evaluations.<field> JSON key
	Column string // generated raw-CSV column name (benchmark:<name>)
}

// AABenchmarkFields is the verbatim annex-b §2.4 list. When multiple entries
// map to the same Column (the three tau*_banking variants -> τ3 Banking),
// the highest converted value wins per (model, Column).
var AABenchmarkFields = []AABenchmarkField{
	{Field: "artificial_analysis_coding_index", Column: "benchmark:Artificial Analysis Coding Index"},
	{Field: "artificial_analysis_agentic_index", Column: "benchmark:Artificial Analysis Coding Agent Index"},
	{Field: "tau_banking", Column: "benchmark:τ3 Banking"},
	{Field: "tau3_banking", Column: "benchmark:τ3 Banking"},
	{Field: "tau2_banking", Column: "benchmark:τ3 Banking"},
	{Field: "terminalbench_v2_1", Column: "benchmark:Terminal-Bench"},
	{Field: "terminalbench_hard", Column: "benchmark:Terminal-Bench Hard"},
	{Field: "scicode", Column: "benchmark:SciCode"},
	{Field: "ifbench", Column: "benchmark:IFBench"},
	{Field: "ifeval", Column: "benchmark:IFEval"},
	{Field: "hle", Column: "benchmark:Humanity's Last Exam"},
	{Field: "gpqa_diamond", Column: "benchmark:GPQA Diamond"},
	{Field: "mmmu_pro", Column: "benchmark:MMMU Pro"},
	{Field: "gdpval_aa_normalized", Column: "benchmark:GDPval-AA"},
}
