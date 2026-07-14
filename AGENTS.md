# AGENTS.md — Zond

## Stack

- **Language:** Go 1.25 (pinned via `go.mod` and Dockerfile)
- **Runtime deps:** `go.yaml.in/yaml/v3` (single external dep)
- **Stdlib:** `net/http` (Go 1.22+ ServeMux patterns), `log/slog`, `context`, `sync`, `os/signal`, `testing`, `httptest`
- **Container:** distroless `static-debian12:nonroot` (~10MB)
- **CI:** Woodpecker on `golang:1.25-alpine`

## Architecture

```
cmd/zond/main.go              — entrypoint, signals, http.Server lifecycle
internal/config/              — YAML + env config loading
internal/probe/               — HTTP probing (sequential + parallel)
internal/server/              — HTTP handlers and routing
```

## Conventions

- **Idiomatic Go:** gofmt-clean, no unused vars/imports, accept interfaces return structs where it helps testing
- **Errors:** return `error`; use `fmt.Errorf("...%w", err)` for wrapping
- **Logging:** `log/slog` with text handler to stderr; structured keys via `slog.Int`, `slog.String`, etc.
- **Concurrency:** bounded parallelism, `sync.WaitGroup` for fan-out, `context.Context` everywhere I/O happens
- **No globals, no init() side effects** — config wired through `main()` → constructors
- **Money/time:** store durations as `time.Duration`, never ints (we don't deal with money here but rule applies repo-wide)
- **Enums:** if you ever need them, start at 1 (`iota + 1`)

## Best practices applied

- Static, stripped binary (`-trimpath -ldflags="-s -w"`)
- CGO disabled for portability
- Distroless nonroot runtime
- Graceful shutdown on SIGINT/SIGTERM
- Bounded HTTP timeouts (`ReadHeaderTimeout`, `WriteTimeout`)
- Body drain after probe to enable HTTP keep-alive
- Per-target timeouts via `context.WithTimeout`
- Race-detector in CI (`go test -race`)

## Hard rules

- **NEVER commit plaintext secrets, env values, API keys, passwords.** `.env.example` uses placeholders only.
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
docker build -t zond .   # build container image
```

## CI

Woodpecker pipeline (`.woodpecker.yml`) runs on every push:
- `go mod verify`
- `go vet ./...`
- `gofmt -l .` (must produce empty output)
- `go test -race -count=1 ./...`
- `go build ./...`

Local dev must pass the same checks before commit.