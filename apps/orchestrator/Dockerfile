# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.26 AS builder

# ca-certificates: needed for HTTPS calls (Clerk JWKS, Upstash, Supabase)
# git: needed by `go mod download` for some indirect deps
# RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Download dependencies first (cached layer if go.mod/go.sum unchanged)
COPY apps/orchestrator/go.mod apps/orchestrator/go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY apps/orchestrator/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o orchestrator ./cmd/orchestrator

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM debian:12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates docker.io curl \
    && rm -rf /var/lib/apt/lists/*

RUN curl -sSL "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o /tmp/ssm.deb \
    && dpkg -i /tmp/ssm.deb \
    && rm /tmp/ssm.deb

WORKDIR /app

COPY --from=builder /build/orchestrator .
COPY challenges /challenges
ENV CHALLENGES_DIR=/challenges

EXPOSE 8080

RUN groupadd -r orchestrator && useradd -r -g orchestrator orchestrator
USER orchestrator

ENTRYPOINT ["/app/orchestrator"]