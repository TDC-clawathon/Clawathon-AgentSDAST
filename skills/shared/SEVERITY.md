# Shared severity & category conventions

These definitions are shared across all agents (SAST, DAST, Report) so that
findings, statistics, and reports use a consistent vocabulary.

## Severity levels

- **Critical** — remote code execution, auth bypass, or trivially exploitable data exposure.
- **High** — injection (SQLi, command), IDOR/BOLA, sensitive data exposure requiring some effort.
- **Medium** — XSS, CSRF, misconfiguration with limited blast radius.
- **Low** — information disclosure, missing hardening headers.
- **Info** — observations with no direct security impact.

## Categories (for `by_category` statistics)

Use these canonical names: `Injection`, `Authentication`, `Authorization`,
`Misconfiguration`, `Sensitive Data`, `XSS`, `SSRF`, `Cryptography`, `Other`.

## Naming

Keep finding titles concise (≤ 80 characters). Reference the affected endpoint
or component in the dedicated location/area field, not inside the title.
