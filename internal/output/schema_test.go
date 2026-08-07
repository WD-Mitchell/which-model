package output

import (
	"bytes"
	"errors"
	"testing"
)

type schemaFailWriter struct {
	err error
}

func (w *schemaFailWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > 1 {
		n = 1
	}
	return n, w.err
}

func TestPrintSchema(t *testing.T) {
	tests := []struct {
		name      string
		doc       map[string]any
		want      string
		contains  string
		deterministic bool
	}{
		{
			name: "small schema document",
			doc:  map[string]any{"type": "object", "title": "usage"},
			want: "{\"title\":\"usage\",\"type\":\"object\"}\n",
		},
		{
			name: "empty document",
			doc:  map[string]any{},
			want: "{}\n",
		},
		{
			name:     "int64 precision preserved",
			doc:      map[string]any{"n": int64(9_007_199_254_740_993)},
			contains: `"n":9007199254740993`,
		},
		{
			name:          "deterministic document",
			doc:           map[string]any{"b": 1, "a": 2},
			deterministic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := PrintSchema(&buf, tt.doc); err != nil {
				t.Fatalf("PrintSchema() error = %v", err)
			}
			if tt.want != "" && buf.String() != tt.want {
				t.Errorf("PrintSchema() = %q, want %q", buf.String(), tt.want)
			}
			if tt.contains != "" && !bytes.Contains(buf.Bytes(), []byte(tt.contains)) {
				t.Errorf("PrintSchema() = %q, want substring %q", buf.String(), tt.contains)
			}
			if tt.deterministic {
				var second bytes.Buffer
				if err := PrintSchema(&second, tt.doc); err != nil {
					t.Fatalf("second PrintSchema() error = %v", err)
				}
				if !bytes.Equal(buf.Bytes(), second.Bytes()) {
					t.Errorf("PrintSchema() is not deterministic: %q vs %q", buf.String(), second.String())
				}
			}
		})
	}
}

func TestPrintSchemaIndex(t *testing.T) {
	tests := []struct {
		name     string
		commands []string
		want     string
	}{
		{
			name:     "commands in given order",
			commands: []string{"usage", "pick"},
			want:     "{\"commands\":[\"usage\",\"pick\"]}\n",
		},
		{
			name:     "nil commands",
			commands: nil,
			want:     "{\"commands\":null}\n",
		},
		{
			name:     "empty commands",
			commands: []string{},
			want:     "{\"commands\":[]}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := PrintSchemaIndex(&buf, tt.commands); err != nil {
				t.Fatalf("PrintSchemaIndex() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("PrintSchemaIndex() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintSchemaWriterError(t *testing.T) {
	wantErr := errors.New("writer failed")
	w := &schemaFailWriter{err: wantErr}
	if err := PrintSchema(w, map[string]any{"type": "object"}); !errors.Is(err, wantErr) {
		t.Fatalf("PrintSchema() error = %v, want %v", err, wantErr)
	}
}
