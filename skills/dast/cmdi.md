# OS Command Injection

**CWE-78**

## Summary
User input is passed to a shell/OS command, letting an attacker run arbitrary commands on
the server. Often the highest-impact API bug (full host compromise).

## Where to look
- Params that feed tools: `ping`/`host`/`ip`, `filename`, `format`/`convert`, `cmd`,
  archive/extract, image processing, DNS lookups, backup/export features.

## How to test with this tool
- `scan_api` with `plugins:["cmdi"]` injects separators and matches command-output
  signatures (`uid=`, `root:x:0:0:`, Windows `[fonts]`).
- If output isn't reflected (blind), use a **time-based** payload via `http_request`:
  - Unix: `; sleep 8`, `$(sleep 8)`, `| sleep 8`
  - Windows: `& ping -n 8 127.0.0.1`
  Then compare response time to a baseline request (read timings via `get_scan_logs`).

## Separators & bypasses
- `; cmd`, `| cmd`, `|| cmd`, `& cmd`, `&& cmd`, `` `cmd` ``, `$(cmd)`.
- Newline/CR injection: `\ncmd`, `%0acmd`, `\rcmd` (bypasses naive `;|&` filters).
- Quote-context breakout: `'; id; '`, `"; id; "`.

## Confirmation signals
- Command output (e.g. `uid=0(root)`, `/etc/passwd` content) appears in the response, OR
- A reliable, repeatable time delay matching an injected sleep (blind).

## Not a vulnerability
- A reflected payload with no command output and no timing effect.

## Remediation
Avoid shelling out. If unavoidable, use argument arrays (no shell interpolation), strict
allow-lists, and input validation. Never build a command string from user input.
