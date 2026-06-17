# AgentDAST — Architecture

AgentDAST is a plugin-based DAST scanner for REST APIs. It exists at two levels:

1. **The core scanner** — a precise, stable instrument. Given *one endpoint*, *one
   parameter*, and *one vulnerability class*, it answers: **vulnerable or not, with
   evidence.** It is deterministic, has no LLM dependency, and is the unit of work.
2. **The AI auditor** — an LLM that *uses* the core scanner as a tool. The user hands it
   a swagger spec, a SAST report, and auth headers; the auditor verifies each reported
   test case against the live API by calling the scanner, then writes a final audit
   verdict for the user.

> Mental model: **the AI is the auditor; the core scan is the instrument the auditor
> uses.** The scanner must be reliable and stable so the auditor can trust its output.

**Docs map:** [README.md](README.md) is the feature/flag reference;
[TUTORIAL.md](TUTORIAL.md) is the hands-on walkthrough (CLI → AI → server/platform
flow); this file is the design — *why* it's built the way it is.

---

## Three run modes, one engine

```
                         ┌─────────────────────────────────────────┐
                         │            core scanner engine            │
   CLI  ───────────────► │  parse spec / build endpoint              │
   (agentdast scan)      │  → run plugins (endpoint × param × vuln)  │
                         │  → Finding{evidence, confidence}          │
   MCP  ───────────────► │                                           │
   (agentdast mcp)       │  internal/core + internal/plugins         │
                         │                                           │
   AI   ───────────────► │                                           │
   (agentdast ai)        └─────────────────────────────────────────┘
        │                                  ▲
        │  LLM tool-call loop              │  toolexec bridge
        └──────────────────────────────────┘  (Executor.Scan / RunConfig)
```

All three modes funnel through `internal/toolexec.Executor`, which is the single place
that turns a request (CLI flags, MCP tool args, or an LLM tool call) into a
`core.Scanner.Run`. This guarantees identical behavior — and identical header/param
handling — regardless of how the scan was triggered.

---

## Package map

```
main.go                 process entry → cmd.Execute()
cmd/                    cobra commands
  root.go               registers plugins, wires subcommands
  scan.go               single-URL OR whole-spec scan
  plugins.go            list plugins
  report.go             render a saved result
  mcp.go                start the MCP server
  ai.go                 start the AI auditor
  serve.go              start the HTTP service (server mode)
pkg/types/              shared models (no internal deps)
  config.go             ScanConfig, AIConfig
  finding.go            Finding, Severity, Confidence
  request.go            RequestLog
  result.go             ScanResult, ScanSummary, Summarize()
internal/core/          the engine
  plugin.go             Plugin interface (lives here to avoid an import cycle)
  registry.go           PluginRegistry (name → Plugin)
  context.go            ScanContext passed to each plugin
  executor.go           RequestExecutor — builds/sends HTTP, injects payloads
  urlscan.go            BuildEndpointFromURL — single-endpoint target
  scanner.go            Scanner.Run — resolves targets, fans out plugins
internal/parser/        OpenAPI/Swagger → normalized endpoints
  loader.go             load file/URL, convert Swagger 2.0 → OpenAPI 3
  extractor.go          walk the spec → []Endpoint
  types.go              ParsedSpec, Endpoint, ParamInfo, BodyInfo
internal/plugins/       one file per vulnerability class (13)
  plugin.go             shared detection helpers (matchSignal, stripPayload, …)
  all.go                Register(registry)
  sqli.go cmdi.go ssrf.go ssti.go xss.go path_traversal.go xxe.go
  idor.go auth.go mass_assignment.go sensitive_data.go cors.go open_redirect.go
internal/knowledge/     embedded vulnerability reference (one <name>.md per class)
  knowledge.go          Get(name)/List() with alias resolution (//go:embed *.md)
  idor.md sqli.md ssrf.md … (served to the AI via get_knowledge)
internal/logging/       slog setup (level + text/json), installed by cmd root
internal/toolexec/      bridge: tool args → scanner → result (shared by MCP + AI)
                        also Knowledge(name) and ScanLogs(scan_id) for the AI
internal/server/        HTTP service: /health, /api/scan, /api/status + worker
  store.go              MySQL (dast table, sast base_url/swagger/report read) + MinIO (swagger+SAST in / report out)
  server.go             routes, request validation, async scan worker
internal/mcp/           MCP JSON-RPC 2.0 server (stdio/TCP)
internal/ai/            the AI auditor
  client.go             OpenAI-compatible client (custom base URL)
  tools.go              scan_api / list_plugins / http_request / get_knowledge / get_scan_logs schemas
  orchestrator.go       the audit loop + report consolidation
  report.go             system/user prompts
  sast.go               SAST report ingestion
internal/output/        text / json / markdown formatters (+ OWASP/severity helpers)
internal/storage/       in-memory (default) / MySQL / MinIO
config/                 YAML config + env overrides
```

---

## The scanning unit (core)

### Data flow of one scan

```
ScanConfig ──► Scanner.Run
                 │
                 ├─ resolveTargets():
                 │     TargetURL set?  → BuildEndpointFromURL  → []Endpoint{1}
                 │     SwaggerSource?  → parser.LoadSpec       → []Endpoint{N}
                 │
                 ├─ registry.Resolve(plugins)  → []Plugin   (empty = all)
                 │
                 └─ scanEndpoints():  for each (endpoint × plugin), bounded by
                       concurrency, run plugin.Test(ScanContext) → []Finding
                                          │
                                          ▼
                              RequestExecutor.Inject(param, payload)
                                  → HTTP request → RequestLog
```

### The Plugin contract

```go
type Plugin interface {
    Name() string                 // "sqli"
    Description() string
    Category() string             // injection | auth | exposure | config
    Severity() types.Severity
    DefaultPayloads() []string
    Test(ctx *core.ScanContext) []types.Finding
}
```

A plugin uses the `ScanContext` helpers and never builds HTTP itself:

```go
ctx.Params()                  // injection targets in scope (honors the insert_point filter)
ctx.Inject(param, payload)    // inject payload into one param → RequestLog
ctx.Baseline()                // unmodified request (for comparison / blind tests)
ctx.Executor.SendRawBody(...) // full control over the body (XXE, mass assignment)
```

### Accuracy model (why findings are trustworthy)

The earlier version produced false positives because a server that merely *echoed* the
payload tripped signature matches. The detection helpers in `plugins/plugin.go` fix this:

- **`stripPayload(resp, payload)`** removes the reflected payload from the response
  before any signature match — an echoed input can no longer be mistaken for a leak.
- **`matchSignal(resp, payload, signatures)`** is the standard match used by sqli, cmdi,
  ssrf, path_traversal, xxe.
- **SQLi** combines three techniques: error-based (broad engine signature set incl.
  SQLite/MySQL/Postgres/Oracle/MSSQL), boolean-based blind (TRUE matches baseline, FALSE
  diverges), and time-based blind (SLEEP delay, confirmed twice).
- **XSS** requires an HTML content type *and* unescaped reflection (encoded reflection =
  the app is escaping correctly = not a finding).
- **SSRF** matches only response-body signatures from the fetched resource (e.g. EC2
  `ami-id`), never the URL itself.
- **IDOR** only flags *secured* endpoints returning *distinct* objects, as a candidate
  needing two-account verification (public catalogs are not flagged).
- **Confidence** (`confirmed` / `probable` / `possible`) is set per finding so the
  auditor can weight them.

Each `Finding` carries `Evidence` (a request/response snippet) and the triggering
`RequestLog`, so every claim is backed by what actually happened on the wire.

---

## The AI auditor

`internal/ai/orchestrator.go` runs this loop:

```
1. Load spec (context) + SAST report (raw) + apply --target, --header, --insert-point.
2. Baseline auto-scan (optional, default on): one broad scan of every spec endpoint
   against --target, to ground the model with real findings up front.
3. Build messages: system prompt (auditor rules + iterative workflow) + user prompt
   (API surface, baseline results, SAST report, applied headers/insert-points).
4. Tool-call loop (backstopped by --max-turns, default 300):
     model → wants to call a tool?
        yes → toolexec runs it → return the result → loop
        no  → the model is done → its message is the audit narrative → exit
5. If the loop hits the turn cap, force a tools-disabled synthesis call so the user
   always gets a real report (never a placeholder).
6. finalReport(): the model narrative + ONE consolidated, deduplicated list of
   scanner-confirmed findings (dedup key: plugin|method|endpoint|param).
```

### The auditor's tools (iterative verify loop)

The auditor verifies one hypothesis at a time, and — crucially — can **doubt the
scanner and check its work**:

| tool             | purpose                                                              |
|------------------|----------------------------------------------------------------------|
| `get_knowledge`  | pull reference knowledge for a vuln class (how to test/confirm, FPs) before testing; falls back to model expertise if absent |
| `scan_api`       | run one plugin at one `insert_point` on one endpoint → `VULNERABLE`/`NOT` verdict + a `scan_id` |
| `get_scan_logs`  | fetch the exact request/response exchanges a scan made (by `scan_id`) when the model doubts a result |
| `http_request`   | send a fully custom request (auth/login flows, business logic, hand-crafted payloads) and judge the raw response |
| `list_plugins`   | enumerate available plugins                                          |

`scan_api` args: `target_url` (path or full URL), `method`/`body`, `plugins` (usually
one), `insert_point` (`loc:name` over query/header/path/cookie/body — multiple allowed;
undeclared points injected anyway), `headers` (per-call auth, overrides the global
`--header`).

**The feedback loop**: scan → if unconvinced, `get_scan_logs` → combine with
`get_knowledge` → craft a custom payload → re-test (`scan_api`/`http_request`) → repeat
until confident. Every tool-driven scan is run in full-log mode so its exchanges are
retrievable by `scan_id`. When a SAST report is present, a **verification gate** prevents
the model from writing the final report until it has actually exercised test cases.

### Headers and insert-point propagation

CLI `--header` / `--insert-point` set `AIConfig.CustomHeaders` / `InsertPoints`, which the
orchestrator copies onto `toolexec.Executor.DefaultHeaders` / `DefaultInsertPoints`.
`Executor.RunConfig` merges these into **every** scan — both the baseline and each
`scan_api` call the model makes — with caller-supplied values winning. So an auth token
passed once on the CLI authenticates the whole audit. (Verified by
`internal/toolexec/toolexec_test.go`.)

### Separation of confirmed vs claimed

The final report deliberately splits:

- **Auditor Analysis** — the model's narrative (may cite SAST claims).
- **Scanner-Confirmed Findings** — only what the scanner dynamically proved.

This makes the report honest: a reader sees exactly which risks were verified against the
running API versus inferred from static analysis.

---

## Server mode (HTTP API)

`internal/server` runs AgentDAST as a service for the wider platform. It is a thin,
stateful wrapper around the same AI audit flow:

```
                ┌────────────── agentdast serve ──────────────┐
  POST /api/scan│  validate → CreateScan(dast: new)            │
  ─────────────►│  → 202 {scan_id}                             │
                │        │ (async worker, bounded by a semaphore)
                │        ▼                                      │
                │  status=processing                           │
                │  sast table by project_id → base_url + swagger key + SAST report key
                │    (each overridable by the matching request field)
                │  MinIO ──bytes──► temp files (swagger + SAST report)
                │  AI orchestrator.Run(swagger, base_url, SAST report, prompt)
                │  report markdown ──► MinIO (<project_id>/dast/report.md)
                │  dast: status=done, result_path  (or fail+error_msg)
  GET /api/status──── read dast row ───────────────────────────► {status, result_path|error}
  GET /health ─────── DB ping ─────────────────────────────────► {status}
                └──────────────────────────────────────────────┘
        MySQL (state: dast + sast read)      MinIO (swagger + SAST report in / report out)
```

- **project_id is the request**: a scan needs only `project_id`. The service reads the
  project's base URL, swagger key, and SAST report key from the `sast` table, then pulls
  the swagger + SAST report from MinIO by those keys. **Files never cross the API** — only
  keys do. `base_url` / `swagger_path` / `sast_report_path` may be sent to override a
  column.
- **Boundaries**: the service *owns* the `dast` table and *reads* the `sast` table
  (column/table names are config, defaulting to `sast.result_swagger_path`,
  `sast.result_report_path`, and `sast.result_swagger_base_url` keyed by `project_id`).
  It never writes SAST data. The SAST report is supplementary — a missing one yields a
  dynamic-only audit, not a failure; a missing row / swagger key / base URL fails the scan.
- **Async**: scans run in goroutines bounded by `MAX_CONCURRENT_SCANS`; the request
  returns a `scan_id` immediately and the client polls `/api/status`.
- **Reuse**: the worker calls the exact same `ai.Orchestrator` + `toolexec.Executor`
  the CLI `ai` command uses — server mode adds persistence and an HTTP shell, not new
  scanning logic.
- **Files**: `internal/server/store.go` (MySQL + MinIO), `internal/server/server.go`
  (HTTP + worker), `cmd/serve.go` (command). Containerized via `Dockerfile`; wired into
  the platform by the parent `docker-compose.yml`.

### Platform integration & deployment

AgentDAST is one agent in a multi-agent platform (alongside AgentSAST and a Manager).
The agents do **not** pass files to each other — they pass a `project_id` and share two
backends: a MySQL database (each agent owns its own table) and a MinIO bucket laid out
per project.

```
 AgentSAST ──writes──► sast table (result_swagger_base_url / _swagger_path / _report_path)
     │                 MinIO  <project>/sast/openapi.yaml + <project>/sast/report.md
     ▼
 Manager ── POST /api/scan {project_id} ──► AgentDAST ──reads sast row, pulls from MinIO
                                                       ──writes <project>/dast/report.md
                                                       ──owns the dast table (state)
```

MinIO object layout (one prefix per project):

```
/<bucket>/<project-id>/
  ├── raw/                 # original inputs
  ├── sast/openapi.yaml    # ← result_swagger_path   (read)
  ├── sast/report.md       # ← result_report_path    (read)
  └── dast/report.md       # ← written by AgentDAST
```

- **Service boundary on the network**: the container does **not** publish a host port
  (`expose: 8080`, not `ports:`). Sibling agents reach it only on the compose network as
  `http://agentdast:8080`. This keeps the scan API off the host while leaving it fully
  reachable inside the platform. A `ports:` mapping in a compose override re-exposes it
  for local debugging.
- **Config-driven coupling**: the only hard dependency on AgentSAST is the `sast` table
  shape, and every column/table name is an env var (`SAST_TABLE`, `SAST_*_COLUMN`). The
  report's output location is likewise configurable (`DAST_REPORT_DIR`/`DAST_REPORT_FILE`,
  default `<project>/dast/report.md`).

### Testing

- **Unit tests** live next to the code (`*_test.go`): the scanner engine, parser,
  toolexec bridge (header/insert-point propagation), and server request-validation paths.
  Run with `go test ./...`.
- **End-to-end harness** at the repo root: `docker-compose.test.yml` adds a mock that
  plays both the OpenAI endpoint and the scan target, so `test/e2e.sh` exercises the
  entire platform flow — seed the `sast` table + MinIO, trigger a scan by `project_id`
  from a throwaway container on the network, and assert the report lands at
  `<project>/dast/report.md` — with no real LLM. See [TUTORIAL.md](TUTORIAL.md) §10.

## MCP server

`internal/mcp` implements MCP over JSON-RPC 2.0 (stdio by default, `--tcp` optional) with
no external SDK. It advertises four tools — `scan_api`, `list_plugins`, `get_scan_result`
(with optional `include_logs`), and `get_knowledge` — dispatched straight through the same
`toolexec.Executor` the AI mode uses. This lets any MCP host (Claude Desktop, Cursor) drive
the scanner and read the vulnerability knowledge base directly.

---

## Storage

Default is in-memory (no setup). When `MYSQL_*` env vars are present, scan metadata and
findings persist to MySQL; when `MINIO_*` vars are present, full request logs are gzipped
to MinIO and referenced by object key. `storage.FromEnv()` selects the backend.

---

## Knowledge base

`internal/knowledge` is an embedded set of markdown files (`//go:embed *.md`), one per
vulnerability class, describing how to test, confirm, avoid false positives, and remediate
it. `Get(name)` resolves aliases (e.g. `bola`→`idor`, `"sql injection"`→`sqli`) and returns
the document; a miss tells the caller to fall back to its own expertise. It is served to
the AI via `get_knowledge` and over MCP. Because it is embedded, the binary is
self-contained — no runtime path needed. To extend the knowledge, drop a new
`<name>.md` into the folder.

## Logging

`internal/logging` installs a process-wide `log/slog` logger (level + text/json) on
**stderr**, so stdout stays clean for reports/JSON. Global flags `--log-level` /
`--log-format` (and `LOG_LEVEL` / `LOG_FORMAT`) control it. At debug level the executor
logs every HTTP exchange, the scanner logs lifecycle/panics, the orchestrator logs
turns/tool-calls, and the server emits structured access logs and per-scan events tagged
with `scan_id`/`project_id`.

## Extending: add a vulnerability plugin

1. Create `internal/plugins/<name>.go` implementing `core.Plugin`.
2. In `Test`, iterate `ctx.Params()`, call `ctx.Inject` / `ctx.Baseline`, and use
   `matchSignal` (payload-stripped) so reflected input cannot cause false positives.
3. Return `Finding`s with `Evidence`, `Confidence`, and `Remediation`.
4. Register it in `internal/plugins/all.go`.
5. Add `internal/knowledge/<name>.md` so the AI can pull guidance for it, and map the
   plugin in `output.owaspFor` for the report classification.

It is then automatically available to CLI, MCP, and the AI auditor — no other wiring.
```
