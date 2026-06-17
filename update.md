# AgentSAST & AgentDAST — integration changes (handover)

This document describes platform integration changes for **AgentSAST** and **AgentDAST**: shared skills layout, MinIO, Manager model config, and MySQL `AgentModelConfig`. Use it when merging or maintaining these services with the original agent codebases.

**Baseline commits (before platform integration):**

| Service    | Commit    | Message                    |
|-----------|-----------|----------------------------|
| AgentSAST | `13b5f49` | feat: swagger-cli validate |
| AgentDAST | `1939f26` | update scan to run DAST    |

---

## 1. Skills — single source at repo root

All editable skill / knowledge files live **only** under the repository root:

```
skills/
  sast/
    SKILL.md
    references/              # idor.md, sqli.md, xss.md, payment.md
  dast/
    auth.md, sqli.md, xss.md, …   # one .md per vulnerability class
```

### Removed (do not add skills here anymore)

| Old path | Status |
|----------|--------|
| `AgentSAST/skills/` | **Deleted** — use `skills/sast/` |
| `AgentDAST/internal/knowledge/*.md` | **Deleted** — use `skills/dast/` |

`AgentDAST/internal/knowledge/` now contains **Go code only** (`knowledge.go`, `files.go`, `source.go`, tests).

### Data flow

```
skills/  (git — edit here)
    │
    ├─► skills-init (compose) ──► skills_data volume  ──► agent-dast /app/skills/dast
    │
    ├─► Manager startup  ──► MinIO skills/sast/ + skills/dast/  (seed if missing)
    │                              │
    │                              └─► AgentSAST HTTP  ──► CODEX_HOME/skills/sast
    │
    └─► AgentDAST CLI/MCP  ──► read skills/dast/ from disk (auto-detect or SKILLS_DAST_DIR)
```

| Layer | SAST | DAST |
|-------|------|------|
| **Edit in git** | `skills/sast/` | `skills/dast/` |
| **MinIO** | `skills/sast/` | `skills/dast/` (Manager UI seed only) |
| **Manager UI** | `#/skills` → MinIO | `#/skills` → MinIO |
| **HTTP agent runtime** | MinIO → Codex home | `skills_data` volume → `knowledge.Init()` |
| **CLI / MCP** | N/A (server only) | `skills/dast/` on disk |

Manager seeds MinIO on startup (**upload missing files only**; set `SKILLS_SYNC_OVERWRITE=1` to replace all).

**Docker build context** for agents and Manager is the **repo root** so Dockerfiles can `COPY skills/...`.

---

## 2. AgentSAST changes

### 2.1 New files

| File | Purpose |
|------|---------|
| `internal/service/codex/skills_minio.go` | Download `skills/sast/` from MinIO → `$CODEX_HOME/skills/sast` |
| `internal/db/repo/modelconfig.go` | Read `AgentModelConfig` from shared MySQL |

### 2.2 Modified files

| File | Change |
|------|--------|
| `Dockerfile` | Repo-root build; **no** bundled `skills/` in image |
| `cmd/server/main.go` | MinIO skill sync at startup + before each job; `ModelConfig` repo |
| `internal/handler/route_run.go` | Optional `model` in `POST /run` |
| `internal/service/codex/codex.go` | `Runner.ConfigureModel(model)` |
| `internal/service/codex/runner.go` | Rewrites `CODEX_HOME/config.toml` per job model |
| `internal/service/codex/setup.go` | `WriteConfig()`; skills installed from MinIO only |
| `internal/service/job/job.go` | Model resolve + MinIO skill refresh each run |

### 2.3 Runtime behaviour

1. **Startup:** download MinIO `skills/sast/` → `$CODEX_HOME/skills/sast` (retry up to 15×). Requires Manager to have seeded MinIO first.
2. **Each job:** refresh skills from MinIO (UI edits apply without restart).
3. **Model:** `AgentModelConfig` (`sast`) → request `model` → env `LLM_MODEL`.

### 2.4 Env vars

| Variable | Default / notes |
|----------|-----------------|
| `SKILLS_MINIO_PREFIX` | `skills/sast` |
| `LLM_MODEL` | Fallback if DB empty |
| `CODEX_HOME` | Codex config + installed skill tree |

### 2.5 API

```json
POST /api/sast/run
{
  "project_id": "<uuid>",
  "base_url": "https://api.example.com",
  "model": "optional-override"
}
```

---

## 3. AgentDAST changes

### 3.1 New files

| File | Purpose |
|------|---------|
| `internal/knowledge/files.go` | Load `skills/dast/*.md` from disk (`Init`, `ResolveDastSkillsDir`) |
| `internal/knowledge/source.go` | `documentSource` + default init |

### 3.2 Modified files

| File | Change |
|------|--------|
| `Dockerfile` | Repo-root build; `COPY skills/dast` as fallback; `SKILLS_DAST_DIR` |
| `cmd/serve.go` | `knowledge.Init(SKILLS_DAST_DIR)` on startup |
| `internal/knowledge/knowledge.go` | `Get`/`List` via `currentSource` |
| `internal/server/server.go` | `scanRequest.model`; `resolveModel()` |
| `internal/server/store.go` | `EnabledAgentModel("dast")` |

### 3.3 Runtime behaviour

| Mode | Knowledge source |
|------|------------------|
| **`agentdast serve`** | Disk: `SKILLS_DAST_DIR` (compose mounts `skills_data` → `/app/skills/dast`) |
| **CLI / MCP / `ai`** | Same: `skills/dast/` (walk from cwd or `SKILLS_DAST_DIR`) |

Compose service **`skills-init`** rsyncs host `./skills/` into named volume **`skills_data`** before `agent-dast` starts. Re-sync after git edits: `docker compose run --rm skills-init`.

Per scan: model from `AgentModelConfig` (`dast`) → request `model` → `OPENAI_MODEL`.  
`OPENAI_API_KEY` required; `OPENAI_MODEL` optional if DB configured.

### 3.4 Env vars

| Variable | Default / notes |
|----------|-----------------|
| `SKILLS_DAST_DIR` | `/app/skills/dast` in container; auto-detect in repo for CLI |
| `OPENAI_MODEL` | Fallback model |
| `MINIO_*` | Scan artifacts only (not knowledge) |

### 3.5 API

```json
POST /api/dast/run
{
  "project_id": "<uuid>",
  "base_url": "https://api.example.com",
  "model": "optional-override",
  "prompt": "optional auditor context"
}
```

---

## 4. Shared: `AgentModelConfig` (MySQL)

Managed by **Manager** (`Manager/lib/db/index.js`):

```sql
AgentModelConfig (
  agent_type ENUM('sast','dast') UNIQUE,
  model_name VARCHAR(255),
  enabled TINYINT(1)
)
```

Resolution order at job/scan time:

1. `model` in HTTP request (Manager orchestrator)
2. `AgentModelConfig` where `enabled = 1`
3. Env (`LLM_MODEL` / `OPENAI_MODEL`)

---

## 5. Manager integration

| Feature | Location |
|---------|----------|
| Seed `skills/` → MinIO | `Manager/lib/storage/syncSkills.js` (startup) |
| Skills CRUD UI | `Manager` → `#/skills` |
| Model assignment UI | `Manager` → Models |
| Pass `model` to agents | `Manager/lib/agents/client.js` |

Compose order: **Manager starts first** (seeds MinIO) → agents start.

Env (Manager):

| Variable | Notes |
|----------|--------|
| `SKILLS_SYNC_ON_STARTUP` | `1` (default) |
| `SKILLS_SYNC_OVERWRITE` | `0` (default); `1` = replace MinIO from seed |
| `SKILLS_SEED_SAST_DIR` | Override; default `skills/sast/` |
| `SKILLS_SEED_DAST_DIR` | Override; default `skills/dast/` |

---

## 6. Migration checklist

### Skills (both owners)

- [ ] Edit **only** `skills/sast/` and `skills/dast/` in git.
- [ ] Do **not** recreate `AgentSAST/skills/` or `AgentDAST/internal/knowledge/*.md`.
- [ ] After git changes: restart Manager (or `POST /api/skills/sync`) to seed new files to MinIO.
- [ ] New DAST topic: add `skills/dast/<name>.md` + alias in `knowledge.go` if needed.
- [ ] New SAST reference: add under `skills/sast/references/`.

### AgentSAST owner

- [ ] Ensure MinIO has `skills/sast/` before agent starts.
- [ ] `LLM_*` env still required; model name from Manager Models page.
- [ ] Docker build: `docker compose build agent-sast` (repo-root context).

### AgentDAST owner

- [ ] HTTP serve: knowledge from `skills_data` volume (`knowledge.Init`, `SKILLS_DAST_DIR`).
- [ ] After editing `skills/dast/` on host: `docker compose run --rm skills-init` then restart `agent-dast`.
- [ ] Local CLI/tests: run from repo root (or set `SKILLS_DAST_DIR`) so `skills/dast/` is found.
- [ ] Docker build: `docker compose build agent-dast` (repo-root context).
- [ ] Update agent README/TUTORIAL references from `internal/knowledge/<name>.md` → `skills/dast/<name>.md` when you touch those docs.

---

## 7. File diff summary (vs baseline)

```
skills/                               # NEW — canonical skills (repo root)
  sast/
  dast/

AgentSAST/
  Dockerfile                          # repo-root context; no local skills copy
  cmd/server/main.go                  # MinIO skills + model config
  internal/db/repo/modelconfig.go     # NEW
  internal/handler/route_run.go       # optional model
  internal/service/codex/skills_minio.go  # NEW
  internal/service/codex/*.go         # ConfigureModel, WriteConfig
  internal/service/job/job.go         # model + skill sync
  skills/                             # REMOVED

AgentDAST/
  Dockerfile                          # COPY skills/dast fallback; SKILLS_DAST_DIR
  cmd/serve.go                        # knowledge.Init from disk
  internal/knowledge/files.go         # NEW — disk source from skills/dast
  internal/knowledge/source.go        # NEW — no go:embed
  internal/knowledge/knowledge.go     # source abstraction
  internal/knowledge/*.md             # REMOVED
  internal/server/server.go           # model resolve
  internal/server/store.go            # AgentModelConfig

Manager/
  lib/storage/syncSkills.js           # seed from skills/ only
  Dockerfile                          # COPY skills/sast + skills/dast

docker-compose.yml                    # skills-init + skills_data volume for agent-dast
```

---

## 8. Quick verification

```bash
# Full stack (skills-init runs before agent-dast)
docker compose up -d --build manager agent-sast agent-dast

# Re-sync host skills/ into volume after edits
docker compose run --rm skills-init && docker compose restart agent-dast

# MinIO seeded from skills/
curl -s http://localhost:8000/api/skills/sast | jq '.files | length'   # expect 5
curl -s http://localhost:8000/api/skills/dast | jq '.files | length'   # expect 13

# Agent logs
docker compose logs agent-sast | grep 'skills loaded from minio'
docker compose logs agent-dast | grep 'knowledge loaded from disk'

# Local DAST knowledge tests (from repo root)
cd AgentDAST && go test ./internal/knowledge/...
```

---

*Handover doc for AgentSAST / AgentDAST owners after platform integration (Manager, MinIO, root `skills/`).*
