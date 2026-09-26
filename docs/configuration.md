# Configuration

| Option | Env var | Default | Description |
|---|---|---|---|
| `port` | `ZOND_PORT` | `8080` | HTTP listen port |
| `targets[].name` | — | required | URL slug in `/health/<name>`, must be unique |
| `targets[].url` | — | required | Internal URL to probe (Docker DNS or any) |
| `targets[].timeout` | — | `5000` | Per-target probe timeout in **milliseconds** |

Config resolution (highest priority first):
1. `ZOND_TARGETS` env var (overrides targets entirely; default timeout applies; uses target name from `name=url` pairs)
2. `ZOND_CONFIG_PATH` env var → YAML file (supports `.yml` and `.yaml`)
3. `./zond.yml` in working directory (falls back to `./zond.yaml`)

For `port`: `ZOND_PORT` env var overrides the YAML `port` field. Only `ZOND_TARGETS`
short-circuits both — when it is set, the YAML file is not consulted at all.

Duplicate target names are rejected at load time (env or YAML).

## Config file

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

## Environment variables

```bash
docker run -p 8080:8080 \
  -e ZOND_TARGETS="metube=http://hl-metube:8081/,ollama=http://hl-ollama:11434/api/tags" \
  ghcr.io/spy4x/zond:latest
```

Note: env var targets use the default timeout (5000ms). For custom timeouts,
use a config file.
