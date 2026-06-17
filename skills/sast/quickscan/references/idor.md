# IDOR / BOLA (Broken Object-Level Authorization)

**OWASP API1:2023 · CWE-639 / CWE-284**

An endpoint exposes an object by an identifier (id, uuid, slug, filename) and
serves/modifies it **without verifying the caller owns or may access it**.

## Where to look in the code
- Handlers that read a path/query/body id and immediately query by it:
  `WHERE id = ?`, `findByID(id)`, `repo.Get(id)` — with **no**
  `AND owner_id = currentUser` and no role/tenant check.
- Auth that only proves *authentication* (valid JWT/session) but never
  *authorization* for the specific object.
- Sequential or guessable ids (auto-increment int, v1/v7 UUID with timestamp).
- Mass-assignment siblings: updating `owner_id`, `role`, `account_id` straight
  from the request body.

## Signals that raise severity
- Object is sensitive (orders, payments, PII, documents).
- Identifiers are enumerable.
- Write/delete operations, not just read.

## Test cases for the report
- Authenticate as user A, then request ids belonging to user B (read, update,
  delete). Enumerate ids (`1,2,3…` or UUIDv7 by timestamp prefix).
- Swap `account_id`/`owner_id` in the body to another tenant.
- Remove/short-circuit the auth header and re-test (authn vs authz).

## In the enriched OpenAPI
Add `x-auth` describing exactly who may call it and whether ownership is checked,
e.g. `x-authorization: owner-only (NOT enforced — orders.go:66)`.
