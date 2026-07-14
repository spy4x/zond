// Package probe runs HTTP health checks against upstream targets.
package probe

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Target is the minimum interface needed to probe a single upstream.
// Kept independent of the config package to avoid import cycles
// and to make this package trivially testable.
type Target struct {
	Name    string
	URL     string
	Timeout time.Duration
}

// Checker performs HTTP GET probes against targets.
// Safe for concurrent use; the underlying http.Client handles many in flight.
type Checker struct {
	client *http.Client
}

// New returns a Checker using defaultTimeout as the upper bound for any
// per-target timeout that resolves to zero.
func New(defaultTimeout time.Duration) *Checker {
	return &Checker{
		client: &http.Client{
			Timeout: defaultTimeout,
			// Follow up to 5 redirects — 3xx counts as a healthy upstream.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// Result is the outcome of probing a single target.
type Result struct {
	Name string
	OK   bool
}

// Check probes one target. Returns OK=true on any 2xx or 3xx response.
// Any error (timeout, DNS, connection refused, malformed URL) → OK=false.
func (c *Checker) Check(ctx context.Context, name, url string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return Result{Name: name, OK: false}
	}
	req.Header.Set("User-Agent", "Zond/1.0")
	req.Header.Set("Accept", "*/*")

	resp, err := c.client.Do(req)
	if err != nil {
		return Result{Name: name, OK: false}
	}
	// Drain body so the underlying connection can be reused.
	defer resp.Body.Close()
	_, _ = drain(resp.Body)

	return Result{Name: name, OK: resp.StatusCode >= 200 && resp.StatusCode < 400}
}

// CheckAll probes every target in parallel and returns results in the
// same order as the input slice. Failures are isolated — one bad target
// never aborts the others.
func (c *Checker) CheckAll(ctx context.Context, targets []Target) []Result {
	results := make([]Result, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t Target) {
			defer wg.Done()
			results[i] = c.Check(ctx, t.Name, t.URL, t.Timeout)
		}(i, t)
	}
	wg.Wait()
	return results
}
