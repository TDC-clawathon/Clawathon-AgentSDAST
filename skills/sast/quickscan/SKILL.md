---
name: quickscan
mode: quickscan
kind: toggle
description: Lightweight, fast SAST pass over an API service's source. Targets the high-impact vulnerability classes (IDOR/BOLA, SQLi, XSS, payment/pricing logic) for quick feedback, and produces the enriched OpenAPI contract + DAST-oriented report.
deliverables: sast/openapi.yaml, sast/report.md, sast/base_url.txt
references: idor, sqli, xss, payment
---

# Quick Scan

A **fast, targeted** static review of the API project in the workspace. Optimize
for quick feedback: focus on the highest-impact vulnerability classes rather than
exhaustive coverage. Ground every claim in the actual code.

Use the available tools — `extract_archive`, `list_files`, `read_file`,
`search_code`, `get_knowledge`, `write_artifact`, `validate_openapi`. Locate
routers/handlers/models with `search_code`/`list_files`, then `read_file` the
relevant files; don't read the whole tree.

## Inputs
- `./raw/` — the raw upload (a source tree, or an archive: `.zip`/`.tar`/`.tar.gz`/`.tgz`).
  Extract it first if needed, then locate entrypoints, routers, controllers,
  models, and any existing `openapi.y*ml` / `swagger.json`.

## Focus (high-impact classes)
Pull the matching reference with `get_knowledge` when you suspect a class, and use
it as a checklist — still ground each finding in code:
- `idor` — IDOR / BOLA (broken object-level authorization)
- `sqli` — SQL injection
- `xss` — reflected / stored / DOM XSS
- `payment` — pricing/business-logic flaws (price/quantity tampering, overflow, coupon/currency abuse)

Quick scan is a fast pass, not exhaustive — flag plausible issues with clear
confidence/evidence and move on. For comprehensive, repo-wide methodology enable
**deepscan**; for payment-gateway integration weaknesses enable **pgwscan**.

## Deliverables (write under sast/ via write_artifact)
1. `openapi.yaml` — one OpenAPI 3.x document reflecting the **real runtime
   contract**: code-derived validation (`minimum`/`maximum`, `minLength`/`pattern`,
   `enum`, `required`), real formats (uuid v4 vs v7, email, e164, date-time),
   business rules (`x-*` extensions), enforced auth (`securitySchemes` + `security`,
   or state explicitly if none — itself a finding), and `x-source: <file>:<line>`
   per operation. A top-level `servers:` whose first url is the **verified** base
   URL (determine the real prefix from the routing code; correct the supplied one).
2. `report.md` — DAST-oriented: per endpoint, method+path+auth, attack surface
   (params reaching sinks), candidate vulns mapped to OWASP API Top 10 + CWE with
   WHY (`file:line`), and 2–4 concrete test payloads. End with a prioritized
   table: `endpoint | issue | severity | CWE`.
3. `base_url.txt` — only the verified base URL, one line.

After writing `openapi.yaml`, call `validate_openapi` and fix any errors until it
returns valid. Keep going until all three files exist and validate.
