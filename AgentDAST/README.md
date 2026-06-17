# AgentDAST

A plugin-based **Dynamic Application Security Testing (DAST)** scanner for REST APIs,
written in Go. It takes an OpenAPI/Swagger spec (file or URL), fuzzes every endpoint
and parameter through a set of vulnerability plugins, and reports findings. It runs as
a **CLI**, an **MCP server**, or an **AI-orchestrated auditor** that verifies SAST
findings against the live API.

> **New here?** Start with the **[TUTORIAL.md](TUTORIAL.md)** — a hands-on walkthrough
> from a one-line CLI scan to the full platform flow. This README is the feature/flag
> reference; **[ARCHITECTURE.md](ARCHITECTURE.md)** explains the design.

## Features

- **Core scanner engine** with a fan-out worker pool (endpoint × plugin).
- **Plugin architecture** — each vulnerability class is an independent plugin.
- **Input**: OpenAPI 3 or Swagger 2.0 (auto-converted), from a file path or URL.
- **Custom headers & params**, per-run, injected into every request.
- **Selective scanning** — choose plugins, parameters, and headers, or run everything.
- **Output modes**: `results` (findings only) or `full` (every request/response log).
- **Output formats**: `text` (colorized), `json`, `markdown`.
- **Three run modes**: CLI, MCP (stdio/TCP), and AI audit.
- **Optional persistence** to MySQL (metadata) + MinIO (gzipped request logs).

## Build

```bash
cd AgentDAST
go build -o agentdast .
```

## Logging

All commands share structured, leveled logging (via `log/slog`) on **stderr**, so stdout
stays clean for reports/JSON. Control it with global flags or env vars:

```bash
agentdast scan ... --log-level debug --log-format json   # or LOG_LEVEL / LOG_FORMAT
```

`--log-level debug` shows every HTTP request (method, url, status, ms), per-plugin
activity, and AI turns/tool-calls; the server emits structured access logs and per-scan
lifecycle events tagged with `scan_id` and `project_id`.

## Vulnerability Plugins

| Plugin            | Category   | Severity | Detects                                                        |
|-------------------|------------|----------|----------------------------------------------------------------|
| `sqli`            | injection  | critical | SQL injection: error-based, boolean-based blind, time-based blind |
| `cmdi`            | injection  | critical | OS command injection via command-output signatures             |
| `ssrf`            | injection  | critical | SSRF on URL-like params — confirms the resource was fetched    |
| `xss`             | injection  | high     | Reflected XSS — unescaped reflection in an HTML response       |
| `path_traversal`  | injection  | high     | Directory traversal to system files                            |
| `xxe`             | injection  | high     | XML External Entity processing                                 |
| `idor`            | auth       | high     | IDOR/BOLA candidates on secured endpoints (needs 2-account verify) |
| `auth`            | auth       | high     | Secured endpoints reachable without credentials                |
| `mass_assignment` | injection  | high     | Privileged-field auto-binding (sentinel-value confirmed)       |
| `ssti`            | injection  | critical | Server-Side Template Injection (evaluates a distinctive expr)  |
| `sensitive_data`  | exposure   | medium   | High-signal secret leakage (keys, cards, SSNs) in responses    |
| `cors`            | config     | medium   | Permissive CORS configuration                                  |
| `open_redirect`   | config     | medium   | Unvalidated redirect via a user-controlled target parameter    |

Injection-class plugins try multiple **bypass/encoding techniques** (WAF-bypass SQL
meta-characters, double/overlong-encoded traversal, alternate-IP-encoding SSRF, multi-
context XSS polyglots, newline/substitution command separators) and strip the reflected
payload before matching, so an echoed input never counts as a hit.

**Accuracy:** all signature-based plugins strip the reflected payload from the response
before matching, so an endpoint that merely echoes input is **not** flagged (the common
SSRF/XSS false positive). SQLi uses error + boolean + time-based techniques with a
baseline comparison. XSS requires an HTML content type and unescaped reflection. IDOR
only flags *secured* endpoints whose objects are enumerable.

```bash
./agentdast plugins                  # list all
./agentdast plugins --category auth  # filter by category
```

## CLI Usage

A scan targets **a single endpoint** (`--url`) or a **whole spec** (`--swagger`).

```bash
# Scan a single endpoint (full URL) — the common DAST case, just give it a URL
./agentdast scan --url "https://api.example.com/products/search?q=test" --plugin sqli --plugin xss

# Single endpoint by path against a base target
./agentdast scan --url "/products/search?q=test" --target https://api.example.com

# POST endpoint with a JSON body, fuzzing specific body fields
./agentdast scan --url "/profile" --target https://api.example.com \
  --method POST --data '{"name":"x","email":"y"}' --body-param name --body-param email

# Scan every endpoint in a spec
./agentdast scan --swagger ./examples/petstore.yaml --target https://api.example.com \
  -H "Authorization: Bearer $TOKEN" --plugin sqli --plugin xss \
  --output-mode full --output json --out results.json

# Render a report from a saved result
./agentdast report --file results.json --format markdown --out report.md
```

Key `scan` flags: `--url` / `--swagger` (one required), `--target` (base URL, also
resolves a path `--url`), `--method`, `--data`, `--body-param`, `-H/--header`,
`--param`, `--plugin`, `--insert-point`, `--output-mode {results|full}`,
`--output {text|json|markdown}`, `--out`, `--timeout`, `--concurrency`,
`--insecure`, `--follow-redirects`.

### Insert points

`--insert-point` controls **where** payloads are injected. Each entry is a bare name
(matched in any location) or a `location:name` where location is one of
`query|header|path|cookie|body`. Multiple points are supported, and a point not declared
in the spec is injected anyway — so you can target arbitrary headers/cookies:

```bash
# fuzz only the q query param
agentdast scan --url "/search?q=x&page=1" --target https://api.example.com --insert-point query:q

# fuzz two params
agentdast scan --swagger api.yaml --target https://api.example.com --insert-point a --insert-point b

# fuzz a custom header that isn't in the swagger
agentdast scan --url "/me" --target https://api.example.com --insert-point header:X-Account-Id
```

## MCP Server

Exposes four tools — `scan_api`, `list_plugins`, `get_scan_result`, `get_knowledge` —
over MCP JSON-RPC 2.0.

```bash
./agentdast mcp              # stdio (Claude Desktop, Cursor, …)
./agentdast mcp --tcp :8765  # TCP transport
```

Example Claude Desktop config:

```json
{
  "mcpServers": {
    "agentdast": { "command": "/path/to/agentdast", "args": ["mcp"] }
  }
}
```

## AI Audit Mode

The swagger spec is given to the model **for context** (to understand the API). The
model then calls `scan_api` on **specific endpoints** to verify SAST findings and probe
new ones, against the live API at `--target`. Works with any OpenAI-compatible endpoint.

```bash
export OPENAI_API_KEY=sk-...
./agentdast ai \
  --swagger ./swagger.yml \
  --target http://localhost:3000 \
  --sast-report ./sast-report.md \
  --context "Focus on authentication and BOLA" \
  --model gpt-4o \
  --base-url https://api.openai.com/v1 \
  --out audit-report.md
```

**`--target` is what makes scanning work.** It is the base URL of the *running* API.
Many specs declare `servers: [{url: /}]` (e.g. OWASP Juice Shop), which has no host —
without `--target` the scanner has nowhere to send requests. Always pass the live URL.

How it runs (the AI is the **auditor**, the core scan is its **instrument**):
1. **Baseline scan** (`--auto-scan`, default on): before the conversation, every spec
   endpoint is scanned against `--target` and the findings are handed to the model.
   This grounds the report even when the model can't call tools well.
2. For each endpoint / SAST test case the model runs an **iterative verify loop** with
   these tools:
   - **`get_knowledge(vuln)`** — pull reference knowledge for the class (how to test,
     confirm, avoid false positives). The knowledge base lives in
     [internal/knowledge/](internal/knowledge/) as one markdown file per vulnerability;
     if a class has no file, the model uses its own expertise.
   - **`scan_api`** — run one plugin at one `insert_point` to confirm/refute. The result
     is a crisp `VULNERABLE` / `NOT VULNERABLE` verdict (with a `scan_id`).
   - **`get_scan_logs(scan_id)`** — if the model doubts a result, it pulls the exact
     request/response exchanges the scan made, combines them with the knowledge, crafts
     its own payload, and re-tests (`scan_api` or `http_request`) — repeating until it is
     confident the verdict is right.
   - **`http_request`** — any fully custom request (auth/login flows, business logic,
     hand-crafted payloads).
   It is not allowed to write the final report until every SAST test case has been
   verified against the live API.
3. The model decides when it is done (it stops calling tools). `--max-turns` (default
   **300**) is only a safety backstop; if reached, the tool forces a final report instead
   of a placeholder.
4. The final report has two clearly separated sections: **Auditor Analysis** (the model's
   narrative) and **Scanner-Confirmed Findings** (only what the scanner dynamically
   proved, consolidated and deduplicated). Unverified SAST claims are not presented as
   confirmed.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design.

Flags: `--target`, `--sast-report`, `--context`, `--model`, `--api-key`, `--base-url`,
`--max-turns` (default 300), `--enable-mcp` (expose the scan tool to the model, default
true), `--auto-scan` (baseline scan, default true). Credentials/model may also come from
`OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`.

### Vulnerability knowledge base

The folder [internal/knowledge/](internal/knowledge/) holds one markdown file per
vulnerability class (`idor.md`, `sqli.md`, `ssrf.md`, …) describing how to test it, how to
confirm it, common false positives, and remediation. The files are embedded into the
binary and exposed through the `get_knowledge` tool (AI mode and MCP). The auditor calls it
to ground its testing; if a class has no file it falls back to its own expertise. Add a new
`<name>.md` to extend the base — no rebuild wiring needed beyond recompiling.

### Local / weaker models

Some local models (LM Studio, Ollama) don't reliably support tool/function calling. If
the model errors when tools are sent, run with `--enable-mcp=false`: no tools are sent,
the **baseline auto-scan** still runs, and the model writes a grounded report from those
results plus the SAST report.

```bash
./agentdast ai --swagger ./swagger.yml --target http://localhost:3000 \
  --sast-report ./sast-report.md \
  --base-url http://127.0.0.1:1234/v1 --model my-local-model --api-key none \
  --enable-mcp=false
```

## Server Mode (HTTP API)

Run AgentDAST as a long-lived service that drives the AI audit flow from a REST API,
backed by MySQL (state) and MinIO (swagger in / report out). This is the mode used in
the parent `docker-compose.yml`.

```bash
agentdast serve            # listens on :8080 (PORT env), config from environment
```

### Endpoints

| Method | Path           | Params                                   | Returns                                   |
|--------|----------------|------------------------------------------|-------------------------------------------|
| GET    | `/health`      | —                                        | `{"status":"ok"}` (503 if DB unreachable) |
| POST   | `/api/scan`    | `project_id` (required), `base_url?`, `swagger_path?`, `sast_report_path?`, `prompt?` | `{"scan_id":"<uuid>"}` (202)              |
| GET    | `/api/status`  | `scan_id`                                | `{"status", "result_path" | "error", ...}`|

**`project_id` is the only field you need to send.** The service looks it up in the
`sast` table and reads the project's target base URL (`result_swagger_base_url`),
swagger key (`result_swagger_path`), and SAST report key (`result_report_path`) from
there. It then downloads the swagger and SAST report from MinIO by those keys — files
are **never** sent over the API. `base_url` / `swagger_path` / `sast_report_path` may
still be passed in the request to **override** the corresponding column.

```bash
# Start a scan — project_id is enough; everything else is read from the sast table
curl -X POST localhost:8080/api/scan \
  -H 'Content-Type: application/json' \
  -d '{"project_id":"proj-123","prompt":"focus on auth"}'
# -> {"scan_id":"7f3c..."}

# Poll status
curl "localhost:8080/api/status?scan_id=7f3c..."
# -> {"scan_id":"7f3c...","status":"done","result_path":"proj-123/dast/report.md", ...}
```

### Flow

```
POST /api/scan(project_id, [base_url?, swagger_path?, sast_report_path?, prompt?])
   │  insert dast row (status=new) → return scan_id (202)
   └─ async worker:
        status=processing
        → look up project_id in the `sast` table → base_url + swagger key + SAST report key
          (each overridable by the matching request field)
        → download swagger + SAST report from MinIO
        → run the AI audit (base_url = live target, SAST report grounds it, prompt = guidance)
        → upload the markdown report to MinIO at <project_id>/dast/report.md
        → status=done, result_path=<minio key>     (or status=fail, error=<reason>)
```

The SAST report is supplementary: if no key is supplied or found, the audit proceeds
dynamically without it. A missing `sast` row, missing swagger key, or missing base URL
fails the scan (recorded with the reason on the `dast` row).

### `dast` table

The service owns and auto-creates this table:

| column        | type         | notes                                   |
|---------------|--------------|-----------------------------------------|
| `id`          | varchar(36)  | scan id (uuid)                          |
| `project_id`  | varchar(255) | from the request                        |
| `result_path` | varchar(512) | MinIO object key of the report          |
| `status`      | varchar(16)  | `new` → `processing` → `done` \| `fail` |
| `error_msg`   | text         | failure reason (added; surfaced on fail)|
| `last_update` | datetime     | last state change                       |

> `error_msg` is added beyond the requested columns so a failed scan can report *why* it
> failed via `/api/status` — the original column set had nowhere to put the reason.

### Configuration (environment)

`PORT`, `MYSQL_HOST/PORT/USER/PASSWORD/DATABASE`, `MINIO_ENDPOINT/ACCESS_KEY/SECRET_KEY/USE_SSL/BUCKET`,
`OPENAI_BASE_URL/API_KEY/MODEL`, `AI_MAX_TURNS`, `MAX_CONCURRENT_SCANS`, the SAST
lookup mapping `SAST_TABLE` / `SAST_SWAGGER_COLUMN` / `SAST_REPORT_COLUMN` /
`SAST_BASEURL_COLUMN` / `SAST_PROJECT_COLUMN` (defaults `sast` / `result_swagger_path` /
`result_report_path` / `result_swagger_base_url` / `project_id`; set `SAST_REPORT_COLUMN`
or `SAST_BASEURL_COLUMN` empty to disable that lookup), and the report output location
`DAST_REPORT_DIR` / `DAST_REPORT_FILE` (defaults `dast` / `report.md`, written as
`<project_id>/dast/report.md`). See `.env.example` in the repo root.

### Docker

```bash
# from the repo root (parent of AgentDAST/)
cp .env.example .env      # fill in OPENAI_API_KEY, OPENAI_MODEL, passwords
docker compose up -d --build

# The service is internal to the compose network (no host port is published), so
# reach it the way a sibling agent does — from inside the network:
docker compose exec agentdast wget -qO- http://agentdast:8080/health
# or with a throwaway container on the same network:
docker compose run --rm agentdast wget -qO- http://agentdast:8080/health
```

The `Dockerfile` builds a static binary into a minimal Alpine image with a `/health`
HEALTHCHECK. The compose service waits for MySQL + MinIO to be healthy before starting.
It is **not** published to the host: sibling agents (Manager, AgentSAST) call it at
`http://agentdast:8080` on the `agentsdast` network. To expose it for local debugging,
add a `ports:` mapping in an override file.

## Persistence (optional)

With no env vars set, results live in memory. Set these to persist (the repo's
`docker-compose.yml` provides both services):

- MySQL: `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DB`
- MinIO: `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_BUCKET`

The MySQL store keeps scan metadata + findings; full request logs are gzipped into MinIO.

## Architecture

```
cmd/                CLI commands (scan, plugins, report, mcp, ai)
pkg/types/          shared data models (ScanConfig, Finding, RequestLog, ScanResult)
internal/core/      scanner engine, plugin interface + registry, HTTP executor
internal/parser/    OpenAPI/Swagger loader + endpoint extractor
internal/plugins/   the 13 vulnerability plugins
internal/toolexec/  shared bridge: tool args → scanner → result (used by MCP + AI)
internal/mcp/       MCP JSON-RPC 2.0 server
internal/ai/        AI orchestration (OpenAI-compatible tool-calling loop)
internal/output/    text / json / markdown formatters
internal/storage/   in-memory / MySQL / MinIO backends
config/             YAML config loader with env overrides
```

### Writing a plugin

Implement `core.Plugin` and register it in `internal/plugins/all.go`:

```go
type Plugin interface {
    Name() string
    Description() string
    Category() string
    Severity() types.Severity
    DefaultPayloads() []string
    Test(ctx *core.ScanContext) []types.Finding
}
```

Inside `Test`, iterate `ctx.Params()` and call `ctx.Inject(param, payload)` (or
`ctx.Baseline()` for unmodified requests), then inspect the returned `RequestLog`.

## Legal

Only scan systems you are authorized to test. This tool sends potentially malicious
payloads and can disrupt running services.
