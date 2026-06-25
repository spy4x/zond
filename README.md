# Zond 🚀

[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Fspy4x%2Fzond-blue)](https://github.com/spy4x/zond/pkgs/container/zond)
[![Deno](https://img.shields.io/badge/deno-2.2-black?logo=deno)](https://deno.com)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**Zond** (Зонд — Russian: "probe") is a tiny internal health probe bridge.
It receives external health check requests from monitoring systems
like [Gatus](https://github.com/TwiN/gatus) and forwards them to internal
containers via Docker DNS names — no authentication required, no sensitive
data exposed.

```
┌──────────┐     ┌──────────┐     ┌─────────────┐
│  Gatus   │────▶│  Zond    │────▶│ hl-metube   │
│ (cloud)  │     │ (home)   │     │ :8081       │
└──────────┘     └──────────┘     └─────────────┘
                      │
                      ├──▶ hl-ollama:11434/api/tags
                      ├──▶ hl-grafana:3000/api/health
                      └──▶ hl-transmission:9091
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

Compare to the alternative — managing a dedicated monitoring user, TOTP,
and per-subject access control rules for each service.

## Features

- **Zero attack surface** — returns `200` or `503`, no data, no tokens
- **Single endpoint per service** — `GET /health/<name>`
- **Bulk check** — `GET /` or `GET /health` lists all targets
- **Config-driven** — YAML file or `ZOND_TARGETS` env var
- **Docker-native** — 30MB Deno image, no dependencies
- **Standalone binary** — `deno task compile` for bare-metal

## Quick start

```yaml
# zond.yaml
port: 8080
targets:
  - name: metube
    url: http://hl-metube:8081/
  - name: ollama
    url: http://hl-ollama:11434/api/tags
```

```bash
docker run -p 8080:8080 \
  -v $(pwd)/zond.yaml:/app/zond.yaml \
  ghcr.io/spy4x/zond:latest

curl http://localhost:8080/health/metube
# ok
```

### Via env vars

```bash
docker run -p 8080:8080 \
  -e ZOND_TARGETS="metube=http://hl-metube:8081/,ollama=http://hl-ollama:11434/api/tags" \
  ghcr.io/spy4x/zond:latest
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
OK metube http://hl-metube:8081/
OK ollama http://hl-ollama:11434/api/tags
DOWN grafana http://hl-grafana:3000/api/health
```

Overall status is `503` if any target is down.

## Configuration

| Option | Env var | Default | Description |
|---|---|---|---|
| `port` | `ZOND_PORT` | `8080` | HTTP listen port |
| `targets[].name` | — | required | URL slug in `/health/<name>` |
| `targets[].url` | — | required | Internal URL to probe (Docker DNS or any) |
| `targets[].timeout` | — | `5000` | Request timeout in ms |

Config resolution order:
1. `ZOND_TARGETS` env var
2. `ZOND_CONFIG_PATH` env var → YAML file
3. `./zond.yaml` in working directory

## Docker

```bash
docker build -t ghcr.io/spy4x/zond:latest .
docker run --network proxy \
  -v $(pwd)/zond.yaml:/app/zond.yaml \
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

Zond returns only `ok` or `unreachable`. No data to protect, no session
to steal, no action to perform. Adding auth would reintroduce the exact
problem Zond solves. If you must, proxy it through your SSO — but the
health endpoint itself carries zero risk.

## License

MIT
