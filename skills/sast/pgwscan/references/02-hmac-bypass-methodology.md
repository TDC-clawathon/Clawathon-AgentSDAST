# HMAC / Signature Bypass Methodology

Goal: defeat or sidestep the integrity check that protects payment parameters, using only blackbox interaction.

## Core Idea

A signed request looks like:
```
amount=100&order_id=A1&...&signature=<hash of (amount, order_id, ..., secret)>
```

The blackbox attacker does not know the secret. But four classes of bug make the signature defeatable without it:

1. **Coverage gaps** — one or more sent params are not included in the hash. Mutating them succeeds with no signature change.
2. **Boundary ambiguity** — the formula concatenates values without preserving boundaries (no name=value, no escape on the delimiter), so chars can be shifted between adjacent fields with no hash change.
3. **Algorithm weakness** — MD5/SHA1 length-extension or hash-collision against a non-keyed construction.
4. **Stripping acceptance** — the server accepts the request when the signature param is absent or empty.

Classes 1 and 2 are the common findings on real merchant integrations; class 4 is rare-but-trivial; class 3 is rare in modern HMAC schemes. Test 1 → 4 → 2 → 3 in that order for time-efficiency.

## Five Hypotheses to Test

For each, design one minimum-mutation test. Run it. Record the response.

### H1 — Param Order

Possible orders: alphabetical (case-sensitive or case-insensitive), declaration order in the request, fixed order from the SDK. Test by reordering two params in the request body while keeping the signature unchanged. If the response succeeds, param order in the request does not feed the hash directly — the server sorts before hashing.

### H2 — Delimiter

Common delimiters: `|`, `:`, `&`, `,`, ` ` (space), `;`, `\n`, or none.

Test by inserting one of those characters inside a value where the value is user-controlled (e.g., `description`). If a `|` appears in `description` and the signature still validates, the server is using `|` as the delimiter and is vulnerable to a delimiter-injection variant of the shift attack.

### H3 — Param Name Inclusion

Two formulas exist:
- **Values only**: `value1|value2|value3|secret`
- **Name + value**: `name1=value1&name2=value2&...&secret`

Test by renaming a param while keeping its value (e.g., `price` → `pri ce`) — if the signature still validates, names are not in the hash. (More commonly: leave the name alone but watch the shift test below — if the shift works, the formula is values-only.)

### H4 — Coverage

The most productive hypothesis. For each param in the request, mutate one at a time and resubmit with the original signature. Record which mutations are accepted.

Critical params to test (in priority order):
1. `description` / `desc` / `transaction_info` / `note` — frequently omitted
2. `return_url`, `cancel_url`, `notify_url` — sometimes only one is hashed
3. `merchant_email`, `buyer_info` — long string fields are often skipped
4. `tax`, `discount`, `fee_shipping`, `fee_cal` — secondary money fields
5. Any param that is not always present (optional fields)

If even one important param is unhashed, the bug exists. Combine with order tampering (Phase 3).

### H5 — Secret Placement

The secret is appended at the end (`...|values|secret`), prepended (`secret|values...`), or wedged in the middle. Length-extension attacks against MD5/SHA1 work on the **append** form for non-HMAC constructions; HMAC proper is not vulnerable. This matters less for blackbox (you cannot do length extension without knowing the secret length), but is worth noting if the gateway uses a custom non-HMAC scheme.

## The Shift Test (highest-yield single technique)

Take a request where two adjacent params are user-controlled or include a string field. Move one character from the first into the second.

Example: original
```
transaction_info=Pay+for+order+12345&order_code=A1&...&secure_code=abc
```

Mutated:
```
transaction_info=Pay+for+order+1234&order_code=5A1&...&secure_code=abc
```

If the response is accepted, you have proved:
- The signature concatenates values without preserving boundaries
- You can rebalance characters between fields while keeping the hash valid

Now you can:
- Move characters out of `price` (lowering the numeric value the merchant sees) into a neighbouring text field that is later discarded
- Move characters out of `transaction_info` and into `order_code` to inject a controlled order code
- Construct payloads where the server's parser sees a different `price` than the hash was computed over

## Cancel-Flow Verification

Most signature checks happen at the moment the merchant redirects to the gateway. You can test signature acceptance without paying:

1. Build the mutated request URL.
2. Send it as the redirect (paste into browser or replay in Burp).
3. If the gateway shows the payment selection page → signature accepted (and your hypothesis is correct).
4. Click "Cancel" → no money spent.
5. Move on to building the full exploit.

This means signature reverse-engineering is essentially free (no monetary cost) — you can iterate as much as needed.

## When the Signature Stops Validating

If a mutation breaks the signature, the failure mode tells you something:

| Symptom | Meaning |
|---------|---------|
| HTTP 200 with "Invalid signature" message | The mutated field IS in the hash |
| Redirect to error page with `error_code=signature` | Same — the field is hashed |
| Redirect to original error page (generic) | Possibly a different validation failed; not conclusive |
| Request silently dropped, no error | Try another mutation pattern; possibly server-side state validation |
| Mutation accepted (gateway shows payment page) | The field is NOT hashed → exploit primitive found |

## When to Stop

If after testing all 5 hypotheses against every interesting param the signature still appears to cover everything correctly:

1. Move to Phase 4 (callback abuse) — even a perfectly signed forward request may still be defeated by a forgable callback.
2. Re-examine Phase 1 — did you find ALL request endpoints? Often a "create order" endpoint is unsigned and only the redirect is signed.
3. Move on to a different target. Time-box signature reversing to ~2 hours per target; if no progress, the gateway is probably correctly implemented and the bug lives elsewhere (price tampering on an unsigned step, business-logic on coupons, race conditions, etc.).

## Notes for Blackbox Testers

- You will need to guess the join-character used between values. Try `|`, `:`, `&`, `,`, space, `;` in that priority order.
- You will need to guess whether param names are in the hash. The shift test resolves this implicitly.
- You will need to guess the param ordering (sort? declaration?). Try mutating order in the URL — if the signature still validates, the server is sorting.
- You will need to identify the set of params NOT in the hash. The single-param mutation sweep is exhaustive but slow; prioritize description-like fields first.
- All of this is "form a hypothesis → run one minimum-change test → log result." Resist the urge to mutate multiple things at once.
