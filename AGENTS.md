# AGENTS.md — Zond

## Stack

- **Language:** Go 1.25
- **Runtime deps:** `go.yaml.in/yaml/v3` (single external dep)
- **Stdlib:** `net/http` (Go 1.22+ ServeMux patterns), `log/slog`, `context`, `sync`, `os/signal`, `net`, `flag`, `testing`, `httptest`
- **Container:** distroless `static-debian12:nonroot` (~10MB)
- **CI:** Woodpecker on `golang:1.25-alpine`

## Invariants

- **Stateless / no persistence.** Zond keeps no state between requests (no DB, no cache, no files). Each request reads the resolved config once at startup and probes upstreams fresh.
- **One-process, one-port.** The HTTP server is the only long-lived goroutine; probes fan out and die.
- **Stdlib first.** Add a third-party dep only with explicit justification in PR.

## Architecture

```
cmd/zond/main.go              — entrypoint, flags, signal handling, http.Server lifecycle
internal/config/              — YAML + env config loading with validation
internal/probe/               — HTTP probing (sequential + parallel), drain helper
internal/server/              — HTTP handlers and routing
```

## Conventions

- **Idiomatic Go:** gofmt-clean, no unused vars/imports, accept interfaces return structs where it helps testing
- **Errors:** return `error`; wrap with `fmt.Errorf("...%w", err)`
- **Logging:** `log/slog` text handler to stderr; structured keys via `slog.Int`, `slog.String`, etc.
- **Concurrency:** bounded parallelism, `sync.WaitGroup` for fan-out, `context.Context` everywhere I/O happens
- **No globals, no init() side effects** — config wired through `main()` → constructors
- **Probe lifetimes detached from request lifetimes** — a client disconnect must not register as "all targets down"
- **3xx is healthy.** Zond does NOT follow redirects; the 3xx response itself is the contract.

## Types

`probe.Target` is the canonical Target struct (Name, URL, Timeout as `time.Duration`).
YAML stores `timeout` as integer milliseconds — common with Prometheus, Gatus, and k8s
probes; avoids parser-specific duration strings.

## Best practices applied

- Static, stripped binary (`-trimpath -ldflags="-s -w"`)
- CGO disabled for portability
- Distroless nonroot runtime
- Graceful shutdown on SIGINT/SIGTERM
- Bounded HTTP timeouts (`ReadHeaderTimeout`, `WriteTimeout`)
- Body drain after probe to enable HTTP keep-alive
- Per-target timeouts via `context.WithTimeout`
- Self-probe via `-healthcheck` flag (used by Dockerfile `HEALTHCHECK`)
- Race-detector in CI (`go test -race`)

## Hard rules

- **NEVER commit plaintext secrets, env values, API keys, passwords.**
- **NEVER break API contract:** `/health/<name>`, `/health`, `/` responses stay backward-compatible
- **Fail-open principle:** non-critical subsystems (logging) never block primary operation
- **Single dependency policy:** justify any new third-party dep in PR description; default is stdlib

## Commands

```bash
go test ./...            # run unit tests
go test -race ./...      # with race detector
go vet ./...             # static analysis
gofmt -l .               # format check (empty = clean)
go build ./...           # compile all packages
go build -o zond ./cmd/zond && ./zond -healthcheck  # manual healthcheck smoke
docker build -t zond .   # build container image
```

## CI

Woodpecker pipeline (`.woodpecker.yml`) on `golang:1.25-alpine`:
- `go mod verify`
- `go vet ./...`
- `gofmt -l .`
- `go test -race -count=1 ./...` (gcc + musl-dev installed for cgo)
- `go build ./...`

Local dev must pass the same checks before commit.
