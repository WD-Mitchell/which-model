package score

import (
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
)

func TestParseBenchmarkConfigGolden(t *testing.T) {
	cfg, err := ParseBenchmarkConfig(goldenBenchmarksTOML())
	if err != nil {
		t.Fatalf("ParseBenchmarkConfig: %v", err)
	}
	if len(cfg.EvidenceGroups) != 2 {
		t.Fatalf("EvidenceGroups = %d, want 2", len(cfg.EvidenceGroups))
	}
	se := cfg.EvidenceGroups[0]
	if se.Category != "software_engineering" {
		t.Errorf("group[0].Category = %q, want software_engineering", se.Category)
	}
	if len(se.Benchmarks) != 11 {
		t.Errorf("group[0] has %d benchmarks, want 11", len(se.Benchmarks))
	}
	if se.Benchmarks[0] != "SWE-Bench Verified" || se.Benchmarks[10] != "Toolathlon" {
		t.Errorf("group[0] order wrong: %v", se.Benchmarks)
	}
	finance := cfg.EvidenceGroups[1]
	if finance.Category != "finance" {
		t.Errorf("group[1].Category = %q, want finance", finance.Category)
	}
	if len(finance.Benchmarks) != 5 {
		t.Errorf("group[1] has %d benchmarks, want 5", len(finance.Benchmarks))
	}
	if finance.Benchmarks[0] != "Finance Agent" || finance.Benchmarks[4] != "GDPval-AA" {
		t.Errorf("group[1] order wrong: %v", finance.Benchmarks)
	}
	// No aliases in this fixture: every listed name maps to itself.
	if len(cfg.CanonicalBenchmarks) != 16 {
		t.Errorf("CanonicalBenchmarks has %d entries, want 16", len(cfg.CanonicalBenchmarks))
	}
	for _, group := range cfg.EvidenceGroups {
		for _, name := range group.Benchmarks {
			aliases := cfg.CanonicalBenchmarks[name]
			if len(aliases) != 1 || aliases[0] != name {
				t.Errorf("CanonicalBenchmarks[%q] = %v, want [%q]", name, aliases, name)
			}
		}
	}
	if CategoryMinimumEvidence["security"] != 1 || CategoryMinimumEvidence["software_engineering"] != 2 {
		t.Errorf("CategoryMinimumEvidence = %v", CategoryMinimumEvidence)
	}
}

func TestParseBenchmarkConfigAliasCanonicalization(t *testing.T) {
	cfg, err := ParseBenchmarkConfig([]byte(`[benchmark_selection]
groups = ["finance"]
benchmarks = []

[benchmark_groups.finance]
benchmarks = ["FinanceAgent", "GDPval-AA"]

[benchmark_aliases]
"GDPval-AA" = "GDPval"
`))
	if err != nil {
		t.Fatalf("ParseBenchmarkConfig: %v", err)
	}
	if len(cfg.EvidenceGroups) != 1 {
		t.Fatalf("EvidenceGroups = %d, want 1", len(cfg.EvidenceGroups))
	}
	got := cfg.EvidenceGroups[0].Benchmarks
	if len(got) != 2 || got[0] != "FinanceAgent" || got[1] != "GDPval" {
		t.Errorf("group benchmarks = %v, want [FinanceAgent GDPval] (alias rewritten, order preserved)", got)
	}
	// Both resolve through identity.BenchmarkKey.
	if identity.BenchmarkKey("FinanceAgent") != identity.BenchmarkKey("FinanceAgent") {
		t.Error("FinanceAgent key mismatch")
	}
	if identity.BenchmarkKey("GDPval-AA") != identity.BenchmarkKey("GDPval") || identity.BenchmarkKey("GDPval") != "gdpval" {
		t.Errorf("GDPval-AA key = %q, want gdpval", identity.BenchmarkKey("GDPval-AA"))
	}
	canon := cfg.CanonicalBenchmarks["GDPval"]
	if len(canon) != 1 || canon[0] != "GDPval" {
		t.Errorf("CanonicalBenchmarks[gdpval] = %v, want [GDPval]", canon)
	}
}

func TestParseBenchmarkConfigStrictness(t *testing.T) {
	valid := `[benchmark_selection]
groups = ["software_engineering"]
benchmarks = []

[benchmark_groups.software_engineering]
benchmarks = ["SWE-Bench Verified"]
`
	tests := []struct {
		name string
		toml string
	}{
		{
			name: "unknown top-level key",
			toml: "foo = 1\n" + valid,
		},
		{
			name: "unknown key inside group table",
			toml: `[benchmark_selection]
groups = ["software_engineering"]
benchmarks = []

[benchmark_groups.software_engineering]
benchmarks = ["SWE-Bench Verified"]
foo = "bar"
`,
		},
		{
			name: "group lists unknown benchmark name",
			toml: `[benchmark_selection]
groups = ["finance"]
benchmarks = []

[benchmark_groups.finance]
benchmarks = ["GDPval-AA"]

[benchmark_aliases]
"GDPval-AA" = "Phantom"
`,
		},
		{
			name: "group listed without table",
			toml: `[benchmark_selection]
groups = ["software_engineering"]
benchmarks = []
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBenchmarkConfig([]byte(tt.toml))
			if err == nil {
				t.Fatal("ParseBenchmarkConfig: nil error")
			}
			var se *Error
			if !errors.As(err, &se) {
				t.Fatalf("error = %v, want *Error", err)
			}
			if se.Code != ErrInvalidBenchmarkConfig {
				t.Errorf("Code = %v, want ErrInvalidBenchmarkConfig", se.Code)
			}
			if se.Message == "" || !strings.HasPrefix(se.Message, "benchmark config:") {
				t.Errorf("Message = %q, want prefix %q", se.Message, "benchmark config:")
			}
		})
	}
}
