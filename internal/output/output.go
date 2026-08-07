// Package output renders command output: JSON, text, and JSON Schema.
package output

import (
	"fmt"
	"io"
	"strings"
)

// RenderLines writes each line followed by "\n"; empty input writes nothing.
func RenderLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// RenderTable writes an aligned table: header row first, each column padded to
// the width of its widest cell (header included), cells joined with one space,
// each row newline-terminated. Rows shorter than the header are padded with
// empty cells; a row longer than the header returns an error.
func RenderTable(w io.Writer, headers []string, rows [][]string) error {
	if len(headers) == 0 {
		return nil
	}
	// Validate all rows before writing anything.
	for i, row := range rows {
		if len(row) > len(headers) {
			return fmt.Errorf("output: row %d has %d cells, header has %d", i, len(row), len(headers))
		}
	}
	// Column width = widest cell in that column across header and rows; rows
	// with fewer than len(headers) cells count as padded with "".
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	writeRow := func(cells []string) error {
		var line strings.Builder
		for i, cell := range cells {
			if i > 0 {
				line.WriteByte(' ')
			}
			line.WriteString(cell)
			line.WriteString(strings.Repeat(" ", widths[i]-len(cell)))
		}
		_, err := fmt.Fprintln(w, line.String())
		return err
	}
	if err := writeRow(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(row); err != nil {
			return err
		}
	}
	return nil
}
