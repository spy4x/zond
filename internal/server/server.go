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
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spy4x/zond/internal/probe"
)

// Config holds the dependencies needed to construct a Server.
type Config struct {
	Targets []Target
	Checker *probe.Checker
	Logger  *slog.Logger
}

// Target is the read-only view of a configured upstream used by handlers.
type Target struct {
	Name    string
	URL     string
	Timeout time.Duration
}

// Server is the HTTP entry point.
type Server struct {
	cfg      Config
	mux      *http.ServeMux
	byName   map[string]Target
	allOrder []string // stable display order
}

// New constructs a Server with all routes registered.
func New(cfg Config) *Server {
	s := &Server{
		cfg:    cfg,
		mux:    http.NewServeMux(),
		byName: make(map[string]Target, len(cfg.Targets)),
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

// Handler returns the registered http.Handler (for use with http.Server).
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) handleOne(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	target, ok := s.byName[name]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown target: %s", name), http.StatusNotFound)
		return
	}

	probeCtx := s.probeCtx(r)
	res := s.cfg.Checker.Check(probeCtx, target.Name, target.URL, target.Timeout)
	writeProbeResult(w, res)
}

func (s *Server) handleAll(w http.ResponseWriter, r *http.Request) {
	targets := make([]probe.Target, 0, len(s.allOrder))
	for _, name := range s.allOrder {
		t := s.byName[name]
		targets = append(targets, probe.Target{Name: t.Name, URL: t.URL, Timeout: t.Timeout})
	}

	probeCtx := s.probeCtx(r)
	results := s.cfg.Checker.CheckAll(probeCtx, targets)

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

func (s *Server) probeCtx(r *http.Request) context.Context {
	return r.Context()
}

// writeProbeResult writes "ok"/"unreachable" with 200 or 503.
func writeProbeResult(w http.ResponseWriter, res probe.Result) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if res.OK {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, "unreachable\n")
}
