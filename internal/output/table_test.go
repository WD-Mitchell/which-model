package output

import (
	"bytes"
	"testing"
)

func TestRenderTable(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		want    string
		wantErr bool
	}{
		{
			name:    "no headers writes nothing",
			headers: []string{},
			rows:    nil,
			want:    "",
		},
		{
			name:    "header only",
			headers: []string{"name"},
			rows:    nil,
			want:    "name\n",
		},
		{
			name:    "single row matching headers",
			headers: []string{"a", "b"},
			rows:    [][]string{{"1", "2"}},
			want:    "a b\n1 2\n",
		},
		{
			name:    "column width from cells",
			headers: []string{"name", "used"},
			rows:    [][]string{{"claude", "25%"}},
			want:    "name   used\nclaude 25% \n",
		},
		{
			name:    "cell wider than header",
			headers: []string{"x"},
			rows:    [][]string{{"longer"}},
			want:    "x     \nlonger\n",
		},
		{
			name:    "short row padded with empty cell",
			headers: []string{"h1", "h2"},
			rows:    [][]string{{"only-one"}},
			want:    "h1       h2\nonly-one\n",
		},
		{
			name:    "row longer than header errors without output",
			headers: []string{"a", "b"},
			rows:    [][]string{{"1", "2", "3"}},
			want:    "",
			wantErr: true,
		},
		{
			name:    "width from longest cell",
			headers: []string{"a"},
			rows:    [][]string{{"1"}, {"22"}, {"333"}},
			want:    "a  \n1  \n22 \n333\n",
		},
		{
			name:    "empty cell in row",
			headers: []string{"p", "q"},
			rows:    [][]string{{"r", "s"}, {"", "t"}},
			want:    "p q\nr s\n  t\n",
		},
		{
			name:    "single column rows",
			headers: []string{"a"},
			rows:    [][]string{{"1"}, {"2"}},
			want:    "a\n1\n2\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RenderTable(&buf, tt.headers, tt.rows)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RenderTable() error = nil, want error")
				}
				if got := buf.String(); got != "" {
					t.Errorf("RenderTable() wrote %q on error, want empty buffer", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderTable() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("RenderTable() = %q, want %q", got, tt.want)
			}
		})
	}
}
