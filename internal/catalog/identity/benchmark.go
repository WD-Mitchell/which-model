package identity

import (
	"strings"
	"unicode"
)

// BenchmarkAliases is the verbatim Python alias dict (SPEC §2.5,
// generate_scores.py:122-129). Only "gdpvalaa" → "gdpval" is an effective
// collapse; the identity entries are kept verbatim for parity with the source.
var BenchmarkAliases = map[string]string{
	"financeagent":                       "financeagent",
	"gdpval":                             "gdpval",
	"gdpvalaa":                           "gdpval",
	"humanityslastexam":                  "humanityslastexam",
	"artificialanalysiscodingindex":      "artificialanalysiscodingindex",
	"artificialanalysiscodingagentindex": "artificialanalysiscodingagentindex",
}

// BenchmarkKey returns the stable deduplication key for a benchmark name
// (SPEC.md §2.5, verbatim port of _benchmark_key, generate_scores.py:117-133).
// Total: never errors.
func BenchmarkKey(name string) string {
	s := strings.ToLower(name) // Python casefold; ASCII-equivalent (SPEC.md §4).
	s = strings.ReplaceAll(s, "\u2019", "'")
	s = strings.ReplaceAll(s, "`", "'")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) { // Python str.isalnum
			b.WriteRune(r)
		}
	}
	compact := b.String()
	if alias, ok := BenchmarkAliases[compact]; ok {
		return alias
	}
	return compact
}
