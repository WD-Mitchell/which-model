package identity

import "testing"

func TestAnnotatedNameIdentityDedup(t *testing.T) {
	a := IdentityKey("Claude Opus 4.5 [claude-opus-4-5-20251101]", "default")
	b := IdentityKey("Claude Opus 4.5 (latest)", "high")
	want := Identity{Model: "Claude Opus 4.5", Reasoning: "high"}
	if a != want || b != want {
		t.Fatalf("annotated spellings produced %v and %v, want %v", a, b, want)
	}
	counts := map[Identity]int{}
	counts[a]++
	counts[b]++
	if len(counts) != 1 {
		t.Errorf("map-key dedup: got %d distinct keys, want 1", len(counts))
	}
}

func TestDefaultHighEquality(t *testing.T) {
	d := IdentityKey("Example", "default")
	h := IdentityKey("Example", "high")
	if d != h {
		t.Fatalf("default and high rows must share one identity: %v != %v", d, h)
	}
	merged := map[Identity]string{}
	merged[d] = "default-row"
	merged[h] = "high-row"
	if len(merged) != 1 {
		t.Fatalf("merged map has %d entries, want 1", len(merged))
	}
	if merged[h] != "high-row" || d.Reasoning != "high" {
		t.Errorf("merged identity reasoning = %q, want high", d.Reasoning)
	}
}

func TestLowVsHighDistinct(t *testing.T) {
	low := IdentityKey("GPT-5.6 Sol", "low")
	high := IdentityKey("GPT-5.6 Sol", "high")
	if low == high {
		t.Fatalf("low and high must be distinct: %v", low)
	}
	m := map[Identity]struct{}{low: {}, high: {}}
	if len(m) != 2 {
		t.Errorf("expected 2 distinct identities, got %d", len(m))
	}
}

func TestAliasGrouping(t *testing.T) {
	if BenchmarkKey("Finance Agent") != BenchmarkKey("FinanceAgent") {
		t.Error("Finance Agent and FinanceAgent must share a key")
	}
	if BenchmarkKey("GDPval") != BenchmarkKey("GDPval-AA") {
		t.Error("GDPval and GDPval-AA must share a key")
	}
	if BenchmarkKey("Finance Agent") == BenchmarkKey("GDPval") {
		t.Error("Finance Agent and GDPval must not share a key")
	}
	names := []string{"Finance Agent", "FinanceAgent", "GDPval", "GDPval-AA"}
	groups := map[string]int{}
	for _, n := range names {
		groups[BenchmarkKey(n)]++
	}
	if len(groups) != 2 {
		t.Errorf("grouping four names yielded %d groups, want 2", len(groups))
	}
}

func TestVariantComposition(t *testing.T) {
	level, ok := ParseEffort("reasoning effort xhigh")
	if level != "xhigh" || !ok {
		t.Fatalf("ParseEffort(reasoning effort xhigh) = (%q, %v), want (xhigh, true)", level, ok)
	}
	if got := CollapseReasoning(level); got != "xhigh" {
		t.Errorf("CollapseReasoning(xhigh) = %q, want xhigh", got)
	}
	if got := IdentityKey("Nova", level); got != (Identity{Model: "Nova", Reasoning: "xhigh"}) {
		t.Errorf("IdentityKey(Nova, xhigh) = %v, want {Nova, xhigh}", got)
	}
}

func TestEmptyVariantPassthrough(t *testing.T) {
	level, ok := ParseEffort("")
	if level != "" || ok {
		t.Fatalf("ParseEffort(\"\") = (%q, %v), want (\"\", false)", level, ok)
	}
	if got := IdentityKey("Nova", ""); got != (Identity{Model: "Nova", Reasoning: ""}) {
		t.Errorf("IdentityKey(Nova, \"\") = %v, want {Nova, \"\"}", got)
	}
}

func TestPipelineOrder(t *testing.T) {
	rows := []struct{ model, reasoning string }{
		{"Claude Opus 4.5 (latest)", "default"},
		{"Claude Opus 4.5 [claude-opus-4-5-20251101]", "high"},
		{"GPT-5.6 Sol", "low"},
	}
	seen := map[Identity]struct{}{}
	for _, r := range rows {
		seen[IdentityKey(r.model, r.reasoning)] = struct{}{}
	}
	if len(seen) != 2 {
		t.Errorf("pipeline order produced %d distinct identities, want 2", len(seen))
	}
	if got := EffortOrder[IdentityKey("GPT-5.6 Sol", "low").Reasoning]; got != 1 {
		t.Errorf("EffortOrder[low] = %d, want 1", got)
	}
}

func TestWhitespaceAnnotationCombo(t *testing.T) {
	if got := CleanModelName("  Claude Opus 4.5 [claude-opus-4-5-20251101]  "); got != "Claude Opus 4.5" {
		t.Errorf("CleanModelName combo = %q, want Claude Opus 4.5", got)
	}
	if got := IdentityKey("  Claude Opus 4.5 [claude-opus-4-5-20251101]  ", "default"); got != (Identity{Model: "Claude Opus 4.5", Reasoning: "high"}) {
		t.Errorf("IdentityKey combo = %v, want {Claude Opus 4.5, high}", got)
	}
}

func TestMaxBareWord(t *testing.T) {
	if level, ok := ParseEffort("max"); level != "max" || !ok {
		t.Errorf("ParseEffort(max) = (%q, %v), want (max, true)", level, ok)
	}
	if level, ok := ParseEffort("max reasoning"); level != "max" || !ok {
		t.Errorf("ParseEffort(max reasoning) = (%q, %v), want (max, true)", level, ok)
	}
}

func TestBenchmarkKeyAcrossEvidenceSources(t *testing.T) {
	u2019 := BenchmarkKey("Humanity’s Last Exam")
	ascii := BenchmarkKey("Humanity's Last Exam")
	if u2019 != ascii || u2019 != "humanityslastexam" {
		t.Errorf("apostrophe variants: %q vs %q, want humanityslastexam", u2019, ascii)
	}
}
