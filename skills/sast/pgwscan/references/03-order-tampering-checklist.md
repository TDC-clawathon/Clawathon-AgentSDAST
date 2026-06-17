# Order and Price Tampering Checklist

Goal: change a monetary or item field such that the server still accepts the order but credits less money — or accepts impossible orders.

Apply each vector individually first; combine only after each is understood. Track results in a table.

## Money-Field Mutations

### V1 — Direct numeric mutation on `amount` / `price` / `total`

Try in this order:
1. Decrease by a small amount: `100000` → `99999`, then → `1`
2. Set to `0`
3. Set to `0.01`, `0.1`, `0.001`
4. Negative number: `-1`, `-100000`
5. Very large: `999999999999`, `1e308`
6. Scientific notation: `1e2` (= 100), `1e-10` (= ~0), `1.5e2`
7. String with leading zeros: `00001`, `+1`, ` 1` (space), `1 ` (trailing space)
8. Hex / octal: `0x64`, `0100` (depending on parser)
9. Comma-separated thousands: `1,000` vs `1000`
10. Decimal locales: `1.000,00` (European) vs `1,000.00` (US)
11. Unicode digits: `１００` (full-width)

Watch the response: a generic "invalid amount" error is informative; an accepted request that displays the lowered amount on the gateway page is the win.

### V2 — Currency swap

**Caveat:** most merchants validate `currency` against a catalog whitelist; an unknown currency rejects at create-order. This vector works only when the merchant blindly trusts user-supplied `currency` AND the gateway processes the charge at the requested unit value without converting back. Realistic hit-rate is low; try this AFTER V1 / V4 / V7.

When testing, try low-unit-value currencies the merchant might plausibly support:
- IDR (Indonesian Rupiah, ~16,000 / USD)
- VND (Vietnamese Dong, ~25,000 / USD)
- KHR (Cambodian Riel, ~4,100 / USD)
- LAK (Lao Kip, ~21,000 / USD)
- IRR (Iranian Rial, ~42,000 / USD)
- Crypto / token symbols if the merchant accepts them — the rate may be stale or attacker-influenced

Watch for these failure-but-still-bug patterns:
- Merchant accepts the currency code but charges the same numeric amount in the new currency (e.g. `100 USD` → `100 VND` ≈ $0.004)
- Gateway shows the new currency on its page but settles to merchant in the original currency at a stale rate
- Catalog whitelist enforced on the merchant frontend only; backend accepts any currency (test the API directly, not the UI)

### V3 — Quantity manipulation

`quantity` is often less validated than `price`:
- Negative: `-1` → may invert total
- Zero: `0` → may pass through with `total = 0`
- Fractional: `0.001`, `0.5`
- Extreme: `999999999`
- Type confusion: `"1"` vs `1` vs `[1]`

### V4 — Total override when total is sent separately

If both `price` and `total` are sent, change `total` to a lower value while leaving `price` correct. Server may use the request's `total` instead of recomputing it.

### V5 — Sub-item / line-item manipulation

If the order has `items[]`:
1. Add a new item with `price=0` and `quantity=10000`
2. Set existing item `price` to 0
3. Swap the cheap and expensive items between main and sub-item slots
4. Send the same item twice with conflicting prices — see which wins
5. Add an item with a known-valid `item_id` from another product (lower-priced)

### V6 — Coupon / voucher abuse

1. Apply the same coupon twice in parallel (race condition before redemption count increments)
2. Modify `discount_amount` directly while leaving the coupon code valid
3. Apply a percentage coupon (`-10%`) and change the percentage to `100` or `1000`
4. Apply a coupon that doesn't apply to this product but the server doesn't enforce
5. Use a coupon code from one user account on another account
6. Stack coupons that aren't supposed to be stackable

### V7 — Hidden / undocumented fields

Inspect JSON responses and JS bundles for fields the UI does not normally send:
- `internal_price`, `wholesale_price`, `cost`
- `manual_override`, `admin_amount`, `force_amount`
- `tax_exempt`, `vip_discount`, `staff_price`
- `currency_rate`, `exchange_rate`

Try sending each one in the create-order request. Servers often have fields the UI never exposes that nonetheless take effect when sent.

### V8 — Type juggling

Useful when the server is loosely typed (PHP, JavaScript, Python with bad input handling). Pure type-confusion only — does NOT include injection (see V8b for that):
- `price=100` → `price[]=100` (array — many backends pick first/last element or coerce to "Array" string)
- `price=100` → `price=true` (boolean → 1 in some languages)
- `price=100` → `price=null` (null → 0 in many comparisons)
- `price=100` → `price=` (empty string → 0 / NaN)
- JSON: send `"price": "100"` vs `"price": 100` vs `"price": ["100"]` vs `"price": {"value": 100}` vs `"price": -100`
- PHP loose-comparison quirks: `"0e1234567"` == `"0e9876543"` (both equal `0` numerically); `"100abc"` → `100` via string-to-int coercion
- Magic floats: `0.1 + 0.2 != 0.3` — accumulated rounding can let `99.99999...` pass a `< 100` check

### V8b — Injection in money/order fields (separate from type juggling)

Distinct attack class — these target SQLi / NoSQLi / template injection in order endpoints, not just type confusion:
- NoSQL: `price={"$ne": null}`, `price={"$gt": 0}`, `quantity={"$where": "this.qty > 0"}` (MongoDB)
- SQLi in numeric context: `price=1 OR 1=1`, `price=1; UPDATE orders SET status='paid'--`, `quantity=1 UNION SELECT NULL`
- Boolean SQLi: `price=1 AND 1=1` vs `price=1 AND 1=2` — differential timing or response
- Template injection: `price={{7*7}}`, `price=${7*7}`, `price=#{7*7}` if amount is rendered into emails/PDFs
- LDAP / XPath injection if the merchant's order lookup uses those backends

Treat these as their own bug class — even if not money-mutation, an SQLi on a payment endpoint is a critical finding.

### V9 — Last-mile mutation on the gateway page

Even if the create-order step is correctly signed, the gateway redirect URL is a separate signed request. Mutate the `amount` on the gateway URL and resubmit — sometimes the gateway trusts a signature it shouldn't, or the merchant doesn't verify the final amount on callback.

### V10 — Race conditions

1. Submit two orders for the same item simultaneously when stock is 1
2. Apply and then revoke a coupon; submit the order in between
3. Pay for an order while another tab cancels it
4. Trigger a refund while the original payment is still settling

## Pre-Phase Checklist

Before mutating money, capture and record:
- The exact create-order request (full headers and body)
- The gateway redirect URL with all params
- The "amount" displayed on the gateway page
- The callback URL on success
- The final invoice / receipt page

After every mutation, watch all four for divergence — the bug often appears as inconsistency between two of these.

## Result Table Template

| # | Field | Original | Mutated | Server response | Gateway page amount | Final invoice amount | Bug? |
|---|-------|----------|---------|-----------------|---------------------|----------------------|------|
| 1 | price | 100000 | 1 | 200 OK, no error | 1 VND | 1 VND | YES |
| 2 | price | 100000 | -1 | 400 invalid amount | n/a | n/a | NO |
| 3 | currency | VND | IDR | 200 OK | 100000 IDR | 100000 IDR | partial |

The "Bug?" column gets a YES when at least one of the four observed values is mismatched in the attacker's favour.

## Stop Conditions

Stop tampering and move to Phase 4 (callback abuse) when:
- All money fields validate correctly
- All sub-item swaps are rejected
- Hidden fields produce no effect
- Race conditions are properly serialized

Phase 4 (callback) is historically the most productive next step on merchant integrations — even when forward-request tampering fully fails.
