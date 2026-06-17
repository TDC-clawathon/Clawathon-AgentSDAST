# Sensitive Data / Excessive Data Exposure

**OWASP API3:2023 · CWE-200**

## Summary
Responses leak secrets or more data than the client needs — API keys, private keys,
tokens, credit cards, SSNs, internal fields, other users' PII.

## Where to look
- List/detail endpoints that return full objects (over-fetching).
- Error/debug responses, stack traces, verbose health/info endpoints.
- Config/export/backup endpoints.

## How to test with this tool
- `scan_api` with `plugins:["sensitive_data"]` matches high-signal patterns (private keys,
  AWS/Google/Slack/GitHub tokens, credit-card and SSN formats, `"password":` in bodies).
- Manual review with `http_request` + `get_scan_logs`: read full responses and judge
  whether fields are excessive (e.g. password hashes, internal flags, other users' data).

## Confirmation signals
- A real secret/credential or regulated PII present in a response the caller should not
  receive.

## Not a vulnerability / be careful
- Generic email addresses or the caller's own data are usually expected (do not flag PII
  indiscriminately). Focus on secrets and cross-user/excessive exposure.

## Remediation
Return only the fields the client needs (response DTOs/allow-lists); never serialize whole
ORM objects. Remove secrets from responses, logs, and errors; disable debug output in prod.
