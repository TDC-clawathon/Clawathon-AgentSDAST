# threat-model (deepscan methodology)


# Security Threat Model

## Objective

Establish the repository-scoped threat model at the path defined in `../../references/scan-artifacts.md`. If this already exists, stop here. If a threat model or clearly authoritative security scan guidance is provided or already exists, persist it unchanged to this file, then stop here.

`AGENTS.md` can be that authoritative source when it is sufficiently specific about the repository's product surfaces, trust boundaries, attacker-controlled inputs, assumptions, or security scan guidance to serve as the threat model.

If no threat model is provided, generate a repository-scoped threat model to be used in future bug discovery. The threat model should holistically cover the entire repository and should make it obvious:

- what assets or privileges matter
- what trust boundaries exist
- what inputs are attacker-controlled
- what invariants the code must preserve
- what repository-wide failure modes would matter most

## Artifact Resolution

The path references in this skill are the default locations for this phase.
If the user explicitly provides a different path for a required input or output, use the user-provided path instead of the corresponding default path referenced in this skill.
If a required input is still missing, stop and ask the user for it before continuing.
Use the shared scan artifact path conventions in `../../references/scan-artifacts.md`.

## Workflow

1. Resolve `repo_name`, `security_scans_dir`, and the repository-scoped threat model path using `../../references/scan-artifacts.md`.
2. If the repository-scoped threat model already exists, stop here.
3. If a threat model or authoritative security scan guidance is provided or referenced:
   - write it exactly to the repository-scoped threat model path
   - treat that file as the only threat model source of truth
   - do not expand, summarize, or reinterpret it
   - `AGENTS.md` is acceptable here when it is clearly being used as the security scan guidance or threat model source for this scan and is sufficiently repository-specific to stand in for a threat model
4. Otherwise, generate a repository-scoped threat model using the checklist below.
5. Before finalizing this phase, sanity-check that:
   - the threat model is repository-scoped rather than being centered around any specific scan target
   - it describes repository-wide primary product or runtime surfaces and trust boundaries before covering any narrower examples
   - any vulnerability-class discussion is about repository-context classes, not findings about any current diff
6. Write the exact threat model to the repository-scoped threat model path.

## Threat Model Generation Guidance

Generate and structure the threat model using `references/threat-model-guidance.md`.

## Hard Rules

- A provided threat model or authoritative security scan guidance is authoritative.
- Threat model generation must stay at repository scope unless the user explicitly asks for narrower scope.
- Do not turn this phase into findings about any current diff.
- Do not let the current scan target, touched subsystem, or changed directories become the center of gravity for this phase unless the user explicitly asks for that narrower scope.
- In large monorepos, avoid centering `personal/`, `test/`, `tests/`, `docs/`, `examples/`, or one-off developer tooling unless repository evidence shows those are real deployed or privileged workflow surfaces.
- Call out trust boundaries and assumptions explicitly.
- Keep references to vulnerability types at the level of repository-context classes, rather than any diff findings.
- Persist the threat model output to the repository-scoped threat model path from `../../references/scan-artifacts.md`.

---
### reference: threat-model-guidance

# Threat Model Guidance

Use this guidance during threat model generation.

## Threat Model Generation Checklist

Do not restate this checklist in the final threat model output.

- Start at the repository root and use the minimum hops needed to understand the repository's real-world purpose before narrowing into critical components.
- Keep this phase at repository scope unless the user explicitly asks for a narrower target-scoped threat model.
- Ignore any reviewed commit, diff, changed files, changed directories, commit title, and scan target during threat model generation unless the user explicitly asks for narrower scope.
- Distinguish primary product or runtime code from developer-only, test-only, documentation-only, example, prototype, or one-off tooling paths.
- Identify the primary product or runtime surfaces the repository actually exposes.
- Identify the main trust boundaries and which actors sit on each side of them.
- Explicitly separate attacker-controlled, operator-controlled, and developer-controlled inputs.
- Describe common vulnerability classes that are relevant in this repository context rather than findings about the current diff.
- Call out mitigations, robustness measures, and security controls already present in the repository when they materially affect severity or scope.
- Explain when attacker stories are realistic, when they are out of scope, and when the repository's real-world usage makes a vulnerability class less important.
- Note unique security considerations for the codebase, for example:
  - authn/authz, session management, CSRF, XSS, SSRF, injections, tenant boundaries, rate limits, and secret handling for web applications
  - key management, privacy assumptions, ACLs/RBAC, PII handling, and auditability for cryptography or privacy-sensitive systems
  - public interfaces, embedding assumptions, safe-by-default behavior, footguns, and secure usage patterns for libraries or frameworks
  - production/runtime code paths versus CI, build, or local developer tooling
- Explain when a vulnerability class would be critical, high, medium, or low in this repository and give a couple of concrete examples at each level.
- If a vulnerability class requires attacker control that does not exist in the repo's real-world usage, say so in the severity calibration discussion.
- When possible, point to specific files, components, or controls that ground the threat model.

## Output Contract

When generating a threat model, structure it in Markdown with these sections:

- Overview
- Threat Model, Trust Boundaries, and Assumptions
- Attack Surface, Mitigations, and Attacker Stories
- Severity Calibration (Critical, High, Medium, Low)

The threat model should help a security researcher understand the codebase and its likely security-relevant failure modes. It should be detailed, repository-scoped, and suitable for reuse across unrelated diffs in the same repo.

Within those sections, make sure the output covers:

- repository overview and intended real-world usage
- trust boundaries and assumptions
- attacker stories and out-of-scope attacker stories
- attack surfaces and existing mitigations
- which vulnerability classes matter most in context
- which vulnerability classes are less severe or out of scope in context
- severity calibration with concrete examples at each level
- references to concrete files or controls when those materially ground the model
