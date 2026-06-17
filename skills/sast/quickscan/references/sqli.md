# SQL Injection

**OWASP API8:2023 · CWE-89**

User input reaches a SQL query as **code** instead of a bound parameter.

## Where to look in the code
- String building around queries: `"... WHERE x = '" + input + "'"`,
  `fmt.Sprintf("... %s ...", input)`, template literals, `.Raw(...)`,
  `db.Query(query)` where `query` was concatenated.
- Inputs that often slip through: `ORDER BY`/`sort`, `LIMIT`/`offset`, column or
  table names, `IN (...)` lists, `LIKE` patterns, JSON path operators — these
  frequently **can't** be parameterized so devs concatenate them.
- ORMs used unsafely: raw fragments, `Where("status = " + s)`, `Exec` with
  interpolated strings.
- Declared `enum` in the spec but **not** enforced server-side before the query.

## Confirm it's exploitable
- Is the value parameterized (`?`, `$1`, named binds)? If yes → not SQLi.
- Identifier contexts (ORDER BY/column) need allow-list validation, not binds.

## Test cases for the report
- Boolean: `' OR '1'='1`, `') OR ('1'='1`.
- Error-based: `'`, `"`, `\`.
- Union: `' UNION SELECT NULL,NULL-- -` (match column count).
- Time-based (blind): `' OR SLEEP(3)-- -`, `'; SELECT pg_sleep(3)-- -`.
- Destructive (note, don't run in prod): `'; DROP TABLE orders;-- -`.
- ORDER BY: `?sort=(CASE WHEN (1=1) THEN id ELSE name END)`.

## In the enriched OpenAPI
On affected params add `x-sink: sql (concatenated, orders.go:34)` and tighten the
schema to the real allow-list (`enum`, `pattern`).
