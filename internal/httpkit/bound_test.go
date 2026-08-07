package httpkit

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeBoundStream(w http.ResponseWriter, n int) {
	for start := 0; start < n; start += 100 {
		end := min(start+100, n)
		chunk := make([]byte, end-start)
		for i := range chunk {
			chunk[i] = byte(start + i)
		}
		_, _ = w.Write(chunk)
		w.(http.Flusher).Flush()
	}
}

func boundBody(n int) []byte {
	body := make([]byte, n)
	for i := range body {
		body[i] = byte(i)
	}
	return body
}

func TestResponseBound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clbig":
			w.Header().Set("Content-Length", "200")
			_, _ = w.Write(make([]byte, 200))
		case "/stream":
			writeBoundStream(w, 200)
		case "/stream2":
			writeBoundStream(w, 99)
		case "/clok":
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write(make([]byte, 100))
		case "/clbad":
			w.Header().Set("Content-Length", "abc")
			_, _ = w.Write(make([]byte, 10))
		case "/default":
			writeBoundStream(w, 300_000)
		case "/exact":
			writeBoundStream(w, 262_144)
		}
	}))
	defer srv.Close()

	cases := []struct {
		name     string
		client   *Client
		path     string
		wantCode string
		wantBody []byte
	}{
		{"clbig", NewClient(WithMaxBytes(100)), "/clbig", "response_too_large", nil},
		{"stream", NewClient(WithMaxBytes(100)), "/stream", "response_too_large", nil},
		{"clok", NewClient(WithMaxBytes(100)), "/clok", "", make([]byte, 100)},
		{"stream2", NewClient(WithMaxBytes(100)), "/stream2", "", boundBody(99)},
		{"clbad", NewClient(WithMaxBytes(100)), "/clbad", "", make([]byte, 10)},
		{"default", NewClient(), "/default", "response_too_large", nil},
		{"exact boundary", NewClient(WithMaxBytes(262_144)), "/exact", "", boundBody(262_144)},
		{"above boundary", NewClient(WithMaxBytes(262_145)), "/stream", "", boundBody(200)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := tc.client.Do(context.Background(), req)
			if tc.wantCode != "" {
				e, ok := AsError(err)
				if !ok || e.Code != tc.wantCode {
					t.Fatalf("Do err = %v, want %s", err, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("Do err = %v, want nil", err)
			}
			if !bytes.Equal(body, tc.wantBody) {
				t.Errorf("body mismatch: got %d bytes, want %d", len(body), len(tc.wantBody))
			}
		})
	}
}
