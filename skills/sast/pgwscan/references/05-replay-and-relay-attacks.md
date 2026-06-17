# Replay and Relay Attacks

Goal: pay $1 once, get credited as if you paid $1000 — repeatedly.

This document covers the multi-transaction abuse patterns that complement Phase 4's single-callback bugs.

## Replay Attack — Reuse the Same Payment Twice

Pattern:
1. Create order O1, pay $1 successfully. Capture the success callback `C1`.
2. Create order O2 ($1, same product). Resend `C1` to the merchant — does the merchant credit O2?

If the merchant's idempotency key is the **transaction ID** (from the gateway), but does not bind it to the **order ID**, you can replay the same `C1` against many different orders. The transaction ID has been used before, but the order has not — depending on the check, the merchant may credit.

Variant — temporal replay:
1. Capture `C1` for order O1.
2. Wait 24 hours.
3. Resend `C1`. Some merchants only deduplicate within a session or a short window.

## Relay Attack — Cross-Amount Confirmation

The hallmark Vietnamese payment-bypass bug. Pattern:
1. Order A1 = $1 for "Cheap Item". Pay it. Capture success callback `C(A1, $1, success)`.
2. Order A2 = $1000 for "Expensive Item". Do NOT pay. The order is in "pending" state.
3. Replay `C(A1, $1, success)` with `orderId` swapped to A2: `C(A2, $1, success)`.
4. If the merchant credits A2 because "transaction succeeded", you have full bypass.

Why this works in practice:
- Many merchants use `signature(amount, status, orderId, ...)` — but the developer copies the verification code from the gateway's docs and only verifies the signature, not whether the `amount` field equals the order's expected amount.
- The merchant code path: `if signature_valid && status == "success" → mark order paid`. No check that `amount` ≥ `order.total_due`.

## Relay Attack — Cross-Merchant

Less common but devastating when it works:
1. Identify two merchants M1 and M2 using the same payment gateway.
2. Pay M1 for $1.
3. Capture the gateway's success callback before it reaches M1.
4. Replay it (with `merchantId` adjusted) to M2.
5. M2 receives "payment succeeded" for an order that was never created on M2 — but if M2 has any "pay then create order" flow or auto-credits unrecognized callbacks, bug.

Requires gateway-level signing weakness or a leak between merchants. Rare on well-designed gateways.

## Relay Attack — Status Upgrade

1. Pay $1. Get `C(O1, $1, success)`.
2. Cancel order O2 ($1000) at the gateway. Get `C(O2, $1000, cancelled)`.
3. Splice the two: send `C(O2, $1000, success)` with O1's signature (won't validate cleanly, but if signature is missing or stripped, it's accepted).
4. Or: take the `success` status from C1 and the `(orderId, amount)` from C2 and combine.

The success of this depends on Phase 4's discoveries — if status is in the hash, you need to bypass that first.

## Idempotency Bypass

Even when the merchant has idempotency, common implementations are flawed:
- **Idempotency key = transaction ID alone**: replay against different orders works.
- **Idempotency key = order ID alone**: pay-once-credit-many works against the same order.
- **Idempotency check happens AFTER credit**: race condition — fire 50 callbacks in parallel.
- **Idempotency key includes timestamp**: change the timestamp on each replay.

## Capturing Callbacks Without Spending Money

If you absolutely cannot spend money on the test:
1. Use the cancel flow to capture failed-payment callbacks (Phase 1).
2. Use a sandbox / test merchant if the gateway provides one.
3. Use third-party leaked data (search GitHub for `secureCode=` and successful callback URLs from open issue trackers).
4. Synthesize a callback from the documented format if you've reversed the signature scheme.

Once you have a real successful callback for any small amount, all relay variants become testable.

## Test Matrix

| Test | Setup | Mutation | Expected vuln |
|------|-------|----------|---------------|
| Same-order replay | C1 captured | Resend C1 unchanged | Double-credit |
| Cross-order relay | C1 + pending O2 | Swap orderId in C1 | Credit O2 |
| Amount upgrade | C1 ($1) + pending O2 ($1000) | Swap orderId, keep amount=$1 | Credit O2 with $1 paid |
| Status upgrade | Cancel callback + success | Splice status | Credit cancelled order |
| Cross-merchant | M1 + M2 same gateway | Swap merchantId | Credit M2 |
| Idempotency race | Single C1 | Fire 50 in parallel | Multiple credits |

## When This Stops Working

Move on to Phase 3 (price tampering at create-order time) when:
- Idempotency is keyed on `(orderId, transactionId)` and atomic
- Merchant re-queries the gateway by transaction ID for canonical state
- Amount in callback is verified ≥ amount due
- Status upgrades are explicitly forbidden ("cannot transition from cancelled to success")

The defense-in-depth checklist on the merchant side is long, and most merchants miss at least one.
