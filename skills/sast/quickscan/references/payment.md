# Payment & Pricing Business-Logic Flaws

**OWASP API6:2023 (unrestricted access to sensitive business flows) · API3 (BOPLA)
· CWE-840 / CWE-20 / CWE-190**

These are logic bugs in how price, quantity, discounts and currency are computed
and trusted. They rarely show up as a single tainted sink — you must read the
**calculation** and ask "what does the server trust from the client?".

## 1. Price tampering (client-supplied price) — formula injection
The server computes `total = price * quantity` using a **price that came from the
request** instead of looking it up from the catalog/DB.
- Look for: `price` / `unit_price` / `amount` read from the request body and used
  in the total; checkout that echoes back client `line_items[].price`.
- Tests: send `price=0`, `price=0.01`, `price=-100` (credit), a different
  product's cheap price.

## 2. Quantity tampering — formula injection
`quantity` is trusted in the total or in inventory math without bounds/sign/parse
checks.
- Look for: `total = price * qty` with no `qty >= 1` / max check; `strconv.Atoi`
  without validation; quantity used to compute a discount.
- Tests: `quantity=0`, `quantity=-1` (negative line → reduces total), fractional
  `quantity=0.0001`, huge values (see overflow).

## 3. Integer overflow / precision
`price * quantity` overflows a fixed-width int, or floats lose precision, wrapping
a huge total to a small/negative number.
- Look for: `int`/`int32` money math; `qty` parsed into int then multiplied;
  money stored as float; no overflow guard.
- Tests: `quantity=4294967296`, `quantity=99999999999999999999`,
  values near `2^31`/`2^63`; combine with price to force wrap.

## 4. Coupons / discounts
- **Stacking:** same or multiple coupons applied repeatedly (`coupon[]` array,
  no idempotency).
- **Negative / >100% discount:** discount not clamped to `0..subtotal`, making the
  total negative (a refund).
- **No ownership/validity check:** expired, other-user, or single-use coupons
  reused; race condition redeeming the same code twice.
- Look for: `total -= discount` without `if discount > subtotal` clamp; coupon
  lookup without `expires_at`/`used` checks.
- Tests: apply the same code N times; `discount=200%`; expired/foreign code;
  concurrent redeem (race).

## 5. Currency confusion
Amount is trusted with a **client-supplied currency**, or currency is ignored so
`100 JPY` is charged as `100 USD` (or refunded in a stronger currency).
- Look for: `currency` taken from the request and not validated against the order;
  totals compared/added across currencies without conversion; missing minor-unit
  handling (JPY has 0 decimals, USD 2).
- Tests: change `currency` between order creation and payment/refund; pay in a
  weak currency, refund in a strong one; mix currencies in one cart.

## General checks
- Recompute everything server-side from trusted catalog data; never trust client
  price/total/currency.
- Validate sign, bounds, and parse errors on every numeric field.
- Make coupon redemption atomic and idempotent.

## In the enriched OpenAPI
Flag every money/quantity/coupon/currency field with `x-business-rule` describing
what the code enforces (or fails to), e.g.
`x-business-rule: total computed from client price (checkout.go:41) — NOT looked up`.
