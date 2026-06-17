# CORS Misconfiguration

**OWASP API8:2023 · CWE-942**

## Summary
A permissive Cross-Origin Resource Sharing policy lets malicious websites read
authenticated responses from the API in a victim's browser.

## How to test with this tool
- `scan_api` with `plugins:["cors"]` sends a crafted `Origin` and inspects the
  `Access-Control-Allow-Origin` (ACAO) / `Access-Control-Allow-Credentials` (ACAC) headers.
- Manual check with `http_request`: set `Origin: https://evil.example.com` and read the
  response headers via `get_scan_logs`.

## Dangerous configurations
- **Reflected Origin + credentials**: `ACAO: <your evil origin>` together with
  `ACAC: true` → an attacker site can read authenticated data. (High)
- **Reflected Origin** without credentials → weakens same-origin policy. (Medium)
- **Wildcard `ACAO: *` with credentials** → invalid/unsafe configuration.
- Trusting `null` Origin, or sloppy suffix matching (`evil-example.com`, `example.com.evil`).

## Confirmation signals
- The response reflects the attacker-chosen `Origin` in `ACAO` (especially with
  `ACAC: true`).

## Not a vulnerability
- A static allow-list of trusted origins, or CORS not enabled at all.

## Remediation
Validate `Origin` against a strict allow-list; never reflect arbitrary origins; never
combine wildcard with credentials; avoid trusting `null`.
