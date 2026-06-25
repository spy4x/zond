# Zond 🚀

[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Fspy4x%2Fzond-blue)](https://github.com/spy4x/zond/pkgs/container/zond)
[![Deno](https://img.shields.io/badge/deno-2.2-black?logo=deno)](https://deno.com)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![GitHub](https://img.shields.io/badge/github-spy4x%2Fzond-181717?logo=github)](https://github.com/spy4x/zond)

**Zond** (Зонд — Russian: "probe") is a tiny internal health probe bridge.
It receives external health check requests from monitoring systems
like [Gatus](https://github.com/TwiN/gatus) and forwards them to internal
containers via Docker DNS names — no authentication required, no internal
details exposed.

```
┌──────────┐     ┌──────────┐     ┌─────────────┐
│  Gatus   │────▶│  Zond    │────▶│ hl-metube   │
│ (cloud)  │     │ (home)   │     │ :8081       │
└──────────┘     └──────────┘     └─────────────┘
```

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
- **Per-target timeout** — configure probe timeouts individually
- **Docker-native** — 30MB Deno image, no dependencies
- **Standalone binary** — `deno task compile` for bare-metal

## Quick start

### 1. Config file

```yaml
# zond.yml
port: 8080
targets:
  - name: metube
    url: http://hl-metube:8081/
  - name: ollama
    url: http://hl-ollama:11434/api/tags
    timeout: 10000  # ms, default 5000
  - name: grafana
    url: http://hl-grafana:3000/api/health
    timeout: 3000
```

```bash
docker run -p 8080:8080 \
  -v $(pwd)/zond.yml:/app/zond.yml \
  ghcr.io/spy4x/zond:latest

curl http://localhost:8080/health/metube
# ok
```

### 2. Environment variables

```bash
docker run -p 8080:8080 \
  -e ZOND_TARGETS="metube=http://hl-metube:8081/,ollama=http://hl-ollama:11434/api/tags" \
  ghcr.io/spy4x/zond:latest
```

Note: env var targets use the default timeout (5000ms). For custom timeouts,
use a config file.

### 3. Docker Compose

```yaml
services:
  zond:
    image: ghcr.io/spy4x/zond:latest
    container_name: zond
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./zond.yml:/app/zond.yml:ro
    networks:
      - internal  # same network as your probes
    deploy:
      resources:
        limits:
          memory: 32M
          cpus: "0.1"
```

## API

### `GET /health/<name>`

| Response | Status | Meaning |
|---|---|---|
| `ok\n` | `200` | Target responded 2xx/3xx |
| `unreachable\n` | `503` | Connection failed or 4xx/5xx |
| `unknown target: <name>\n` | `404` | Target not in config |

### `GET /` or `GET /health`

Returns one line per target, overall `200` if all healthy:

```
OK metube
KO grafana
OK ollama
```

No internal URLs or other details exposed. Overall status is `503` if any
target is down.

## Configuration

| Option | Env var | Default | Description |
|---|---|---|---|
| `port` | `ZOND_PORT` | `8080` | HTTP listen port |
| `targets[].name` | — | required | URL slug in `/health/<name>` |
| `targets[].url` | — | required | Internal URL to probe (Docker DNS or any) |
| `targets[].timeout` | — | `5000` | Request timeout in ms |

Config resolution order:
1. `ZOND_TARGETS` env var (default timeout applies to all)
2. `ZOND_CONFIG_PATH` env var → YAML file (supports `.yml` and `.yaml`)
3. `./zond.yml` in working directory (falls back to `./zond.yaml`)

## Docker

```bash
docker build -t ghcr.io/spy4x/zond:latest .
docker run --network proxy \
  -v $(pwd)/zond.yml:/app/zond.yml \
  ghcr.io/spy4x/zond:latest
```

## Compile (standalone)

```bash
deno task compile
./zond
```

## Development

```bash
deno task dev    # run locally
deno task check  # type-check
deno task lint   # lint
deno task fmt    # format
```

## Why not TCP checks?

TCP checks (`tcp://hl-metube:8081`) confirm a port is open. Zond performs
a real HTTP request and validates the response — catching cases where the
process is listening but returning 5xx errors.

## Why no authentication?

Zond returns only `ok` or `ko`. No data to protect, no session to steal,
no action to perform. Adding auth would reintroduce the exact problem Zond
solves. If you must, proxy it through your SSO — but the health endpoint
itself carries zero risk.

## License

MIT
