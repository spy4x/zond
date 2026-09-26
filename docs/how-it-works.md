# How zond works

## Why Zond?

Monitoring services behind an SSO proxy (Authelia, Authentik) is painful.
You either accept `302` redirects as "healthy" or expose your apps with
dedicated monitoring users. Zond sits **_beside_** your containers (same
Docker network) and probes them directly, returning only `200` or `503`.
No auth bypass, no password management.

**One line in Gatus:**

```yaml
- name: Metube
  url: "https://zond.example.com/health/metube"
  conditions:
    - "[STATUS] == 200"
```

## Features

- **Zero attack surface** — returns `200` or `503`, no internal URLs, no tokens
- **Single endpoint per service** — `GET /health/<name>`
- **Bulk check** — `GET /` or `GET /health` lists all targets by name only
- **Config-driven** — YAML file (`zond.yml` by default, falls back to `zond.yaml`) or `ZOND_TARGETS` env var
- **Per-target timeout** — configure probe timeouts individually (ms in YAML)
- **Parallel probes** — every target probed concurrently, fan-out bounded
- **Detached probe context** — client disconnect cannot poison fan-out results
- **Tiny image** — ~10MB distroless Docker image, single static binary
- **Single dep** — one external library (`go.yaml.in/yaml/v3`)

## Architecture

```
cmd/zond/main.go              — entrypoint, -healthcheck flag, http.Server lifecycle
internal/config/config.go     — YAML + env loader, validation, duplicate-name rejection
internal/probe/probe.go       — HTTP GET with per-target timeout, parallel fan-out, redirect contract
internal/probe/drain.go       — bounded body drain for HTTP keep-alive
internal/server/server.go     — HTTP handlers, routing, response codes
```

Dependency policy: **stdlib first**, one external dep (`go.yaml.in/yaml/v3`)
for YAML parsing. No router, no logger lib, no DI framework — Go 1.25 stdlib
covers it all.

## Why not TCP checks?

TCP checks (`tcp://hl-metube:8081`) confirm a port is open. Zond performs
a real HTTP request and validates the response — catching cases where the
process is listening but returning 5xx errors.

## Why no authentication?

Zond returns only `ok` or `ko`. No data to protect, no session to steal,
no action to perform. Adding auth would reintroduce the exact problem Zond
solves. If you must, proxy it through your SSO — but the health endpoint
itself carries zero risk.

## Development

```bash
go test ./...            # unit tests
go test -race ./...      # race detector
go vet ./...             # static analysis
gofmt -l .               # formatting check (empty = clean)
go build ./...           # compile everything
```
