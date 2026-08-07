package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// errAfterWriter returns errors.New("boom") on any Write once 1 byte has been
// written; the first byte is accepted.
type errAfterWriter struct {
	written int
}

func (w *errAfterWriter) Write(p []byte) (int, error) {
	if w.written >= 1 {
		return 0, errors.New("boom")
	}
	n := len(p)
	if n > 1 {
		n = 1
	}
	w.written += n
	return n, nil
}

// decodeJSON inspects b with json.Decoder + UseNumber into a map.
func decodeJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode output %q: %v", b, err)
	}
	return m
}

func TestRenderJSON(t *testing.T) {
	tests := []struct {
		name          string
		env           OutputEnvelope
		payload       any
		wantExact     string // exact expected bytes; "" means not checked
		wantContains  []string
		wantNotContain []string
		wantErr       error // checked with errors.Is
	}{
		{
			name:     "envelope merged with payload",
			env:      OutputEnvelope{UsageEnabled: true},
			payload:  map[string]any{"ok": true},
			wantExact: "{\"ok\":true,\"schema_version\":\"2.0\",\"usage_enabled\":true}\n",
		},
		{
			name:     "struct payload, no disabled reason",
			env:      OutputEnvelope{},
			payload:  struct {
				A string `json:"a"`
			}{A: "x"},
			wantContains: []string{`"a":"x"`, `"schema_version":"2.0"`, `"usage_enabled":false`},
			wantNotContain: []string{"usage_disabled_reason"},
		},
		{
			name:     "disabled reason present",
			env:      OutputEnvelope{UsageEnabled: false, UsageDisabledReason: "flag"},
			payload:  map[string]any{},
			wantExact: "{\"schema_version\":\"2.0\",\"usage_disabled_reason\":\"flag\",\"usage_enabled\":false}\n",
		},
		{
			name:     "non-empty env schema version wins",
			env:      OutputEnvelope{SchemaVersion: "9.9"},
			payload:  map[string]any{},
			wantContains: []string{`"schema_version":"9.9"`},
		},
		{
			name:    "reserved schema_version",
			env:     OutputEnvelope{},
			payload: map[string]any{"schema_version": "1.0"},
			wantErr: ErrReservedField,
		},
		{
			name:    "reserved usage_disabled_reason",
			env:     OutputEnvelope{},
			payload: map[string]any{"usage_disabled_reason": "x"},
			wantErr: ErrReservedField,
		},
		{
			name:    "array payload not an object",
			env:     OutputEnvelope{},
			payload: []string{"a"},
			wantErr: ErrPayloadNotObject,
		},
		{
			name:    "string payload not an object",
			env:     OutputEnvelope{},
			payload: "a string",
			wantErr: ErrPayloadNotObject,
		},
		{
			name:     "int64 precision preserved",
			env:      OutputEnvelope{},
			payload:  map[string]any{"n": int64(9_007_199_254_740_993)},
			wantContains: []string{`"n":9007199254740993`},
		},
		{
			name:     "sorted keys exact bytes",
			env:      OutputEnvelope{},
			payload:  map[string]any{"b": 1, "a": 2},
			wantExact: "{\"a\":2,\"b\":1,\"schema_version\":\"2.0\",\"usage_enabled\":false}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RenderJSON(&buf, tt.env, tt.payload)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("RenderJSON() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
				}
				if got := buf.String(); got != "" {
					t.Errorf("RenderJSON() wrote %q on error, want empty buffer", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderJSON() error = %v", err)
			}
			out := buf.Bytes()
			if tt.wantExact != "" {
				if got := buf.String(); got != tt.wantExact {
					t.Errorf("RenderJSON() = %q, want %q", got, tt.wantExact)
				}
			}
			for _, want := range tt.wantContains {
				if !bytes.Contains(out, []byte(want)) {
					t.Errorf("output %q does not contain %q", out, want)
				}
			}
			for _, want := range tt.wantNotContain {
				if bytes.Contains(out, []byte(want)) {
					t.Errorf("output %q should not contain %q", out, want)
				}
			}
			// Every document must decode as a JSON object carrying the
			// envelope fields.
			m := decodeJSON(t, out)
			if _, ok := m["schema_version"]; !ok {
				t.Errorf("output %q missing schema_version", out)
			}
			if _, ok := m["usage_enabled"]; !ok {
				t.Errorf("output %q missing usage_enabled", out)
			}
			if tt.env.UsageDisabledReason == "" {
				if _, ok := m["usage_disabled_reason"]; ok {
					t.Errorf("output %q carries usage_disabled_reason but envelope left it empty", out)
				}
			}
			// UseNumber precision spot-check.
			if v, ok := m["n"]; ok {
				num, isNum := v.(json.Number)
				if !isNum {
					t.Errorf("m[\"n\"] = %T (%v), want json.Number", v, v)
				} else if num.String() != "9007199254740993" {
					t.Errorf("m[\"n\"] = %s, want 9007199254740993", num.String())
				}
			}
		})
	}
}

func TestRenderJSONDeterministic(t *testing.T) {
	env := OutputEnvelope{UsageEnabled: true}
	payload := map[string]any{"b": 1, "a": 2}
	var buf1, buf2 bytes.Buffer
	if err := RenderJSON(&buf1, env, payload); err != nil {
		t.Fatalf("first RenderJSON() error = %v", err)
	}
	if err := RenderJSON(&buf2, env, payload); err != nil {
		t.Fatalf("second RenderJSON() error = %v", err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Errorf("non-deterministic output: %q vs %q", buf1.String(), buf2.String())
	}
}

func TestRenderJSONWriterError(t *testing.T) {
	w := &errAfterWriter{}
	err := RenderJSON(w, OutputEnvelope{}, map[string]any{"ok": true})
	if err == nil {
		t.Fatal("RenderJSON() error = nil, want writer error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("RenderJSON() error = %v, want it to contain \"boom\"", err)
	}
}
