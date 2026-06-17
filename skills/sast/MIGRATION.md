# SAST skills standardization — migration notes

Reorganized `skills/sast/` into three scan modes and standardized every skill.
This file is documentation only; the AgentSAST engine loads only each mode's
`SKILL.md` + `references/*.md`.

## Structure (after)

```
skills/sast/
├── quickscan/   SKILL.md + references/{idor,sqli,xss,payment}.md
├── deepscan/    SKILL.md + references/*.md  (flattened methodology)
└── pgwscan/     SKILL.md + references/{01..09,99}.md + assets/*
```

- **quickscan** (toggle): lightweight, fast pass over high-impact classes (IDOR, SQLi, XSS, payment logic).
- **deepscan** (toggle): comprehensive, repo-wide multi-phase methodology.
- **pgwscan** (option): payment-gateway–specific checks, layered on quick/deep.

## Naming / metadata
- Standardized YAML frontmatter on every `SKILL.md`: `name`, `mode`, `kind`
  (`toggle`|`option`), `description`, `deliverables`, `references`.
- Renamed skill `name` to its mode (`quickscan`/`deepscan`/`pgwscan`); the
  pgwscan skill was previously `payment-gateway-blackbox-audit`.

## Deepscan: flattened from the legacy Codex plugin tree
The old `deepscan/skills/<sub>/{SKILL.md,references,agents/openai.yaml}` tree was
the `codex-security` plugin (built for the now-removed Codex CLI: parallel
"workers", python scripts, `$codex-*` calls). Per decision, **all sub-skill text
was preserved** by flattening each sub-skill into a single reference:

| New reference (`deepscan/references/`) | Source |
|---|---|
| `attack-path-analysis.md` | sub-skill SKILL.md + `attack-path-facts.md` + `severity-policy.md` |
| `deep-security-scan.md` | sub-skill SKILL.md |
| `finding-discovery.md` | sub-skill SKILL.md |
| `fix-finding.md` | sub-skill SKILL.md |
| `security-diff-scan.md` | sub-skill SKILL.md |
| `security-scan.md` | sub-skill SKILL.md + its repo-wide references |
| `threat-model.md` | sub-skill SKILL.md + `threat-model-guidance.md` |
| `validation.md` | sub-skill SKILL.md + `validation-guidance.md` |

Kept top-level: `final-report.md`, `scan-artifacts.md`, `shared-hard-rules.md`.
A new `deepscan/SKILL.md` indexes these and maps the methodology onto the engine's
real tools (extract/list/read/search/get_knowledge/write/validate).

## Deleted (clutter / non-functional under the new engine)
- 7 × `.DS_Store`.
- 8 × `agents/openai.yaml` — Codex CLI plugin manifests (Codex removed).
- `deepscan/scripts/*.py` — `generate_rank_input.py`, `render_report_html.py`,
  `validate_report_format.py` (the engine has no shell/python; AgentReport now
  renders HTML).
- `deepscan/assets/report_template_inlined.html` (superseded by AgentReport
  templates) and `deepscan/assets/logo.png` (unused branding).

## Detection logic
Preserved. quickscan references (idor/sqli/xss/payment) unchanged; deepscan
methodology text preserved verbatim (frontmatter stripped) when flattened; pgwscan
methodology + references preserved. SKILL.md prose was reframed for the whitebox
single-loop engine, but no detection guidance was removed.

## No duplicates removed
quickscan `payment.md` (whitebox source patterns) and pgwscan (integration
weakness classes) are complementary, not duplicates — both kept.
