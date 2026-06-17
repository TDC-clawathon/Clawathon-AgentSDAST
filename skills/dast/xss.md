# Cross-Site Scripting (Reflected)

**CWE-79**

## Summary
User input is reflected into an HTML response without output encoding, so an attacker can
inject script that runs in a victim's browser.

## Where to look
- Any parameter echoed back in an `text/html` response: search terms, error messages,
  usernames, redirect/landing pages, query echoes.
- Note: APIs returning `application/json` are usually NOT vulnerable to reflected XSS by
  themselves (the browser won't execute script in a JSON response unless mis-served).

## How to test with this tool
- `scan_api` with `plugins:["xss"]`. It injects a uniquely-marked payload and confirms
  ONLY when the payload is reflected **unescaped** (`<`,`>` intact) in an **HTML** response.
- Target a specific spot with `insert_point` (e.g. `["query:q"]`).
- For tricky contexts (attribute, JS, event handler), send a tailored payload via
  `http_request` and inspect the raw body to judge whether it would execute.

## Contexts & payloads
- HTML body: `<script>alert(1)</script>`, `<svg/onload=alert(1)>`.
- Attribute breakout: `"><svg/onload=alert(1)>`, `'><img src=x onerror=alert(1)>`.
- Raw-text element (`<title>`,`<textarea>`,`<style>`): `</title><svg/onload=alert(1)>`.
- Auto-firing: `<details/open/ontoggle=alert(1)>`.
- Polyglot: `'"></script><svg/onload=alert(1)>`.

## Confirmation signals
- The exact payload appears in the response with angle brackets NOT entity-encoded, and
  the `Content-Type` is HTML.

## Not a vulnerability
- Reflection that is HTML-entity-encoded (`&lt;script&gt;`) — the app is escaping correctly.
- Reflection only inside a JSON body served as `application/json`.

## Remediation
Context-aware output encoding; set `Content-Type` correctly; deploy a strict
`Content-Security-Policy`; prefer frameworks that auto-escape.
