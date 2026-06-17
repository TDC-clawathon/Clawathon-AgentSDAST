# syntax=docker/dockerfile:1
# Build from repo root: docker compose build manager

FROM node:22-alpine

WORKDIR /app

COPY Manager/package.json Manager/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --omit=dev

COPY Manager/ .

# Bundled skill seeds (uploaded to MinIO on startup if missing)
COPY skills/sast/ /app/skills-seed/sast/
COPY skills/dast/ /app/skills-seed/dast/
COPY skills/report/ /app/skills-seed/report/

EXPOSE 8000

CMD ["npm", "start"]
