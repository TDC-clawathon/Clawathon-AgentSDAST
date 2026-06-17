# Broken Authentication

**OWASP API2:2023 · CWE-287**

## Summary
Authentication is missing, weak, or improperly enforced, letting an attacker access
protected functionality without valid credentials.

## Where to look
- Endpoints the spec marks `secured` but that respond without a token.
- Token handling: JWTs, API keys, session cookies.
- Login, password reset, refresh, MFA, registration flows.

## How to test with this tool
- `scan_api` with `plugins:["auth"]` checks whether a secured endpoint answers 2xx with no
  credentials.
- Deeper checks with `http_request`:
  - **No token**: call a protected endpoint with no auth header → should be 401/403.
  - **Tampered token**: flip a byte in the JWT signature → should be rejected.
  - **alg:none**: re-sign the JWT with `{"alg":"none"}` and no signature → must be rejected.
  - **Expired/old token**: should be rejected.
  - **Privilege/role**: a low-priv token reaching admin functionality (BFLA).
  - **Method/route confusion**: try alternate methods or `X-HTTP-Method-Override`.

## Confirmation signals
- A protected endpoint returns the protected data/action without valid credentials.
- A forged/tampered/`alg:none` token is accepted.

## Not a vulnerability
- The endpoint is genuinely public (the spec just over-marked it). Note this rather than
  claiming a finding.

## Remediation
Enforce authentication middleware on all protected routes and fail closed. Verify JWT
signature and algorithm (reject `none`), expiry, issuer, and audience. Use short-lived
tokens and proper session management.
