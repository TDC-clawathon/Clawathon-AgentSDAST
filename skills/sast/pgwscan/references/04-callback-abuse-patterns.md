# Callback / Notify / Return URL Abuse

Goal: convince the merchant a payment succeeded when it did not — or for a different amount than was actually paid.

## The Three URLs

Most gateways use three URLs sent by the merchant during order creation:

| URL | Direction | Purpose | Trust level |
|-----|-----------|---------|-------------|
| `return_url` | Browser redirect (user) | Where the user lands after paying | LOW — user can manipulate |
| `notify_url` / `ipn_url` | Gateway → Merchant (server-to-server) | Authoritative payment confirmation | HIGH — should be the source of truth |
| `cancel_url` | Browser redirect (user) | Where the user lands on cancel | LOW |

Bug class: the merchant trusts the **return URL** (low-trust, user-traversable) for crediting the order instead of the **notify URL**. Or trusts the URL params instead of re-querying the gateway.

## Step 1 — Capture the Callback URL Pattern

From Phase 1 cancel-flow recon, you have something like:
```
https://merchant.com/payment/callback?orderId=A1&status=success&amount=100000&signature=abc...
```

Record the full URL and the parameter set.

## Step 2 — Strip-and-Replay

Most basic test:
```
https://merchant.com/payment/callback?orderId=A1&status=success&amount=100000
```

(removed `signature` entirely)

If the merchant accepts this and credits the order, you have unauthenticated callback — full bypass. The merchant trusts the URL without verifying the signature.

Variants:
- Send `signature=` (empty)
- Send `signature=null` (literal string)
- Send `signature=undefined`
- Send `signature` with random hex
- Send a signature that is the right length but all zeros

If any of these is accepted, callback signing is broken.

## Step 3 — Status Flip

Capture a real **failed** payment callback (just cancel through the gateway). It looks like:
```
?orderId=A1&status=failed&signature=xyz...
```

Mutate `status` to `success`, `paid`, `00`, `0`, or `1` (depending on the gateway's status convention). Test:

1. Keep the original signature → if accepted, status is not in the hash
2. Strip the signature → if accepted, the merchant doesn't check signatures on callbacks at all

This is one of the highest-impact bypasses across most merchant integrations: status often is in the hash per the gateway's docs but the developer forgot to validate `signature == computed_signature` on the merchant side, OR validates the signature but never actually reads the verified `status` (uses the URL value directly instead).

## Step 4 — Amount Manipulation

Send the callback with `amount=1` instead of `amount=100000`:
- Keep the original signature first (test if amount is hashed)
- Strip the signature next
- Some merchants use the URL `amount` for crediting; others re-query the gateway and use the gateway's record. Test which.

If the merchant reads `amount` from the URL: bug.

## Step 5 — Order ID Swap

Have order A1 (paid, $1) and order A2 (pending, $1000). Take the successful callback for A1 and change `orderId` to `A2`:
```
Original: ?orderId=A1&status=success&amount=1&signature=...
Mutated:  ?orderId=A2&status=success&amount=1&signature=...
```

If the merchant credits A2 as paid, you have cross-order callback abuse — paying for the cheap order credits the expensive one.

## Step 6 — Idempotency Test (Double Credit)

Send the same successful callback twice (or 100 times). Does the merchant credit the order multiple times?

This is more useful for digital goods (each credit might unlock more downloads, account credits, loyalty points) than physical goods. Test against:
- Wallet top-up flows
- Loyalty / reward point systems
- Affiliate commission systems
- Subscription extensions

## Step 7 — Direct-to-Merchant Callback Without Paying

The notify URL is usually accessible from the public internet (the gateway's servers must reach it). Try:
```
curl "https://merchant.com/payment/notify?orderId=A1&status=success&amount=100000"
```

If accepted (regardless of signature), the merchant doesn't restrict callback origin. Combined with status-flip or signature-strip, this lets you confirm any pending order without ever touching the gateway.

Verify origin restrictions:
- IP allowlist? Test from various IPs (Cloudflare, AWS).
- HTTP method? Try GET vs POST.
- Headers? Try with and without `User-Agent`, `Referer: gateway.com`.

## Step 8 — Race / TOCTOU on Callback

Some merchants check the order status BEFORE crediting (idempotency) but the check and the credit are not atomic. Send 50 parallel callback requests and observe whether multiple credits happen.

## Step 9 — Trusted-Field Hijack

Sometimes the callback includes user-controlled fields (e.g., `note`, `description`) that the merchant logs or displays without escaping. Test for:
- XSS in the callback display page (`?note=<script>...`)
- SSRF if `notify_url` itself is user-supplied during order creation (`notify_url=http://internal:8080/admin`)
- HTTP request smuggling on the notify endpoint

## Common Gateway Behaviors

| Gateway type | Common bug | Likelihood |
|--------------|------------|------------|
| Aggregator (forwarding to bank) | Merchant trusts aggregator callback without re-query | High |
| Direct bank | Merchant uses return URL for crediting | High |
| Wallet (MoMo, ZaloPay style) | Status field outside hash | Medium |
| International (Stripe, PayPal) | Webhook secret leaked or stripped | Low (but high impact) |

## Stop Conditions

Move on when:
- Stripping the signature is rejected with explicit error
- Signature is verified correctly on every field
- Status is hashed and validated
- Amount is re-queried from the gateway, not from the URL
- Idempotency is enforced atomically

Even then, try cross-order replay (Step 5) — it requires both the merchant to verify the signature AND to bind the callback to the order, which is two separate things developers sometimes get wrong.
