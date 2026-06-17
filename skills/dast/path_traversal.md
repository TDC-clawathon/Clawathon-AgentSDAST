# Path Traversal / Local File Inclusion

**CWE-22**

## Summary
A parameter used to build a file path lets an attacker escape the intended directory with
`../` sequences and read (or sometimes write) arbitrary files.

## Where to look
- File-ish params: `file`, `path`, `name`, `template`, `download`, `doc`, `page`, `lang`,
  `image`, `attachment`, `include`.

## How to test with this tool
- `scan_api` with `plugins:["path_traversal"]` requests well-known files and matches
  system-file signatures (`root:x:0:0:`, `[fonts]`).
- Target the param with `insert_point` (e.g. `["query:file"]`).
- For unusual encodings/contexts, craft a payload and use `http_request`, then inspect the
  raw body via `get_scan_logs`.

## Encodings & bypasses
- Plain: `../../../../etc/passwd`, deep `../` chains.
- Filter bypass (stripped `../` reconstructed): `....//....//etc/passwd`.
- URL-encoded `..%2f..%2f`, double-encoded `..%252f..%252f`, overlong `..%c0%af`.
- Null-byte truncation (legacy): `../../etc/passwd%00`.
- Absolute paths / schemes: `/etc/passwd`, `file:///etc/passwd`, `/proc/self/environ`.
- Windows: `..\..\..\windows\win.ini`, `..%5c..%5c`.

## Confirmation signals
- The response contains the contents of a system file (e.g. `root:x:0:0:` from
  `/etc/passwd`, or `[fonts]` from `win.ini`).

## Not a vulnerability
- The traversal payload is echoed but no file content is returned.

## Remediation
Canonicalize and validate the resolved path against an allow-list/base directory; reject
`..`; never build file paths directly from user input.
