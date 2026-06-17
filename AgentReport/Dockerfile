# syntax=docker/dockerfile:1
# Build from repo root: docker compose build agent-report

FROM node:22-alpine

WORKDIR /app

# Chromium + fonts for server-side HTML -> PDF rendering (puppeteer-core).
RUN apk add --no-cache \
      chromium \
      nss \
      freetype \
      harfbuzz \
      ca-certificates \
      ttf-freefont

COPY AgentReport/package.json ./
# Skip puppeteer's own Chromium download — we use the system chromium above.
ENV PUPPETEER_SKIP_DOWNLOAD=true
RUN npm install --omit=dev

COPY AgentReport/ ./
COPY skills/report /app/skills/report

EXPOSE 8003
ENV PORT=8003 \
    SKILLS_REPORT_DIR=/app/skills/report \
    PUPPETEER_SKIP_DOWNLOAD=true \
    PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser

HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8003/health || exit 1

CMD ["npm", "start"]
