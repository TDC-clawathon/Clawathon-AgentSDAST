# Cross-Site Scripting (XSS)

**OWASP API · CWE-79**

User input is reflected into an HTML/JS/JSON response (or stored and later
rendered) without context-correct encoding.

## Types
- **Reflected:** input echoed back in the same response (search, error messages).
- **Stored:** input persisted then rendered to other users (comments, profile
  names, product descriptions).
- **DOM:** client-side JS writes input into the DOM (`innerHTML`, `document.write`).

## Where to look in the code
- Responses that interpolate input into HTML without escaping: `fmt.Fprintf(w,
  "<div>"+q+"</div>")`, templates rendered with autoescaping **off**
  (`text/template` instead of `html/template`; `{{. | safe}}`; `dangerouslySet
  InnerHTML`).
- `Content-Type: text/html` on endpoints that echo input.
- JSON endpoints later rendered unescaped by a client; reflected values in error
  strings.
- Missing/permissive CSP; reflected `Location`/redirect values.

## Test cases for the report
- `<script>alert(1)</script>`, `"><svg onload=alert(1)>`.
- Attribute break-out: `" onmouseover=alert(1) x="`.
- JS context: `';alert(1);//`.
- Stored: submit the payload via the create/update endpoint, then fetch the read
  endpoint that renders it.

## In the enriched OpenAPI
Mark reflected/stored fields with `x-sink: html-reflect (search.go:20)` and note
the response Content-Type and whether output encoding is applied.
