---
name: deepscan
mode: deepscan
kind: toggle
description: Comprehensive, repo-wide static security analysis. Applies a rigorous multi-phase methodology (threat model → discovery → validation → attack-path/severity) for maximum coverage and low false-positive rate, then produces the enriched OpenAPI contract + DAST-oriented report.
deliverables: sast/openapi.yaml, sast/report.md, sast/base_url.txt
references: shared-hard-rules, scan-artifacts, threat-model, finding-discovery, validation, attack-path-analysis, deep-security-scan, security-scan, security-diff-scan, fix-finding, final-report
---

# Deep Scan

A **thorough, repo-wide** static security analysis. Where quickscan is a fast
targeted pass, deepscan aims for maximum coverage with disciplined validation.
Spend the effort to read broadly across the codebase and reason about how inputs
reach sinks.

Use the available tools — `extract_archive`, `list_files`, `read_file`,
`search_code`, `get_knowledge`, `write_artifact`, `validate_openapi`.

> Runtime note: the methodology references below were originally authored for a
> multi-agent CLI runtime. **Adapt them to the tools you have.** Ignore any
> instruction to run python scripts, spawn parallel "workers", call
> `$codex-*` plugins, or write intermediate ledger files — instead perform those
> phases yourself within this single agentic loop, using `read_file`/`search_code`
> to gather evidence and folding the results directly into `report.md`.

## Methodology (pull each via get_knowledge)
Work the phases in order; consult the matching reference for the rules of each:
1. `shared-hard-rules` — discipline: stay grounded in code, no findings without evidence.
2. `threat-model` — map assets, trust boundaries, attacker-controlled inputs, invariants.
3. `finding-discovery` — hunt vulnerabilities repo-wide; preserve every distinct instance (don't collapse to one variant).
4. `validation` — confirm/refute each candidate by tracing code paths and inputs; record confidence.
5. `attack-path-analysis` — trace source→sink, calibrate severity with evidence thresholds (includes the severity policy).
6. `security-scan` / `security-diff-scan` / `deep-security-scan` — full-repo and diff-scoped scan playbooks.
7. `fix-finding` — (optional) remediation guidance for confirmed findings.
8. `final-report` — how to structure and order the report (critical→low).
9. `scan-artifacts` — artifact/path conventions (informational).

Also pull the focused class references when relevant (shared with quickscan):
`get_knowledge("idor"|"sqli"|"xss"|"payment")`.

## Deliverables (write under sast/ via write_artifact)
Same three artifacts as every SAST mode, produced to a higher coverage/confidence bar:
1. `openapi.yaml` — OpenAPI 3.x with code-derived validation/formats/business
   rules (`x-*`), enforced auth (`securitySchemes` + `security`), `x-source:
   <file>:<line>` per operation, and a top-level `servers:` with the **verified**
   base URL (corrected from the routing code).
2. `report.md` — DAST-oriented, per-endpoint attack surface + candidate vulns
   (OWASP API Top 10 + CWE, WHY with `file:line`, concrete test payloads),
   ordered critical→low, ending with a `endpoint | issue | severity | CWE` table.
3. `base_url.txt` — only the verified base URL, one line.

After writing `openapi.yaml`, call `validate_openapi` and fix until valid. Keep
going until all three files exist and validate.
