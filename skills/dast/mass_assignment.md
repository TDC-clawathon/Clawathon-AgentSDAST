# Mass Assignment / BOPLA

**OWASP API3:2023 · CWE-915**

## Summary
The API binds client-supplied JSON straight onto internal objects, so an attacker can set
properties they should not control (e.g. `role`, `isAdmin`, `verified`, `balance`).

## Where to look
- Create/update bodies: `POST`/`PUT`/`PATCH` on users, profiles, orders, accounts.
- Objects with privileged or internal fields (role, permissions, status, price, ownerId).

## How to test with this tool
- `scan_api` with `plugins:["mass_assignment"]` adds undocumented privileged fields with
  distinctive sentinel values and confirms when they are reflected/persisted.
- Manual, stronger proof with `http_request`:
  1. Send a normal update body and note the response object.
  2. Resend adding a privileged field, e.g. `{"...":"...","role":"admin","isAdmin":true}`.
  3. `GET` the object back; if the privileged field stuck, it is confirmed.
- Field-name guesses: `role`, `isAdmin`, `admin`, `is_admin`, `verified`, `isPremium`,
  `accountBalance`, `permissions`, `status`, `ownerId`, `userId`.

## Confirmation signals
- The injected privileged field is accepted and reflected back / persisted with the
  attacker-chosen value (a distinctive sentinel makes this unambiguous).

## Not a vulnerability
- The server ignores/strips the extra field (it does not appear in the stored object).

## Remediation
Bind requests to explicit allow-listed DTOs; never auto-bind client JSON onto internal
models. Mark sensitive properties read-only at the API layer.
