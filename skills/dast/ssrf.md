# Server-Side Request Forgery (SSRF)

**OWASP API7:2023 · CWE-918**

## Summary
The server fetches a URL supplied by the user, letting an attacker make it request
internal services, cloud metadata, or arbitrary hosts.

## Where to look
- URL-like params: `url`, `uri`, `link`, `callback`, `webhook`, `image`, `avatar`,
  `feed`, `proxy`, `dest`, `redirect`, `target`, `next`. Also import-from-URL and
  link-preview features.

## How to test with this tool
- `scan_api` with `plugins:["ssrf"]` points the param at cloud metadata / loopback / file
  schemes and confirms ONLY when the fetched resource's content is returned (not when the
  URL is merely echoed — reflections are stripped before matching).
- For blind SSRF (no content returned), use `http_request` to probe behavioural signals:
  differing status/timing for internal vs external/non-routable hosts; or use an
  out-of-band collector you control and watch for the callback.

## Targets & bypasses
- AWS IMDS: `http://169.254.169.254/latest/meta-data/` (and `/iam/security-credentials/`).
- GCP: `http://metadata.google.internal/computeMetadata/v1/` (needs `Metadata-Flavor: Google`).
- Loopback/internal: `http://127.0.0.1/`, `http://localhost/`, internal hostnames.
- IP-encoding bypasses of 169.254.169.254: decimal `2852039166`, hex
  `0xa9.0xfe.0xa9.0xfe`, IPv6-mapped `[::ffff:169.254.169.254]`.
- Schemes: `file:///etc/passwd`, `gopher://`, `dict://`.

## Confirmation signals
- Internal resource content in the response (e.g. EC2 `ami-id`, `instance-id`,
  `iam/security-credentials`, `/etc/passwd` lines).

## Not a vulnerability
- The server validates and rejects internal/loopback URLs, or only echoes the URL.

## Remediation
Allow-list outbound hosts/schemes; block link-local (169.254/16), loopback, and private
ranges; resolve and re-check the IP after DNS; disable unused URL schemes; require
IMDSv2.
