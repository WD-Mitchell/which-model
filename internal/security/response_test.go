package security

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"
)

type responseReadCloser struct {
	io.Reader
	closed bool
}

func (r *responseReadCloser) Close() error {
	r.closed = true
	return nil
}

type responseErrorReader struct {
	read bool
}

func (r *responseErrorReader) Read(p []byte) (int, error) {
	if !r.read {
		r.read = true
		p[0] = 'x'
		return 1, nil
	}
	return 0, errors.New("read boom")
}

func TestReadResponseBounded(t *testing.T) {
	const tooLarge = "response_too_large: The provider response exceeded the safe size limit."

	tests := []struct {
		name          string
		contentLength int64
		body          io.ReadCloser
		maxBytes      int64
		wantLen       int
		wantErr       string
		noLeak        string
	}{
		{"header oversized", 200, io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 200))), 100, 0, tooLarge, ""},
		{"stream oversized", -1, io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 200))), 100, 0, tooLarge, ""},
		{"header boundary", 100, io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 100))), 100, 100, "", ""},
		{"stream boundary", -1, io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 99))), 100, 99, "", ""},
		{"default bound oversized", -1, io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), MaxResponseBytes+1))), MaxResponseBytes, 0, tooLarge, ""},
		{"default bound boundary", MaxResponseBytes, io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), MaxResponseBytes))), MaxResponseBytes, MaxResponseBytes, "", ""},
		{"zero bound", -1, io.NopCloser(bytes.NewReader([]byte("x"))), 0, 0, tooLarge, ""},
		{"body not leaked", -1, io.NopCloser(bytes.NewReader([]byte("CANARY_BODY_SECRET"))), 2, 0, tooLarge, "CANARY_BODY_SECRET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: 200, Body: tt.body, ContentLength: tt.contentLength}
			data, err := ReadResponseBounded(resp, tt.maxBytes)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ReadResponseBounded() error = %v, want nil", err)
				}
				if len(data) != tt.wantLen {
					t.Fatalf("ReadResponseBounded() returned %d bytes, want %d", len(data), tt.wantLen)
				}
				return
			}
			if err == nil {
				t.Fatalf("ReadResponseBounded() = nil, want error %q", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Fatalf("ReadResponseBounded() error = %q, want %q", got, tt.wantErr)
			}
			if tt.noLeak != "" && bytes.Contains([]byte(err.Error()), []byte(tt.noLeak)) {
				t.Fatalf("error %q leaks %q", err, tt.noLeak)
			}
		})
	}

	t.Run("read errors pass through", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(&responseErrorReader{}), ContentLength: -1}
		_, err := ReadResponseBounded(resp, 100)
		if err == nil || err.Error() != "read boom" {
			t.Fatalf("ReadResponseBounded() error = %v, want read boom", err)
		}
	})

	t.Run("body is not closed", func(t *testing.T) {
		body := &responseReadCloser{Reader: bytes.NewReader([]byte("ok"))}
		resp := &http.Response{StatusCode: 200, Body: body, ContentLength: 2}
		if _, err := ReadResponseBounded(resp, 2); err != nil {
			t.Fatalf("ReadResponseBounded() error = %v, want nil", err)
		}
		if body.closed {
			t.Fatal("ReadResponseBounded() closed the response body")
		}
	})
}
