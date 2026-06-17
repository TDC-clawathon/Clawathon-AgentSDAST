# syntax=docker/dockerfile:1
# AgentBase Custom Agent — MySQL data plane (health on :8080, MySQL on :3306).
# Build: docker compose -f docker-compose.agentbase.yml build runtime-mysql

FROM mysql:8.4

USER root
RUN microdnf install -y python3 curl \
  && microdnf clean all

COPY runtimes/shared/health_proxy.py /health_proxy.py
COPY runtimes/mysql/entrypoint.sh /agentsdast-entrypoint.sh
RUN chmod +x /agentsdast-entrypoint.sh /health_proxy.py

EXPOSE 8080 3306

HEALTHCHECK --interval=15s --timeout=5s --start-period=60s --retries=10 \
  CMD curl -sf http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/agentsdast-entrypoint.sh"]
