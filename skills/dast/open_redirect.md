# Open Redirect

**CWE-601**

## Summary
A parameter controls the destination of a redirect without validation, so an attacker can
send victims to a malicious site (phishing) or steal OAuth tokens/codes via the redirect.

## Where to look
- Redirect params: `redirect`, `redirect_uri`, `url`, `next`, `return`, `returnTo`,
  `continue`, `goto`, `dest`, `callback`. Common on login/logout/OAuth flows.

## How to test with this tool
- `scan_api` with `plugins:["open_redirect"]` injects an attacker host and confirms when
  the `Location` header (3xx) or a client-side redirect targets it.
- Manual with `http_request` (do not follow redirects): set the param and read the
  `Location` response header via `get_scan_logs`.

## Payloads & bypasses
- `https://evil.example.com`, scheme-relative `//evil.example.com`.
- `https:/evil.example.com` (missing slash), backslash `/\evil.example.com`.
- `https://trusted.com@evil.example.com`, `https://evil.example.com#trusted.com`.
- For OAuth: a `redirect_uri` not on the registered allow-list that still receives the code.

## Confirmation signals
- A 3xx with `Location` pointing at the attacker host, or a body-based redirect (meta
  refresh / `location.href`) to it.

## Not a vulnerability
- The app only redirects to relative paths or an allow-list of hosts.

## Remediation
Redirect only to an allow-list of internal paths/hosts; reject absolute and
scheme-relative external URLs; for OAuth, strictly match registered `redirect_uri`s.
