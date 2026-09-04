package routing

import (
	"encoding/csv"
	"fmt"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"os"
	"testing"
)

func BenchmarkProduceRoutes(b *testing.B) {
	f, err := os.Open("../../data/available_model_scores.csv")
	if err != nil {
		b.Fatal(err)
	}
	records, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil {
		b.Fatal(err)
	}
	rows := make([]identity.Identity, 0, len(records)-1)
	for _, row := range records[1:] {
		rows = append(rows, identity.Identity{Model: row[0], Reasoning: row[1]})
	}
	entries := make([]ModelEntry, 1000)
	for i := range entries {
		row := rows[i%len(rows)]
		entries[i] = ModelEntry{ModelID: fmt.Sprintf("native-%d", i), Name: row.Model, Reasoning: []string{row.Reasoning}}
	}
	in := Input{CatalogRows: rows, Providers: []ProviderInput{{Provider: "benchmark", Kind: usage.KindSubscription, ModelsDev: entries}}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := ProduceRoutes(in)
		if err != nil || len(result.Routes) != len(entries) {
			b.Fatalf("routes=%d err=%v", len(result.Routes), err)
		}
	}
	b.ReportMetric(float64(len(rows)), "identities")
}
