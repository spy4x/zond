# Self-hosting

## Docker

```bash
docker run -p 8080:8080 \
  -v $(pwd)/zond.yml:/app/zond.yml \
  ghcr.io/spy4x/zond:latest

curl http://localhost:8080/health/metube
# ok
```

### Build the image yourself

```bash
docker build -t ghcr.io/spy4x/zond:latest .
docker run --network proxy \
  -v $(pwd)/zond.yml:/app/zond.yml \
  ghcr.io/spy4x/zond:latest
```

The container image includes a `HEALTHCHECK` that calls `zond -healthcheck`,
which connects to the listening socket. The compose healthcheck can remain a
pure HTTP probe (`wget --spider`) or be replaced with `CMD-SHELL`-less form —
either works.

## Docker Compose

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

## Compile (standalone)

```bash
go build -trimpath -ldflags="-s -w" -o zond ./cmd/zond
./zond
./zond -healthcheck && echo alive
```
