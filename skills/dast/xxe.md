# XML External Entity (XXE)

**CWE-611**

## Summary
An XML parser processes external entities defined in attacker-supplied XML, enabling local
file reads, SSRF, and sometimes denial of service.

## Where to look
- Endpoints accepting `application/xml` or `text/xml` bodies (SOAP, SAML, RSS, config
  import, document upload, SVG).

## How to test with this tool
- `scan_api` with `plugins:["xxe"]` posts a DOCTYPE with an external entity pointing at a
  local file and confirms when the file content is returned.
- Manual with `http_request`: set `Content-Type: application/xml` and a body like:
  ```xml
  <?xml version="1.0"?>
  <!DOCTYPE r [<!ENTITY x SYSTEM "file:///etc/passwd">]>
  <r>&x;</r>
  ```
  Inspect the response via `get_scan_logs`.

## Variations
- File read: `file:///etc/passwd`, `file:///c:/windows/win.ini`.
- SSRF via entity: `SYSTEM "http://169.254.169.254/latest/meta-data/"`.
- Blind/OOB XXE: parameter entities that fetch an external DTD from a host you control.
- SVG/Office documents that embed XML.

## Confirmation signals
- Local file content (e.g. `root:x:0:0:`) appears in the response, or an OOB callback fires.

## Not a vulnerability
- The parser rejects the DOCTYPE or does not resolve entities.

## Remediation
Disable DTDs / external entity resolution in the XML parser (e.g.
`disallow-doctype-decl`, `FEATURE_SECURE_PROCESSING`); prefer non-XML formats.
