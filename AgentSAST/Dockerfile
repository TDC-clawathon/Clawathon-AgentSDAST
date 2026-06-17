# syntax=docker/dockerfile:1
# Build from repo root: docker compose build agent-sast
#
# AgentSAST now runs a pure-Go OpenAI-compatible tool loop (no Codex CLI, no
# node, no codex-security plugin). Skills arrive via the shared skills_data
# volume at /app/skills/sast (like agent-dast / agent-report).

# ---- build stage ----
FROM golang:1.26 AS build
WORKDIR /src/AgentSAST
COPY AgentSAST/go.mod AgentSAST/go.sum ./
RUN go mod download
COPY AgentSAST/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/agentsast ./cmd/server

# ---- runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget curl && adduser -D -u 10001 app
USER app
WORKDIR /app

COPY --from=build /out/agentsast /usr/local/bin/agentsast
COPY AgentSAST/config.yaml /app/config.yaml

EXPOSE 8001
ENV PORT=8001 \
    SAST_PORT=8001 \
    SKILLS_SAST_DIR=/app/skills/sast \
    SKILLS_SHARED_DIR=/app/skills/shared

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8001/health || exit 1

ENTRYPOINT ["agentsast"]
