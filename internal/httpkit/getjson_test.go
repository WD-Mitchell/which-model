package httpkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGetJSON(t *testing.T) {
	authCh := make(chan [2]string, 1)
	var sawUA atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			sawUA.Store(r.Header.Get("User-Agent"))
			_, _ = w.Write([]byte(`{"ok":true,"n":3}`))
		case "/bad":
			_, _ = w.Write([]byte(`not json`))
		case "/empty":
			// no body
		case "/arr":
			_, _ = w.Write([]byte(`[1,2]`))
		case "/auth":
			authCh <- [2]string{r.Header.Get("Authorization"), r.Header.Get("User-Agent")}
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	t.Run("ok map", func(t *testing.T) {
		var out map[string]any
		err := NewClient().GetJSON(context.Background(), srv.URL+"/ok", nil, &out)
		if err != nil {
			t.Fatalf("GetJSON err = %v, want nil", err)
		}
		if out["n"] != float64(3) || out["ok"] != true {
			t.Errorf("out = %v, want {n:3, ok:true}", out)
		}
	})

	t.Run("ok struct", func(t *testing.T) {
		var out struct {
			N int `json:"n"`
		}
		err := NewClient().GetJSON(context.Background(), srv.URL+"/ok", nil, &out)
		if err != nil {
			t.Fatalf("GetJSON err = %v, want nil", err)
		}
		if out.N != 3 {
			t.Errorf("out.N = %d, want 3", out.N)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		var out map[string]any
		err := NewClient().GetJSON(context.Background(), srv.URL+"/bad", nil, &out)
		e, ok := AsError(err)
		if !ok || e.Code != "response_json" {
			t.Fatalf("GetJSON err = %v, want response_json", err)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		var out map[string]any
		err := NewClient().GetJSON(context.Background(), srv.URL+"/empty", nil, &out)
		e, ok := AsError(err)
		if !ok || e.Code != "response_json" {
			t.Fatalf("GetJSON err = %v, want response_json", err)
		}
	})

	t.Run("array into map", func(t *testing.T) {
		var out map[string]any
		err := NewClient().GetJSON(context.Background(), srv.URL+"/arr", nil, &out)
		e, ok := AsError(err)
		if !ok || e.Code != "response_json" {
			t.Fatalf("GetJSON err = %v, want response_json", err)
		}
	})

	t.Run("auth header", func(t *testing.T) {
		var out map[string]any
		err := NewClient().GetJSON(context.Background(), srv.URL+"/auth",
			map[string]string{"Authorization": "Bearer sekrit"}, &out)
		if err != nil {
			t.Fatalf("GetJSON err = %v, want nil", err)
		}
		got := <-authCh
		if got[0] != "Bearer sekrit" {
			t.Errorf("server saw Authorization %q, want %q", got[0], "Bearer sekrit")
		}
		if got[1] != "which-model/dev" {
			t.Errorf("server saw User-Agent %q, want %q", got[1], "which-model/dev")
		}
	})

	t.Run("allow list refusal", func(t *testing.T) {
		c := NewClient()
		c.SetAllowList([]string{"https://example.com/"})
		var out map[string]any
		err := c.GetJSON(context.Background(), srv.URL+"/ok", nil, &out)
		e, ok := AsError(err)
		if !ok || e.Code != "endpoint_refused" {
			t.Fatalf("GetJSON err = %v, want endpoint_refused", err)
		}
	})

	t.Run("network error", func(t *testing.T) {
		var out map[string]any
		err := NewClient().GetJSON(context.Background(), "http://127.0.0.1:1", nil, &out)
		e, ok := AsError(err)
		if !ok || e.Code != "network" {
			t.Fatalf("GetJSON err = %v, want network", err)
		}
	})

	t.Run("user agent option", func(t *testing.T) {
		var out map[string]any
		err := NewClient(WithUserAgent("wm/9")).GetJSON(context.Background(), srv.URL+"/ok", nil, &out)
		if err != nil {
			t.Fatalf("GetJSON err = %v, want nil", err)
		}
		if ua, _ := sawUA.Load().(string); ua != "wm/9" {
			t.Errorf("server saw User-Agent %q, want wm/9", ua)
		}
	})
}
