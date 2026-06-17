# syntax=docker/dockerfile:1
# AgentBase Custom Agent — MinIO data plane (health on :8080, S3 API on :9000).

FROM minio/minio:latest AS minio-bin
FROM minio/mc:latest AS mc-bin

FROM alpine:3.20

RUN apk add --no-cache curl python3 ca-certificates

COPY --from=minio-bin /usr/bin/minio /usr/bin/minio
COPY --from=mc-bin /usr/bin/mc /usr/bin/mc
RUN chmod +x /usr/bin/minio /usr/bin/mc

COPY runtimes/shared/health_proxy.py /health_proxy.py
COPY runtimes/minio/entrypoint.sh /agentsdast-entrypoint.sh
RUN chmod +x /agentsdast-entrypoint.sh /health_proxy.py

VOLUME ["/data"]
EXPOSE 8080 9000 9001

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=8 \
  CMD curl -sf http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/agentsdast-entrypoint.sh"]
