# AgentDAST — Tutorial

A hands-on, progressive guide to AgentDAST. It starts with a one-line CLI scan and
ends with the full platform flow: another agent triggers a scan by `project_id` and
AgentDAST audits the live API and writes a report to object storage.

> ⚠️ **Authorization first.** AgentDAST sends real, potentially malicious payloads to
> the target and can disrupt a running service. Only scan systems you are explicitly
> authorized to test.

**Contents**

1. [What you'll learn](#1-what-youll-learn)
2. [Prerequisites](#2-prerequisites)
3. [Build it](#3-build-it)
4. [Your first scan (CLI)](#4-your-first-scan-cli)
5. [Scanning a whole spec & saving reports](#5-scanning-a-whole-spec--saving-reports)
6. [Targeting exactly what you want (insert points)](#6-targeting-exactly-what-you-want-insert-points)
7. [AI audit mode](#7-ai-audit-mode)
8. [MCP server (Claude Desktop / Cursor)](#8-mcp-server-claude-desktop--cursor)
9. [Server mode: the platform flow](#9-server-mode-the-platform-flow)
10. [Run the full flow locally (no real LLM)](#10-run-the-full-flow-locally-no-real-llm)
11. [Reading a report](#11-reading-a-report)
12. [Troubleshooting](#12-troubleshooting)
13. [Where to go next](#13-where-to-go-next)

---

## 1. What you'll learn

AgentDAST has one engine and several front-ends. By the end you'll be able to:

- run a precise single-endpoint scan and a full-spec scan from the CLI;
- have an LLM **auditor** verify SAST findings against the live API;
- expose the scanner to an MCP host (Claude Desktop, Cursor);
- run the **server** that other agents drive over a network by `project_id`.

The mental model throughout: **the core scanner is a precise instrument; everything
else (CLI, MCP, AI, server) is a different way to point it at a target.**

## 2. Prerequisites

- **Go 1.25+** (for building / CLI / MCP / AI modes).
- **Docker + Docker Compose** (only for server mode — Section 9–10).
- An **OpenAI-compatible LLM endpoint** (only for AI/server modes). Any provider that
  speaks the OpenAI chat-completions API works (hosted or local, e.g. LM Studio/Ollama).
- A **target API you are authorized to test**, reachable over HTTP(S).

## 3. Build it

```bash
cd AgentDAST
go build -o agentdast .
./agentdast --help
```

Every command shares two global flags: `--log-level {debug|info|warn|error}` and
`--log-format {text|json}` (or the `LOG_LEVEL` / `LOG_FORMAT` env vars). Logs go to
**stderr**, so stdout stays clean for reports and JSON. Use `--log-level debug` to see
every HTTP request the scanner makes.

List the vulnerability plugins:

```bash
./agentdast plugins                  # all
./agentdast plugins --category auth  # filter by category (injection|auth|exposure|config)
```

## 4. Your first scan (CLI)

The simplest case — point the scanner at one URL:

```bash
./agentdast scan --url "https://api.example.com/products/search?q=test" \
  --plugin sqli --plugin xss
```

What happens: AgentDAST builds a single endpoint from the URL, runs only the `sqli` and
`xss` plugins against the `q` parameter, and prints a colorized report. Drop `--plugin`
to run **all** plugins.

Give a path instead of a full URL by adding `--target` (the base URL):

```bash
./agentdast scan --url "/products/search?q=test" --target https://api.example.com
```

Scan a POST endpoint and fuzz specific JSON body fields:

```bash
./agentdast scan --url "/profile" --target https://api.example.com \
  --method POST --data '{"name":"x","email":"y"}' \
  --body-param name --body-param email
```

Add auth and custom params with `-H/--header` and `--param` (both repeatable):

```bash
./agentdast scan --url "/me" --target https://api.example.com \
  -H "Authorization: Bearer $TOKEN" --param tenant=acme
```

## 5. Scanning a whole spec & saving reports

Point `--swagger` at an OpenAPI 3 or Swagger 2.0 document (file path or URL) to scan
**every** endpoint it declares. A bundled example lives in
[examples/petstore.yaml](examples/petstore.yaml):

```bash
./agentdast scan --swagger ./examples/petstore.yaml \
  --target https://api.example.com \
  -H "Authorization: Bearer $TOKEN" \
  --output-mode full --output json --out results.json
```

- `--output-mode full` records every request/response exchange (not just findings).
- `--output {text|json|markdown}` selects the format; `--out FILE` writes to disk.

Re-render a saved JSON result into any format later, without re-scanning:

```bash
./agentdast report --file results.json --format markdown --out report.md
```

Useful `scan` knobs: `--timeout` (per-request seconds), `--concurrency` (parallel
workers), `--insecure` (skip TLS verify), `--follow-redirects`.

## 6. Targeting exactly what you want (insert points)

`--insert-point` controls **where** payloads go. Each entry is a bare `name` (matched in
any location) or `location:name` where location is `query|header|path|cookie|body`.
Multiple points are allowed, and a point that isn't in the spec is injected anyway — so
you can fuzz arbitrary headers or cookies:

```bash
# only the q query param
./agentdast scan --url "/search?q=x&page=1" --target https://api.example.com \
  --insert-point query:q

# a custom header that isn't declared in the swagger
./agentdast scan --url "/me" --target https://api.example.com \
  --insert-point header:X-Account-Id
```

## 7. AI audit mode

Here the LLM is the **auditor** and the scanner is its **instrument**. You hand it a
spec (for context), a SAST report (the claims to verify), and auth headers; it scans
specific endpoints to confirm or refute each claim, then writes a verdict.

```bash
export OPENAI_API_KEY=sk-...
./agentdast ai \
  --swagger ./swagger.yml \
  --target http://localhost:3000 \
  --sast-report ./sast-report.md \
  --context "Focus on authentication and BOLA" \
  --model gpt-4o \
  --base-url https://api.openai.com/v1 \
  -H "Authorization: Bearer $TOKEN" \
  --out audit-report.md
```

Key points:

- **`--target` is what makes scanning work** — it's the base URL of the *running* API.
  Many specs declare `servers: [{url: /}]` (e.g. OWASP Juice Shop) with no host; without
  `--target` the scanner has nowhere to send requests.
- A header passed once with `-H` authenticates **every** scan the auditor runs (baseline
  and each tool call).
- The auditor's tools: `get_knowledge` (reference for a vuln class), `scan_api` (run one
  plugin at one insert point → `VULNERABLE`/`NOT` + a `scan_id`), `get_scan_logs` (pull
  the exact exchanges when it doubts a result), `http_request` (fully custom request),
  `list_plugins`.
- The final report has two sections: **Auditor Analysis** (the model's narrative) and
  **Scanner-Confirmed Findings** (only what the scanner dynamically proved). Unverified
  SAST claims are never presented as confirmed.

**Weaker / local models** that can't tool-call reliably: add `--enable-mcp=false`. No
tools are sent, but the baseline auto-scan still runs and grounds the report:

```bash
./agentdast ai --swagger ./swagger.yml --target http://localhost:3000 \
  --sast-report ./sast-report.md \
  --base-url http://127.0.0.1:1234/v1 --model my-local-model --api-key none \
  --enable-mcp=false
```

Credentials and model can come from `OPENAI_API_KEY` / `OPENAI_BASE_URL` /
`OPENAI_MODEL` instead of flags.

## 8. MCP server (Claude Desktop / Cursor)

Expose the scanner to any MCP host over JSON-RPC 2.0:

```bash
./agentdast mcp              # stdio transport (Claude Desktop, Cursor, …)
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

The host then has four tools: `scan_api`, `list_plugins`, `get_scan_result`,
`get_knowledge`.

## 9. Server mode: the platform flow

Server mode is how AgentDAST runs inside the multi-agent platform. It is a long-lived
HTTP service, backed by **MySQL** (state) and **MinIO** (files in/out), that wraps the
same AI audit flow behind a REST API.

### The contract

AgentDAST does not receive files over the API — it receives a **`project_id`** and
looks everything else up. The division of responsibility:

```
 AgentSAST (upstream)                 AgentDAST (this service)
 ────────────────────                 ────────────────────────
 • generates the OpenAPI spec         • reads the sast row by project_id
   and a SAST report                  • downloads swagger + SAST report from MinIO
 • uploads them to MinIO              • runs the AI audit against the live API
 • writes a row in the `sast` table   • writes the report to MinIO
                                      • records state in the `dast` table
        Manager / any agent ── POST /api/scan {project_id} ──►
```

### MinIO layout (per project)

```
/<bucket>/
└── <project-id>/
    ├── raw/                 # original inputs (AgentSAST)
    ├── sast/
    │   ├── openapi.yaml     # spec   ← result_swagger_path
    │   └── report.md        # SAST   ← result_report_path
    └── dast/
        └── report.md        # DAST audit  ← written by AgentDAST
```

### The `sast` table (read by AgentDAST, owned by AgentSAST)

AgentDAST reads three columns, keyed by `project_id`:

| column                    | used as            | example                        |
|---------------------------|--------------------|--------------------------------|
| `result_swagger_base_url` | live API target    | `https://api.example.com`      |
| `result_swagger_path`     | MinIO swagger key  | `<project-id>/sast/openapi.yaml` |
| `result_report_path`      | MinIO SAST key     | `<project-id>/sast/report.md`  |

Column/table names are configurable (`SAST_TABLE`, `SAST_SWAGGER_COLUMN`,
`SAST_REPORT_COLUMN`, `SAST_BASEURL_COLUMN`, `SAST_PROJECT_COLUMN`).

### Endpoints

| Method | Path          | Params                                          | Returns                          |
|--------|---------------|-------------------------------------------------|----------------------------------|
| GET    | `/health`     | —                                               | `{"status":"ok"}` (503 if DB down)|
| POST   | `/api/scan`   | `project_id` (required); `base_url?`, `swagger_path?`, `sast_report_path?`, `prompt?` to override | `{"scan_id":"…"}` (202) |
| GET    | `/api/status` | `scan_id`                                       | `{status, result_path \| error}` |

### What a scan does

```
POST /api/scan {project_id}
   → insert dast row (status=new), return scan_id (202)
   → async worker:
       status=processing
       → resolve base_url + swagger key + SAST report key from the sast table
       → download swagger + SAST report from MinIO
       → run the AI audit (base_url = live target, SAST report grounds it, prompt = guidance)
       → upload markdown to MinIO at <project_id>/dast/report.md
       → status=done, result_path=<key>     (or status=fail, error=<reason>)
```

The SAST report is supplementary — if its key is missing, the audit proceeds
dynamically. A missing `sast` row, swagger key, or base URL fails the scan and the
reason is recorded on the `dast` row (visible via `/api/status`).

### Networking

The service is **internal to the compose network** — no host port is published. Sibling
agents reach it at `http://agentdast:8080`. (To expose it for local debugging, add a
`ports:` mapping in a compose override file.)

## 10. Run the full flow locally (no real LLM)

The repo ships a deterministic end-to-end harness so you can exercise the whole platform
flow — without a real LLM or target — and watch a report land at
`<project>/dast/report.md`. It adds a mock that plays both the OpenAI endpoint and the
scan target.

From the **repo root** (the parent of `AgentDAST/`):

```bash
cp .env.example .env        # the harness sets test values; for real runs, fill OPENAI_*

# Bring up mysql + minio + agentdast + the mock, all on the compose network
docker compose -f docker-compose.yml -f docker-compose.test.yml up -d --build

# Seed the sast table + MinIO (as AgentSAST would), trigger a scan as another
# agent (curl container on the network), poll status, and verify the report:
bash test/e2e.sh
```

`test/e2e.sh` asserts each step and ends with:

```
E2E RESULT: PASS — status=done, report at proj-e2e/dast/report.md
```

It proves, in order: the service is reachable on the network, **not** published to the
host, resolves `base_url`/`swagger`/`report` from the `sast` table by `project_id`,
downloads both from MinIO, runs the pipeline to `done`, and writes
`proj-e2e/dast/report.md`.

For a **real** deployment, set real `OPENAI_*` values in `.env` and run without the test
override:

```bash
docker compose up -d --build
# reach it from inside the network (no host port):
docker compose exec agentdast wget -qO- http://agentdast:8080/health
```

Files involved: [../docker-compose.yml](../docker-compose.yml),
[../docker-compose.test.yml](../docker-compose.test.yml),
[../test/e2e.sh](../test/e2e.sh), [../test/mock-openai.js](../test/mock-openai.js),
[../test/seed/](../test/seed/).

## 11. Reading a report

Every report (CLI markdown, `ai` mode, server mode) follows the same shape:

```
# AI Security Audit Report
| Spec | … |
| Scans executed | N (M HTTP requests) |
| Scanner-confirmed findings | K (🔴 / 🟠 / 🟡 / 🔵) |

**Overall risk: …**

## Auditor Analysis          ← narrative (AI mode); may cite SAST claims
## Scanner-Confirmed Findings ← ONLY what the scanner dynamically proved (deduplicated)
```

The split is deliberate: a reader can always tell what was **verified against the
running API** versus what was **inferred from static analysis**. Each confirmed finding
carries `Evidence` (a request/response snippet) and a `Confidence`
(`confirmed`/`probable`/`possible`), so every claim is backed by what happened on the
wire.

## 12. Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| "provide --url or --swagger" | `scan` needs one of them. |
| Scanner sends nothing / all errors | No reachable target. Pass `--target` (or set a valid `base_url`). |
| `ai`: "chat completion failed" on turn 0 | Bad `--base-url`/`--model`/`--api-key`, or the model can't tool-call → try `--enable-mcp=false`. |
| Everything looks "echoed" but no findings | Working as intended — reflected payloads are stripped before matching to avoid false positives. |
| Server `/api/scan` → 503 | AI not configured: set `OPENAI_API_KEY` and `OPENAI_MODEL`. |
| Server scan `status=fail` "resolve project inputs" | No `sast` row for that `project_id`, or wrong `SAST_*` column mapping. |
| `curl localhost:8080` fails after `docker compose up` | Expected — the service is internal-only. Reach it on the network (Section 9). |
| Scan confirms nothing | A negative result is not proof of safety; inspect `--output-mode full` logs or, in AI mode, let the auditor re-test. |

## 13. Where to go next

- **[ARCHITECTURE.md](ARCHITECTURE.md)** — how the engine, plugins, AI auditor, and
  server fit together, and the accuracy model behind trustworthy findings.
- **[README.md](README.md)** — feature reference and the full flag list.
- **Extend it** — add a vulnerability plugin: implement `core.Plugin`, register it in
  `internal/plugins/all.go`, and drop an `internal/knowledge/<name>.md`. It's then
  available to CLI, MCP, AI, and server modes automatically. See ARCHITECTURE.md →
  *Extending: add a vulnerability plugin*.
