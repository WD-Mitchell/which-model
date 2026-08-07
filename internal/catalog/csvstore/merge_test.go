package csvstore

import (
	"errors"
	"reflect"
	"testing"
)

func rowOf(header []string, values ...string) Row {
	return Row{Header: header, Values: values}
}

func TestCollapseRows(t *testing.T) {
	t.Run("default and high merge to non-default base", func(t *testing.T) {
		header := []string{"model", "reasoning", "intelligence_index", "benchmark:Humanity's Last Exam"}
		rows := []Row{
			rowOf(header, "Example", "default", "10", "61"),
			rowOf(header, "Example", "high", "11", "64"),
		}
		got, err := CollapseRows(rows)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("rows = %d, want 1", len(got))
		}
		want := []string{"Example", "high", "11", "64"}
		if !reflect.DeepEqual(got[0].Values, want) {
			t.Errorf("values = %v, want %v", got[0].Values, want)
		}
	})

	t.Run("base blank metric filled from member", func(t *testing.T) {
		header := []string{"model", "reasoning", "intelligence_index", "benchmark:HLE"}
		rows := []Row{
			rowOf(header, "Example", "high", "", "61"),
			rowOf(header, "Example", "default", "12", ""),
		}
		got, err := CollapseRows(rows)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"Example", "high", "12", "61"}
		if len(got) != 1 || !reflect.DeepEqual(got[0].Values, want) {
			t.Errorf("got %+v, want one row %v", got, want)
		}
	})

	t.Run("first-seen group order", func(t *testing.T) {
		header := []string{"model", "reasoning", "intelligence_index", "benchmark:HLE"}
		rows := []Row{
			rowOf(header, "A", "high", "1", ""),
			rowOf(header, "B", "high", "2", ""),
			rowOf(header, "A", "high", "", "9"),
		}
		got, err := CollapseRows(rows)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Values[0] != "A" || got[1].Values[0] != "B" {
			t.Fatalf("group order = %v, want A then B", got)
		}
	})

	t.Run("blank model errors", func(t *testing.T) {
		header := []string{"model", "reasoning", "intelligence_index"}
		_, err := CollapseRows([]Row{rowOf(header, "", "high", "1")})
		if !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})
}

func TestMergeRows(t *testing.T) {
	header := []string{"model", "reasoning", "intelligence_index", "cost_per_intelligence_index_task_usd"}

	t.Run("fresh wins after collapse", func(t *testing.T) {
		existing := []Row{rowOf(header, "Kimi K2.7 Code", "high", "40.0", "")}
		fresh := []Row{rowOf(header, "Kimi K2.7 Code", "default", "43.0", "")}
		got, err := MergeRows(existing, fresh)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("rows = %d, want 1", len(got))
		}
		want := []string{"Kimi K2.7 Code", "high", "43.0", ""}
		if !reflect.DeepEqual(got[0].Values, want) {
			t.Errorf("values = %v, want %v", got[0].Values, want)
		}
	})

	t.Run("zero is a valid win", func(t *testing.T) {
		existing := []Row{rowOf(header, "M", "high", "1", "55")}
		fresh := []Row{rowOf(header, "M", "high", "1", "0")}
		got, err := MergeRows(existing, fresh)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Values[3] != "0" {
			t.Errorf("cost = %q, want 0", got[0].Values[3])
		}
	})

	t.Run("fresh blank keeps existing", func(t *testing.T) {
		existing := []Row{rowOf(header, "M", "high", "1", "0.22")}
		fresh := []Row{rowOf(header, "M", "high", "1", "")}
		got, err := MergeRows(existing, fresh)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Values[3] != "0.22" {
			t.Errorf("cost = %q, want 0.22", got[0].Values[3])
		}
	})

	benchHeader := []string{"model", "reasoning", "benchmark:SWE-Bench Verified"}

	t.Run("benchmark fallback when not authoritative", func(t *testing.T) {
		existing := []Row{rowOf(benchHeader, "M", "high", "96.0")}
		fresh := []Row{rowOf(benchHeader, "M", "high", "")}
		got, err := MergeRows(existing, fresh)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Values[2] != "96.0" {
			t.Errorf("benchmark = %q, want 96.0", got[0].Values[2])
		}
	})

	t.Run("benchmark cleared when authoritative", func(t *testing.T) {
		existing := []Row{rowOf(benchHeader, "M", "high", "96.0")}
		fresh := []Row{{Header: benchHeader, Values: []string{"M", "high", ""}, Authoritative: map[string]bool{"benchmark:SWE-Bench Verified": true}}}
		got, err := MergeRows(existing, fresh)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Values[2] != "" {
			t.Errorf("benchmark = %q, want blank (cleared)", got[0].Values[2])
		}
	})

	t.Run("fresh-only appended existing-only dropped", func(t *testing.T) {
		existing := []Row{rowOf(header, "Old Model", "high", "9", "")}
		fresh := []Row{rowOf(header, "New Model", "high", "8", "")}
		got, err := MergeRows(existing, fresh)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Values[0] != "New Model" {
			t.Errorf("got %+v, want only New Model", got)
		}
	})
}

func TestMergePartialRefresh(t *testing.T) {
	header := []string{"model", "reasoning", "intelligence_index", "cost_per_intelligence_index_task_usd"}
	existing := []Row{
		rowOf(header, "Old Model", "high", "9", ""),
		rowOf(header, "New Model", "high", "1", ""),
	}
	fresh := []Row{rowOf(header, "New Model", "high", "2", "")}

	t.Run("preserve unselected", func(t *testing.T) {
		got, err := MergePartialRefresh(existing, fresh, []string{"New Model"}, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("rows = %d, want 2", len(got))
		}
		if got[0].Values[0] != "New Model" || got[0].Values[2] != "2" {
			t.Errorf("merged row = %v, want New Model/2", got[0].Values)
		}
		if got[1].Values[0] != "Old Model" || got[1].Values[2] != "9" {
			t.Errorf("appended row = %v, want Old Model/9", got[1].Values)
		}
	})

	t.Run("drop unselected", func(t *testing.T) {
		got, err := MergePartialRefresh(existing, fresh, []string{"New Model"}, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Values[0] != "New Model" {
			t.Errorf("got %+v, want only New Model", got)
		}
	})
}
