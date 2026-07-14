# Build stage — pinned for reproducibility
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache deps separately from source for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static, stripped, fully static binary. CGO disabled for portability.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/zond \
    ./cmd/zond

# Runtime stage — distroless for minimal attack surface (~10MB).
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/zond /usr/local/bin/zond

ENV ZOND_PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/zond"]