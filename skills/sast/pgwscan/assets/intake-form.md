# Engagement Intake Form

Fill in / mark MISSING / mark N/A. Paste back to Claude to begin Phase 1.

## Captured Artifacts

### Create-order request (REQUIRED)

```
<paste Burp "Copy as curl" here>
```

### Redirect URL to gateway (REQUIRED)

```
<paste the redirect URL the merchant returned after create-order>
```

### Cancel-flow callback URL (STRONGLY RECOMMENDED)

```
<paste the URL the browser landed on after cancel>
```

### Successful callback (OPTIONAL — only if low-cost transaction was paid)

```
<paste full callback URL or POST body>
```

## Target / Gateway Context

- Gateway provider name (if known): ____________
- Gateway public docs URL (if found): ____________
- Sandbox / test credentials available?: yes / no
- Other merchants on the same gateway you can cross-reference: ____________

## Optional / Recon

- gh-recon results (any leaked SDK / sample code): ____________
- Specific vulnerability classes you suspect: ____________
- Anything unusual observed in the flow (e.g., extra params in JS, hidden fields): ____________

## Constraints / Practical

- Time-box for this engagement: ____________
- Communication preference (sync handoffs / async / specific hours): ____________
- Tooling on hand (Burp, Caido, mitmproxy, agent-browser, Playwright): ____________
