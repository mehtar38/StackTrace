# ─── Base ────────────────────────────────────────────────────────────────────
FROM node:24-slim

RUN groupadd -r stacktrace && useradd -r -g stacktrace stacktrace

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl jq ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# ─── Dependencies ─────────────────────────────────────────────────────────────
# Copy the entire Express repo — the example requires the parent package
COPY app/express/package*.json ./
RUN npm install --omit=dev

# ─── App code ─────────────────────────────────────────────────────────────────
COPY app/express/ .

RUN chown -R stacktrace:stacktrace /app

# ─── Security ─────────────────────────────────────────────────────────────────
USER stacktrace

EXPOSE 3000
CMD ["node", "examples/web-service/index.js"]