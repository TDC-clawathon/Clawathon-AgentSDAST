package ai

import (
	"fmt"
	"strings"
)

// toolPreamble maps the skill's shell-oriented instructions onto the Go tools
// and neutralizes the Codex-only steps, so the existing SKILL.md can be reused
// verbatim as domain knowledge.
const toolPreamble = `You are AgentSAST, a senior application-security engineer running a STATIC analysis.
This is an AGENTIC task: call tools to do real work — never answer from assumptions.

You have these tools (call them directly; do not ask permission):
- extract_archive(path, dest): extract a .zip/.tar/.tar.gz/.tgz under the workspace.
- list_files(glob, max): discover the project layout.
- read_file(path, max_bytes, offset): open actual source files.
- search_code(pattern, glob, max_results): regex-search for routes, sinks (SQL/exec/file/template/redirect/SSRF), and auth checks.
- get_knowledge(topic): pull a vulnerability reference (idor, sqli, xss, payment).
- write_artifact(name, content): write sast/openapi.yaml, sast/report.md, or sast/base_url.txt.
- validate_openapi(name): validate sast/openapi.yaml and get the exact error to fix.

The SKILL below was written for a shell agent. Map its instructions to the tools above:
  unzip/tar/7z  -> extract_archive
  cat/ls/grep/rg -> read_file / list_files / search_code
  swagger-cli validate -> validate_openapi
IGNORE any instruction to run "$codex-security:deep-security-scan" or any "$codex-*" plugin — you do not have it. Rely on your own analysis and the references.

Strategy to stay within context limits: use list_files and search_code to LOCATE routers/handlers/sinks first, then read_file selectively. Do not read the whole tree.

Finish criteria: keep calling tools until sast/openapi.yaml, sast/report.md and sast/base_url.txt all exist and are non-empty AND validate_openapi returns "valid". Only then reply with a one-line completion summary and NO tool calls.`

// systemPrompt assembles the persona/tool preamble plus the SAST skill text.
func systemPrompt(skillText string) string {
	if strings.TrimSpace(skillText) == "" {
		return toolPreamble
	}
	return toolPreamble + "\n\n===== SAST SKILL =====\n" + skillText
}

// userPrompt states the concrete task and the (possibly wrong) base URL hint.
func userPrompt(cfg Config) string {
	hint := strings.TrimSpace(cfg.BaseURLHint)
	if hint == "" {
		hint = "(not provided — derive it from the routing code)"
	}
	return fmt.Sprintf(`The uploaded project is in ./raw (it may be an archive — extract it first).

Produce exactly three files via write_artifact:
1. sast/openapi.yaml — OpenAPI 3.x enriched with code-derived constraints, a top-level servers: whose first url is the VERIFIED base URL, and securitySchemes/security for any enforced auth.
2. sast/report.md — a DAST-oriented vulnerability report (per-endpoint attack surface, candidate vulns with file:line evidence, OWASP/CWE, concrete test payloads, prioritized summary table).
3. sast/base_url.txt — ONLY the verified base URL on a single line.

The Manager says the base URL is: %s — this MAY BE WRONG or incomplete. Determine the REAL base URL from the routing code and correct it.

After writing openapi.yaml, call validate_openapi and fix any errors until it returns "valid".`, hint)
}
