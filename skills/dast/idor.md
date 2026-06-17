# IDOR / BOLA — Broken Object Level Authorization

**OWASP API1:2023 · CWE-639**

## Summary
The API exposes objects by an identifier (numeric id, UUID, slug, filename) and
authorizes the *action* but not the *object*. An authenticated user can read or modify
another user's object simply by changing the id.

## Where to look
- Path ids: `GET /api/users/{id}`, `GET /rest/basket/{id}`, `/orders/{id}`.
- Query/body ids: `?account_id=`, `{"userId": 5}`.
- Indirect refs: filenames, document keys, invoice numbers, sequential or guessable UUIDs.
- Especially endpoints marked secured in the spec but returning per-user data.

## How to test with this tool
1. `scan_api` with `plugins:["idor"]` flags *candidates* (a secured endpoint returning
   distinct objects for sequential ids). This is a lead, not a confirmation.
2. **Confirm with two identities using `http_request`:**
   - As user A (send A's `Authorization`/`Cookie` via `headers`), read an object you own,
     e.g. `GET /api/basket/1` → note the data.
   - Still as user A, request another id you should *not* own, e.g. `GET /api/basket/2`.
   - If A receives B's data (HTTP 200 with B's object), it is confirmed BOLA.
3. Also test write/delete: `PUT`/`DELETE` on another user's object id.
4. Try id formats: decrement/increment, 0, very large, another tenant's UUID.

## Confirmation signals
- 200 + an object that belongs to a different user/tenant than the caller.
- A write/delete on a foreign id succeeds (200/204) and persists.

## Not a vulnerability
- Public resources (product catalog, static content) returning data for any id.
- The endpoint returns 401/403/404 for ids you don't own (proper authorization).

## Remediation
Enforce a per-object ownership check on every request, scoped to the authenticated
principal (`WHERE owner_id = :current_user`). Do not rely on unguessable ids.
