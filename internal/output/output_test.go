package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"empty input", []string{}, ""},
		{"single line", []string{"hello"}, "hello\n"},
		{"three lines", []string{"a", "b", "c"}, "a\nb\nc\n"},
		{"single empty line", []string{""}, "\n"},
		{"lines with spaces", []string{"line with spaces", "second"}, "line with spaces\nsecond\n"},
		{"empty middle line", []string{"a", "", "c"}, "a\n\nc\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := RenderLines(&buf, tt.lines); err != nil {
				t.Fatalf("RenderLines() error = %v", err)
			}
			got := buf.String()
			if got != tt.want {
				t.Errorf("RenderLines() = %q, want %q", got, tt.want)
			}
			// Round-trip via strings.Split / strings.TrimSuffix: every input
			// line appears verbatim, each newline-terminated.
			wantLines := strings.Split(tt.want, "\n")
			if len(wantLines) > 0 && wantLines[len(wantLines)-1] == "" {
				wantLines = wantLines[:len(wantLines)-1]
			}
			gotLines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
			if len(tt.lines) == 0 {
				if len(gotLines) != 1 || gotLines[0] != "" {
					t.Errorf("empty input should write nothing, got %q", got)
				}
			} else if len(gotLines) != len(wantLines) {
				t.Errorf("line count = %d, want %d (got %q, want %q)", len(gotLines), len(wantLines), gotLines, wantLines)
			} else {
				for i := range wantLines {
					if gotLines[i] != wantLines[i] {
						t.Errorf("line %d = %q, want %q", i, gotLines[i], wantLines[i])
					}
				}
			}
		})
	}
}
