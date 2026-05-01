# ─── Base ────────────────────────────────────────────────────────────────────
FROM node:18-alpine

RUN addgroup -S stacktrace && adduser -S stacktrace -G stacktrace

RUN apk add --no-cache curl jq

WORKDIR /app

# ─── Dependencies ─────────────────────────────────────────────────────────────
# Copy the entire Express repo — the example requires the parent package
COPY app/express/package*.json ./
RUN npm install

# ─── App code ─────────────────────────────────────────────────────────────────
COPY app/express/ .

RUN chown -R stacktrace:stacktrace /app

# ─── Security ─────────────────────────────────────────────────────────────────
USER stacktrace

EXPOSE 3000

# Run from repo root so require('../../') resolves correctly
# This points to the example which is where the bug lives
CMD ["node", "examples/web-service/index.js"]