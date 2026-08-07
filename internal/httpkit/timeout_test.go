package httpkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func timeoutCode(t *testing.T, err error, want string) {
	t.Helper()
	e, ok := AsError(err)
	if !ok || e.Code != want {
		t.Fatalf("err = %v, want code %q", err, want)
	}
}

func TestTimeoutMapping(t *testing.T) {
	blockStarted := make(chan struct{})
	releaseBlock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			time.Sleep(300 * time.Millisecond)
		case "/fast":
			_, _ = w.Write([]byte("ok"))
		case "/block":
			close(blockStarted)
			<-releaseBlock
		}
	}))
	defer srv.Close()

	t.Run("client timeout", func(t *testing.T) {
		c := NewClient(WithTimeout(50 * time.Millisecond))
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/slow", nil)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		_, err = c.Do(context.Background(), req)
		if elapsed := time.Since(start); elapsed >= time.Second {
			t.Errorf("elapsed = %v, want < 1s", elapsed)
		}
		timeoutCode(t, err, "timeout")
	})

	t.Run("caller deadline wins", func(t *testing.T) {
		c := NewClient(WithTimeout(10 * time.Second))
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/slow", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Do(ctx, req)
		timeoutCode(t, err, "timeout")
	})

	t.Run("client deadline applies when earlier", func(t *testing.T) {
		c := NewClient(WithTimeout(50 * time.Millisecond))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/slow", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Do(ctx, req)
		timeoutCode(t, err, "timeout")
	})

	t.Run("caller cancellation", func(t *testing.T) {
		c := NewClient()
		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/block", nil)
		if err != nil {
			t.Fatal(err)
		}
		errCh := make(chan error, 1)
		go func() {
			_, err := c.Do(ctx, req)
			errCh <- err
		}()
		select {
		case <-blockStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("handler never started")
		}
		cancel()
		select {
		case err := <-errCh:
			timeoutCode(t, err, "network")
		case <-time.After(2 * time.Second):
			t.Fatal("Do never returned after cancel")
		}
		close(releaseBlock)
	})

	t.Run("fast handler", func(t *testing.T) {
		c := NewClient(WithTimeout(50 * time.Millisecond))
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/fast", nil)
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

	t.Run("timeout enforced", func(t *testing.T) {
		c := NewClient(WithTimeout(50 * time.Millisecond))
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/slow", nil)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		_, err = c.Do(context.Background(), req)
		if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
			t.Errorf("elapsed = %v, want >= 30ms", elapsed)
		}
		timeoutCode(t, err, "timeout")
	})
}
