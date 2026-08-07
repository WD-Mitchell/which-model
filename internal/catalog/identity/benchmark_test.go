package identity

import "testing"

func TestBenchmarkKey(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"finance agent spaced", "Finance Agent", "financeagent"},
		{"finance agent camel", "FinanceAgent", "financeagent"},
		{"gdpval", "GDPval", "gdpval"},
		{"gdpval dash aa", "GDPval-AA", "gdpval"},
		{"gdpvalaa alias collapse", "GDPvalAA", "gdpval"},
		{"u2019 apostrophe", "Humanity’s Last Exam", "humanityslastexam"},
		{"ascii apostrophe", "Humanity's Last Exam", "humanityslastexam"},
		{"swe-bench verified", "SWE-Bench Verified", "swebenchverified"},
		{"swe-bench case-insensitive", "SWE-bench Verified", "swebenchverified"},
		{"terminal-bench", "Terminal-Bench", "terminalbench"},
		{"aa coding index", "Artificial Analysis Coding Index", "artificialanalysiscodingindex"},
		{"punctuation dropped", "GPT-5.6", "gpt56"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BenchmarkKey(tc.input); got != tc.want {
				t.Errorf("BenchmarkKey(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestBenchmarkAliasesVerbatim(t *testing.T) {
	if len(BenchmarkAliases) != 6 {
		t.Fatalf("len(BenchmarkAliases) = %d, want 6", len(BenchmarkAliases))
	}
	want := map[string]string{
		"financeagent":                       "financeagent",
		"gdpval":                             "gdpval",
		"gdpvalaa":                           "gdpval",
		"humanityslastexam":                  "humanityslastexam",
		"artificialanalysiscodingindex":      "artificialanalysiscodingindex",
		"artificialanalysiscodingagentindex": "artificialanalysiscodingagentindex",
	}
	for k, v := range want {
		got, ok := BenchmarkAliases[k]
		if !ok {
			t.Errorf("BenchmarkAliases missing key %q", k)
			continue
		}
		if got != v {
			t.Errorf("BenchmarkAliases[%q] = %q, want %q", k, got, v)
		}
	}
}
