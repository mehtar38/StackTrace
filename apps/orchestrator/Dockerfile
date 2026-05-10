# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# ca-certificates: needed for HTTPS calls (Clerk JWKS, Upstash, Supabase)
# git: needed by `go mod download` for some indirect deps
RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Download dependencies first (cached layer if go.mod/go.sum unchanged)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o orchestrator ./cmd/orchestrator

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: HTTPS calls at runtime
# docker-cli: used by LocalDockerProvider to talk to the host Docker daemon.
#             Mounted as /var/run/docker.sock in docker-compose.
RUN apk add --no-cache ca-certificates docker-cli

WORKDIR /app

COPY --from=builder /build/orchestrator .

# The orchestrator reads challenge Dockerfiles at runtime (for image builds).
# In local dev, /challenges is bind-mounted from the repo root.
# In production, challenge images are pre-built and pushed to ACR — this mount
# is not needed.
ENV CHALLENGES_DIR=/challenges

EXPOSE 8080

# Run as non-root user for defence in depth
RUN addgroup -S orchestrator && adduser -S orchestrator -G orchestrator
USER orchestrator

ENTRYPOINT ["/app/orchestrator"]