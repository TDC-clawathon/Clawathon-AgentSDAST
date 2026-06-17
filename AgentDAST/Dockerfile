# syntax=docker/dockerfile:1
# Build from repo root: docker compose build agent-dast

# ---- build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src/AgentDAST

COPY AgentDAST/go.mod AgentDAST/go.sum ./
RUN go mod download

COPY AgentDAST/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/agentdast .

# ---- runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget curl && adduser -D -u 10001 app
USER app
WORKDIR /app

COPY --from=build /out/agentdast /usr/local/bin/agentdast
COPY skills/dast /app/skills/dast

EXPOSE 8002
ENV PORT=8002 \
    SKILLS_DAST_DIR=/app/skills/dast

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8002/health || exit 1

ENTRYPOINT ["agentdast"]
CMD ["serve"]
