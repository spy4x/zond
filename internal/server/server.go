// Package server exposes the HTTP handlers for Zond.
//
// Endpoints:
//
//	GET /health/{name}  — check a single target; 200/503/404
//	GET /health         — list all targets with overall 200 or 503
//	GET /               — alias for /health
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spy4x/zond/internal/probe"
)

// Config holds the dependencies needed to construct a Server.
type Config struct {
	Targets []probe.Target
	Checker *probe.Checker
	Logger  *slog.Logger
}

// Server is the HTTP entry point.
type Server struct {
	cfg      Config
	mux      *http.ServeMux
	byName   map[string]probe.Target
	allOrder []string // stable display order
}

// New constructs a Server with all routes registered.
func New(cfg Config) *Server {
	s := &Server{
		cfg:    cfg,
		mux:    http.NewServeMux(),
		byName: make(map[string]probe.Target, len(cfg.Targets)),
	}
	for _, t := range cfg.Targets {
		s.byName[t.Name] = t
		s.allOrder = append(s.allOrder, t.Name)
	}
	sort.Strings(s.allOrder)

	s.mux.HandleFunc("GET /health/{name}", s.handleOne)
	s.mux.HandleFunc("GET /health", s.handleAll)
	s.mux.HandleFunc("GET /{$}", s.handleAll) // exact "/" only
	return s
}

// Handler returns the registered http.Handler.
func (s *Server) Handler() http.Handler { return s.mux }

// overallTimeout returns the upper bound for fan-out probes —
// the longest per-target timeout plus a small safety margin.
// Used to construct a context that survives client disconnect
// so probes can still report accurate results.
func overallTimeout(targets []probe.Target) time.Duration {
	return probe.MaxTimeout(targets) + time.Second
}

func (s *Server) handleOne(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	target, ok := s.byName[name]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown target: %s", name), http.StatusNotFound)
		return
	}

	// Detach probe lifetime from the request: a slow client cancelling
	// the request must not register as "all targets down".
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout(s.cfg.Targets))
	defer cancel()

	res := s.cfg.Checker.Check(ctx, target.Name, target.URL, target.Timeout)
	writeProbeResult(w, res)
}

func (s *Server) handleAll(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout(s.cfg.Targets))
	defer cancel()

	results := s.cfg.Checker.CheckAll(ctx, s.cfg.Targets)

	var b strings.Builder
	allOK := true
	for _, res := range results {
		if !res.OK {
			allOK = false
		}
		status := "KO"
		if res.OK {
			status = "OK"
		}
		b.WriteString(status)
		b.WriteByte(' ')
		b.WriteString(res.Name)
		b.WriteByte('\n')
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !allOK {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_, _ = w.Write([]byte(b.String()))
}

func writeProbeResult(w http.ResponseWriter, res probe.Result) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if res.OK {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok\n")
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprint(w, "unreachable\n")
}
