# AgentSAST

Static Application Security Testing (SAST) agent for the multi-agent API scanner.
Given a project's raw upload (source code + optional Swagger/OpenAPI) in MinIO, it
uses an LLM (via the Codex CLI) to:

1. **Enrich the OpenAPI spec** with constraints that only exist in the code
   (e.g. an `int quantity` clamped to `0..1000`; a `string id` that must be
   UUIDv7; the real `servers:` base URL + auth) → `<project_id>/sast/openapi.yaml`.
2. **Write a security report** of candidate vulnerabilities, attack surface,
   test cases and references → `<project_id>/sast/report.md`, to seed **AgentDAST**.

All the analysis know-how lives in an editable **Codex skill**
(`skills/sast/SKILL.md` + `skills/sast/references/`), not in Go code.

---

## API — the contract the Manager must follow

All routes are grouped under **`/api/sast`**. CORS is open so the static
`mock/manager.html` dashboard can call it from a browser.

| Method | Path                | Purpose |
|--------|---------------------|---------|
| GET    | `/health`           | liveness + dependency check |
| POST   | `/run`              | start a scan (async) |
| GET    | `/status?id=<uuid>` | poll job state / live activity |
| GET    | `/result?id=<uuid>` | fetch the result content |
| POST   | `/cancel?id=<uuid>` | cancel a running job |

### `GET /health`
```json
{ "agent": "AgentSAST", "ping": { "mysql": "ok", "minio": "ok", "llm": "ok" } }
```
`200` when healthy, `503` if MySQL or MinIO is down. `llm` is `ok` /
`not_configured`.

### `POST /run`
Request (both fields **required**):
```json
{ "project_id": "019ec9f9-5997-7739-877d-f8c73cce7582", "base_url": "http://localhost:8100/api/v1" }
```
- `project_id` — **UUIDv4** issued by the Manager; also the MinIO prefix.
- `base_url` — the Manager's best-guess base URL. It may be wrong or
  incomplete; Codex **verifies and corrects it** against the routing code
  (e.g. `.../api` → `.../api/v1`).

Response `202` (runs in the background):
```json
{ "id": "<job-uuid>", "project_id": "<uuid>", "status": "new" }
```
`400` if a field is missing.

### `GET /status?id=<uuid>`
```json
{
  "id": "<job-uuid>",
  "project_id": "<uuid>",
  "status": "process",
  "result_swagger_base_url": "https://api.example.com/api/v1",
  "last_message": "running: rg -n \"db.Exec\" ./raw/src",
  "last_update": "2026-06-15T03:20:43Z"
}
```
- **status lifecycle:** `new` → `process` → `done` | `failed` | `canceled`.
- `last_message` is the latest Codex emit (reasoning / shell command) while
  running, or the error message on failure. `last_update` is bumped on every
  Codex event — watch it as a live activity/heartbeat feed.
- `result_swagger_base_url` is filled once verified. `404` if id unknown.

### `GET /result?id=<uuid>`
Returns the deliverables' **content** (read live from MinIO), only when `done`:
```json
{ "swagger": "<openapi.yaml content>", "report": "<report.md content>" }
```
`409` if the job is not `done` yet.

### `POST /cancel?id=<uuid>`
```json
{ "id": "<job-uuid>", "status": "canceled", "signaled": true }
```
`409` if the job already finished.

---

## MinIO layout — input & output

The Manager only sends `project_id`; AgentSAST always reads/writes under that
prefix. The **input** layout is free-form — clean source tree, `.zip`,
`.tar.gz`, `.7z`, … — Codex extracts it itself. The **output** is always exactly
two objects.

```
<project_id>/                 # project_id = UUIDv4
├── raw/                       # INPUT  — whatever the Manager uploaded (any format)
│   └── source.zip             #          (or a folder, tarball, …)
└── sast/                      # OUTPUT — written by AgentSAST
    ├── openapi.yaml           #          enriched OpenAPI 3.x (servers + security + constraints)
    └── report.md              #          DAST-oriented vulnerability report
```

- Bucket name comes from `MINIO_BUCKET` (default `agentsdast`).
- `result_swagger_path` = `<project_id>/sast/openapi.yaml`,
  `result_report_path` = `<project_id>/sast/report.md`.

---

## MySQL — the `sast` table

Auto-migrated by GORM on startup.

```sql
CREATE TABLE sast (
  id                      VARCHAR(36)  PRIMARY KEY,   -- job id (UUIDv4)
  project_id              VARCHAR(36)  NOT NULL,      -- project id (UUIDv4), MinIO prefix
  result_swagger_path     VARCHAR(512),               -- <project_id>/sast/openapi.yaml
  result_report_path      VARCHAR(512),               -- <project_id>/sast/report.md
  result_swagger_base_url VARCHAR(512),               -- base URL Codex verified against the code
  status                  VARCHAR(16)  NOT NULL,      -- new | process | done | failed | canceled
  last_message            TEXT,                       -- latest Codex emit, or error message
  last_update             DATETIME     NOT NULL,      -- bumped on every Codex emit (heartbeat)
  INDEX idx_sast_project (project_id)
);
```

| column | meaning |
|--------|---------|
| `id` | job id (a UUIDv4 generated per `/run`) |
| `project_id` | the Manager's project UUIDv4; also the MinIO prefix |
| `result_swagger_path` / `result_report_path` | object keys of the two outputs (set when `done`) |
| `result_swagger_base_url` | the base URL Codex verified/corrected from `base_url` |
| `status` | `new` → `process` → `done`/`failed`/`canceled` |
| `last_message` | live Codex activity while running, or the error on failure |
| `last_update` | refreshed on every Codex event — the heartbeat the Manager polls |

---

## Configuration

`config.yaml` interpolates `${VARS}` from the environment. AgentSAST auto-loads
`../.env` (repo-root infra creds) then `./.env` (service-local, usually just the
LLM block); real environment variables win. See `.env.example`.

```yaml
server:
  port: "${SAST_PORT}"
  work_root: "${SAST_WORK_ROOT}"
mysql:
  dsn: "${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DATABASE}?..."
minio:
  endpoint: "${MINIO_ENDPOINT}"
  access_key: "${MINIO_ROOT_USER}"
  secret_key: "${MINIO_ROOT_PASSWORD}"
  bucket: "${MINIO_BUCKET}"
  use_ssl: ${MINIO_USE_SSL}
codex:
  home: "${CODEX_HOME}"
  llm_base_url: "${LLM_BASE_URL}"
  llm_api_key: "${LLM_API_KEY}"
  llm_effort: "${LLM_EFFORT}"
  llm_model: "${LLM_MODEL}"
```

| group | variable | purpose | default |
|-------|----------|---------|---------|
| **MySQL** | `MYSQL_HOST` / `MYSQL_PORT` | DB host/port | `127.0.0.1` / `3306` |
|       | `MYSQL_USER` / `MYSQL_PASSWORD` / `MYSQL_DATABASE` | credentials + db | from root `.env` |
| **MinIO** | `MINIO_ENDPOINT` | host:port | `127.0.0.1:9000` |
|       | `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | credentials | from root `.env` |
|       | `MINIO_BUCKET` | bucket | `agentsdast` |
|       | `MINIO_USE_SSL` | TLS | `false` |
| **Codex / LLM** | `LLM_BASE_URL` | OpenAI-compatible base URL | — (required) |
|       | `LLM_API_KEY` | API key (passed to Codex as `LLM_API_KEY`) | — (required) |
|       | `LLM_MODEL` | model id (e.g. `minimax/minimax-m2.5`) | — (required) |
|       | `LLM_EFFORT` | `model_reasoning_effort` | `high` |
|       | `CODEX_HOME` | where the generated `config.toml` + skill live | `/root/.codex` (container) |
| **Server** | `SAST_PORT` | HTTP port | `8081` |
|       | `SAST_WORK_ROOT` | per-job scratch dir | `$TMPDIR/agentsast` |

On startup AgentSAST writes a **dedicated** `CODEX_HOME` (never your personal
`~/.codex`): a `config.toml` (proxy provider → your LLM, `wire_api="responses"`)
and the `skills/sast/` tree. Per `/run` it downloads `<project_id>/raw` into a
workspace and runs `codex exec`, which invokes the `sast` skill to extract the
archive, analyze the code, verify the base URL, and write the two outputs. The
Codex CLI is bundled in the Docker image (`npm install -g @openai/codex`).

> Note: we deliberately do **not** use `codex exec --output-schema` — with this
> proxy/model, structured output short-circuits the agentic loop (no shell
> commands, no files). The skill writes the verified base URL to a sidecar
> `./sast/base_url.txt` which the runner reads.

---

## Quick start (Docker)

```bash
# from repo root
cp AgentSAST/.env.example AgentSAST/.env    # set LLM_BASE_URL / LLM_API_KEY / LLM_MODEL / LLM_EFFORT
docker-compose up -d mysql minio minio-init
docker-compose up -d --build agentsast
curl -s localhost:8081/api/sast/health      # {"ping":{"llm":"ok","minio":"ok","mysql":"ok"}}
```

Then drive a scan with the demo harness (see **`mock/README.md`** for the full
step-by-step):

```bash
./AgentSAST/mock/minio.sh                    # upload demo_project -> <uuid>/raw/source.zip
open AgentSAST/mock/manager.html             # Run ▶, watch the heartbeat, view results
```

---

## Layout

```
cmd/server                 HTTP entrypoint + wiring
config.yaml                config (env-interpolated)
skills/sast/SKILL.md       the SAST instructions (editable; installed into CODEX_HOME)
skills/sast/references/    per-class refs: idor, sqli, xss, payment
internal/config            load .env + config.yaml
internal/db                gorm open + Ping
internal/db/model          sast model
internal/db/repo           sast data access
internal/store             MinIO client (download prefix / upload / get text)
internal/service/codex     Codex runner, setup (config.toml + skill), event parsing
internal/service/job       orchestration (pull raw → codex → push sast) + cancel
internal/handler           Gin routes (route_*.go) + CORS
mock/                      demo harness (demo_project, minio.sh, manager.html, README.md)
```
