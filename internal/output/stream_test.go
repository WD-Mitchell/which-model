package output

import (
	"bytes"
	"errors"
	"testing"
)

type streamFailWriter struct {
	written int
	err     error
}

func (w *streamFailWriter) Write(p []byte) (int, error) {
	if w.written >= 1 {
		return 0, w.err
	}
	n := len(p)
	if n > 1 {
		n = 1
	}
	w.written += n
	return n, w.err
}

func TestStreamHelpers(t *testing.T) {
	tests := []struct {
		name string
		call func(*bytes.Buffer) error
		want string
	}{
		{
			name: "failure network",
			call: func(buf *bytes.Buffer) error {
				return WriteFailure(buf, "pick", "network", "the provider request failed.")
			},
			want: "which-model pick: [network] the provider request failed.\n",
		},
		{
			name: "failure unauthorized",
			call: func(buf *bytes.Buffer) error {
				return WriteFailure(buf, "usage", "unauthorized", "msg")
			},
			want: "which-model usage: [unauthorized] msg\n",
		},
		{
			name: "failure message newline echoed",
			call: func(buf *bytes.Buffer) error {
				return WriteFailure(buf, "a", "b", "multi\nline")
			},
			want: "which-model a: [b] multi\nline\n",
		},
		{
			name: "warning message",
			call: func(buf *bytes.Buffer) error {
				return WriteWarning(buf, "credential file is world-readable")
			},
			want: "warning: credential file is world-readable\n",
		},
		{
			name: "empty warning",
			call: func(buf *bytes.Buffer) error {
				return WriteWarning(buf, "")
			},
			want: "warning: \n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tt.call(&buf); err != nil {
				t.Fatalf("stream helper error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("stream helper output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactIdentity(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		show      bool
		wantNil   bool
		wantValue string
	}{
		{name: "shown value", value: "octocat", show: true, wantValue: "octocat"},
		{name: "hidden value", value: "octocat", show: false, wantNil: true},
		{name: "shown empty value", value: "", show: true, wantValue: ""},
		{name: "hidden empty value", value: "", show: false, wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactIdentity(tt.value, tt.show)
			if tt.wantNil {
				if got != nil {
					t.Errorf("RedactIdentity() = %q, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("RedactIdentity() = nil, want non-nil")
			}
			if *got != tt.wantValue {
				t.Errorf("RedactIdentity() = %q, want %q", *got, tt.wantValue)
			}
		})
	}
}

func TestWriteFailureWriterError(t *testing.T) {
	wantErr := errors.New("writer failed")
	w := &streamFailWriter{err: wantErr}
	if err := WriteFailure(w, "pick", "network", "msg"); !errors.Is(err, wantErr) {
		t.Fatalf("WriteFailure() error = %v, want %v", err, wantErr)
	}
}
