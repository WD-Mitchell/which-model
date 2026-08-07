package csvstore

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func rowsOf(header []string, records ...[]string) []Row {
	rows := make([]Row, 0, len(records))
	for _, rec := range records {
		rows = append(rows, Row{Header: header, Values: rec})
	}
	return rows
}

func TestValidateRows(t *testing.T) {
	header := strings.Split(rawHeader, ",")
	row1 := []string{"Claude Opus 5", "max", "63.1", "465", "2.34", "61", "78.0", "59.2"}
	row2 := []string{"Kimi K2.7 Code", "high", "43.0", "", "0.22", "67", "60.8", "30.3"}

	t.Run("valid rows", func(t *testing.T) {
		if err := ValidateRows(rowsOf(header, row1, row2)); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})

	t.Run("duplicate identity", func(t *testing.T) {
		err := ValidateRows(rowsOf(header, row1, row1))
		if !errors.Is(err, ErrDuplicateIdentity) {
			t.Errorf("err = %v, want ErrDuplicateIdentity", err)
		}
	})

	t.Run("blank model", func(t *testing.T) {
		bad := append([]string(nil), row1...)
		bad[0] = ""
		if err := ValidateRows(rowsOf(header, bad)); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("blank reasoning", func(t *testing.T) {
		bad := append([]string(nil), row1...)
		bad[1] = ""
		if err := ValidateRows(rowsOf(header, bad)); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("different headers", func(t *testing.T) {
		other := []Row{
			{Header: header, Values: row1},
			{Header: []string{"x"}, Values: []string{"1"}},
		}
		if err := ValidateRows(other); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})
}

func TestValidateRawHeader(t *testing.T) {
	core := RawCoreColumns

	t.Run("non-benchmark extra column", func(t *testing.T) {
		if err := ValidateRawHeader(append(append([]string{}, core...), "notes")); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("duplicate benchmark column", func(t *testing.T) {
		h := append(append([]string{}, core...), "benchmark:SWE-Bench Verified", "benchmark:SWE-Bench Verified")
		if err := ValidateRawHeader(h); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("wrong case first column", func(t *testing.T) {
		h := append([]string{}, core...)
		h[0] = "Model"
		if err := ValidateRawHeader(h); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})
}

func TestValidateRawRows(t *testing.T) {
	header := strings.Split(rawHeader, ",")

	t.Run("non-numeric intelligence", func(t *testing.T) {
		row := []string{"M", "max", "abc", "1", "1", "1", "1", "1"}
		if err := ValidateRawRows(rowsOf(header, row)); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("negative cost", func(t *testing.T) {
		row := []string{"M", "max", "1", "1", "-1.5", "1", "1", "1"}
		if err := ValidateRawRows(rowsOf(header, row)); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})

	t.Run("non-numeric benchmark cell", func(t *testing.T) {
		h := append(append([]string{}, header...), "benchmark:SWE-Bench Verified")
		row := []string{"M", "max", "1", "1", "1", "1", "1", "1", "not-a-number"}
		if err := ValidateRawRows(rowsOf(h, row)); !errors.Is(err, ErrMalformedCSV) {
			t.Errorf("err = %v, want ErrMalformedCSV", err)
		}
	})
}

func TestResolveBenchmarkColumns(t *testing.T) {
	got := ResolveBenchmarkColumns(
		[][]string{{"SWE-Bench Verified", "Terminal-Bench"}, {"Terminal-Bench", "Toolathlon"}},
		[]string{"Toolathlon", "MCP Atlas"},
	)
	want := []string{"SWE-Bench Verified", "Terminal-Bench", "Toolathlon", "MCP Atlas"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
