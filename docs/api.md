# API

## `GET /health/<name>`

| Response | Status | Meaning |
|---|---|---|
| `ok\n` | `200` | Target responded 2xx or 3xx |
| `unreachable\n` | `503` | Connection failed or 4xx/5xx |
| `unknown target: <name>\n` | `404` | Target not in config |

Zond does NOT follow redirects — the 3xx response itself is the contract.
A `302` from `/` to `/login` is healthy; `404` is not.

## `GET /` or `GET /health`

Returns one line per target, overall `200` if all healthy:

```
OK metube
KO grafana
OK ollama
```

No internal URLs or other details exposed. Overall status is `503` if any
target is down.
