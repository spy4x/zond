// Package probe runs HTTP health checks against upstream targets.
//
// A target is "healthy" iff the upstream responds with a 2xx or 3xx
// status within its per-target timeout. Zond deliberately does NOT
// follow redirects — the 3xx itself is the contract.
package probe

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// DefaultTimeout is the per-target probe timeout used when none is set.
const DefaultTimeout = 5 * time.Second

// Target is the minimum input to probe a single upstream.
type Target struct {
	Name    string
	URL     string
	Timeout time.Duration
}

// Result is the outcome of probing a single target.
type Result struct {
	Name string
	OK   bool
}

// Checker performs HTTP GET probes against targets.
// Safe for concurrent use; the underlying http.Client handles many in flight.
type Checker struct {
	client *http.Client
}

// New returns a Checker. The client transport times out per request via
// context.WithTimeout; DefaultTimeout is the fallback for callers that
// pass zero.
func New() *Checker {
	return &Checker{
		client: &http.Client{
			// Zond treats 3xx as a healthy upstream — inspect the 3xx
			// response itself instead of following to the redirected URL.
			// Following would silently convert 302→200 into a false-OK
			// for a target that is in fact redirecting away from itself.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Check probes one target. Returns OK=true on any 2xx or 3xx response.
// Any error (timeout, DNS, connection refused, malformed URL) → OK=false.
// A zero timeout falls back to DefaultTimeout.
func (c *Checker) Check(ctx context.Context, name, url string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
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
	defer resp.Body.Close()
	_ = drain(resp.Body)

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

// MaxTimeout returns the largest Timeout across the targets,
// or DefaultTimeout when no target has a positive timeout.
func MaxTimeout(targets []Target) time.Duration {
	var max time.Duration
	for _, t := range targets {
		if t.Timeout > max {
			max = t.Timeout
		}
	}
	if max <= 0 {
		return DefaultTimeout
	}
	return max
}
