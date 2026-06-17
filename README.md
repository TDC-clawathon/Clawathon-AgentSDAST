# AgentSDAST

## 1. Problem

**Challenge**: Current API security testing still relies on manual analysis, consuming significant time and resources.

**Who faces this problem**: Security engineers, DevSecOps teams, API developers who need to ensure API safety before deployment.

**Why it consumes time/resources**:
- Static code analysis (SAST) requires thorough source code analysis, extracting API constraints (validation rules, auth methods, base URLs)
- Dynamic testing (DAST) needs to scan each endpoint with multiple payloads, but high false positives if only checking signatures
- Verify findings: security team must manually confirm each vulnerability on live API, taking days to validate a single project
- Maintain test cases: manual updates when API changes

---

## 2. Target Users

- **Security Engineers/Testers**: Need to automate vulnerability detection and verification
- **DevSecOps Teams**: Integrate security scanning into CI/CD pipelines
- **API Developers**: Verify security before production, reduce time-to-deploy

---

## 3. How Agent Solves It (Solution)

**Input -> Processing -> Output:**

**AgentSAST** (Static Analysis):
- Input: Source code (`.zip`/`.tar.gz`) + base URL hint
- Processing: LLM (Codex CLI) analyzes code, extracts API constraints (parameter validation, auth scheme, real base URL)
- Output: Enriched OpenAPI 3.x spec + SAST vulnerability report

**AgentDAST** (Dynamic Analysis):
- Input: OpenAPI spec + SAST report + live API target
- Processing: LLM auditor verifies each SAST claim via real HTTP requests, scans missing endpoints
- Output: Verified vulnerability report + Scanner-Confirmed Findings

**Manager** (Orchestration):
- Input: Project details (source code, base URL)
- Processing: Trigger AgentSAST -> AgentDAST -> generate final report, manage models/skills
- Output: Complete security audit

**AgentBase usage**: Multi-agent platform on Docker Compose, sharing MySQL + MinIO. AgentSAST -> AgentDAST chain via `project_id`, no direct file transfer, uses shared object storage. Skills (SAST prompts, vulnerability knowledge) centrally managed at `Manager`.

---

## 4. Value Delivered

When agents run daily:

- **Time savings**: Reduce from 2-3 days manual analysis -> 30-45 minutes (SAST + DAST automated)
- **Eliminate manual steps**: Auto-extract API constraints, auto-scan, auto-verify findings against live API
- **Reduce false errors**: LLM verifies twice (static + dynamic) -> 70%+ reduction in false positives/negatives
- **Continuous security**: Integrate with CI/CD, auto-scan per release, early security issue detection
- **Knowledge reuse**: Vulnerability knowledge base (SAST references, DAST techniques) updated once, used by entire team