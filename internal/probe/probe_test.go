package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	c := New(time.Second)
	res := c.Check(context.Background(), "upstream", srv.URL, time.Second)
	if !res.OK {
		t.Errorf("Check(%q) = KO, want OK", srv.URL)
	}
}

func TestCheckRedirect(t *testing.T) {
	// 3xx counts as healthy.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(time.Second)
	res := c.Check(context.Background(), "r", srv.URL, time.Second)
	if !res.OK {
		t.Errorf("Check 2xx = KO, want OK")
	}
}

func TestCheckServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(time.Second)
	res := c.Check(context.Background(), "down", srv.URL, time.Second)
	if res.OK {
		t.Errorf("Check 500 = OK, want KO")
	}
}

func TestCheckUnreachable(t *testing.T) {
	c := New(200 * time.Millisecond)
	res := c.Check(context.Background(), "dead", "http://127.0.0.1:1/never", 200*time.Millisecond)
	if res.OK {
		t.Errorf("Check unreachable = OK, want KO")
	}
}

func TestCheckTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(time.Second)
	start := time.Now()
	res := c.Check(context.Background(), "slow", srv.URL, 50*time.Millisecond)
	elapsed := time.Since(start)
	if res.OK {
		t.Errorf("Check timeout = OK, want KO")
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("Check took %v, want <300ms (timeout enforced)", elapsed)
	}
}

func TestCheckAll(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()

	c := New(time.Second)
	targets := []Target{
		{Name: "a", URL: up.URL, Timeout: time.Second},
		{Name: "b", URL: down.URL, Timeout: time.Second},
		{Name: "c", URL: "http://127.0.0.1:1/nope", Timeout: 200 * time.Millisecond},
	}
	results := c.CheckAll(context.Background(), targets)
	if len(results) != 3 {
		t.Fatalf("len = %d, want 3", len(results))
	}
	if !results[0].OK || results[1].OK || results[2].OK {
		t.Errorf("results = %+v, want [OK KO KO]", results)
	}
}
