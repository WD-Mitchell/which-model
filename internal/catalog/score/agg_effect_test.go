package score

import (
	"strings"
	"testing"

	sdecimal "github.com/shopspring/decimal"
)

// doublingAggregator doubles the mean — a custom aggregator must change the
// derived bytes, not merely the provenance label (issue #45 review).
type doublingAggregator struct{}

func (doublingAggregator) Aggregate(values, weights []sdecimal.Decimal) (sdecimal.Decimal, bool) {
	if len(values) == 0 {
		return sdecimal.Zero, false
	}
	sum := sdecimal.Zero
	for _, v := range values {
		sum = sum.Add(v)
	}
	return sum.Div(sdecimal.NewFromInt(int64(len(values)))).Mul(sdecimal.NewFromInt(2)).Round(0), true
}
func (doublingAggregator) Name() string { return "test-doubling" }

func TestDeriveCustomAggregatorChangesScores(t *testing.T) {
	raw := readFixture(t, "raw_golden.csv")
	bench := readFixture(t, "benchmarks_golden.toml")
	def, err := Derive(raw, bench, DefaultNormalizer(), DefaultAggregator())
	if err != nil {
		t.Fatalf("Derive default: %v", err)
	}
	got, err := Derive(raw, bench, DefaultNormalizer(), doublingAggregator{})
	if err != nil {
		t.Fatalf("Derive custom: %v", err)
	}
	if string(def) == string(got) {
		t.Fatal("custom aggregator produced identical bytes to default — aggregator not used")
	}
	if !strings.Contains(strings.SplitN(string(got), "\n", 2)[0], "aggregator=test-doubling") {
		t.Error("provenance does not name the custom aggregator")
	}
}
