package httpkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRedirectHardFail(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/loc301":
			http.Redirect(w, r, "/target", http.StatusMovedPermanently)
		case "/loc302":
			http.Redirect(w, r, "/target", http.StatusFound)
		case "/noloc":
			w.WriteHeader(http.StatusMultipleChoices)
		case "/ok":
			_, _ = w.Write([]byte("ok"))
		case "/target":
			_, _ = w.Write([]byte("target"))
		}
	}))
	defer srv.Close()

	c := NewClient()

	t.Run("301", func(t *testing.T) {
		before := hits.Load()
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/loc301", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Do(context.Background(), req)
		e, ok := AsError(err)
		if !ok || e.Code != "redirect_refused" {
			t.Fatalf("Do err = %v, want redirect_refused", err)
		}
		if got := hits.Load(); got != before+1 {
			t.Errorf("hits = %d, want %d (redirect never followed)", got, before+1)
		}
	})

	t.Run("302", func(t *testing.T) {
		before := hits.Load()
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/loc302", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Do(context.Background(), req)
		e, ok := AsError(err)
		if !ok || e.Code != "redirect_refused" {
			t.Fatalf("Do err = %v, want redirect_refused", err)
		}
		if got := hits.Load(); got != before+1 {
			t.Errorf("hits = %d, want %d (redirect never followed)", got, before+1)
		}
	})

	t.Run("no location", func(t *testing.T) {
		before := hits.Load()
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/noloc", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Do(context.Background(), req)
		e, ok := AsError(err)
		if !ok || e.Code != "redirect_refused" {
			t.Fatalf("Do err = %v, want redirect_refused", err)
		}
		if got := hits.Load(); got != before+1 {
			t.Errorf("hits = %d, want %d", got, before+1)
		}
	})

	t.Run("ok", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/ok", nil)
		if err != nil {
			t.Fatal(err)
		}
		body, err := c.Do(context.Background(), req)
		if err != nil {
			t.Fatalf("Do err = %v, want nil", err)
		}
		if string(body) != "ok" {
			t.Errorf("body = %q, want ok", body)
		}
	})

	t.Run("twice", func(t *testing.T) {
		before := hits.Load()
		for range 2 {
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/loc301", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Do(context.Background(), req)
			e, ok := AsError(err)
			if !ok || e.Code != "redirect_refused" {
				t.Fatalf("Do err = %v, want redirect_refused", err)
			}
		}
		if got := hits.Load(); got != before+2 {
			t.Errorf("hits = %d, want %d", got, before+2)
		}
	})
}
