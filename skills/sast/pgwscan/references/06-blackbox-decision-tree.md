# Blackbox Decision Tree

Goal: pick the right technique given the symptom you observe. Use this when stuck.

## Starting Symptom: "I have a captured payment request and don't know where to start"

```
Is the gateway provider documented publicly?
├── YES → Read the docs. Skip Phase 2 hypothesis-testing; verify the docs match live behavior.
└── NO  → Use gh-recon to dork for SDK code, sample integrations, leaked merchant code.
            └── Still nothing? → Phase 2 from scratch.

Does the request have a signature parameter?
├── YES → Phase 2 (signature reverse) AND Phase 4 (callback bypass)
└── NO  → Phase 3 (direct tampering) — this is rare and lucky
```

## Symptom: "Mutating `price` returns 'invalid signature'"

```
The price IS in the hash. Try in order:
1. Phase 2 → shift test (move chars between adjacent values)
2. Phase 2 → identify unhashed params (check description, transaction_info, note)
3. Phase 3 → mutate currency instead of price
4. Phase 4 → maybe price is enforced on forward but not callback
```

## Symptom: "Mutating any field returns 'invalid signature'"

```
1. Confirm: is the signature itself in the hash? (Test by mutating signature → expect failure)
2. Try modifying ONLY the signature → does the response say something different?
   - "missing signature" if stripped → Phase 4 step 2 (strip-and-replay)
   - "expired signature" → check for timestamp param, try replaying old request
3. Move attention to callback (Phase 4) — server-side verification of callback often weaker
4. Check unsigned steps in flow — maybe create-order is unsigned, only redirect is signed
```

## Symptom: "I see a `desc` / `note` / `transaction_info` field with arbitrary text"

```
HIGH PRIORITY — these fields are usually unhashed.
1. Mutate desc to a value containing the gateway's delimiter (|, :, &, space)
   → if accepted, you've confirmed unhashed AND have an injection primitive
2. Try the shift attack: move price digits into desc
3. Try injecting fake parameters via desc (e.g., desc=foo&price=1)
```

## Symptom: "Cancel button gives me a callback URL with no signature"

```
1. Take that URL, change status to success → strip-and-replay (Phase 4 step 2)
2. Take that URL, change orderId to a pending high-value order → relay (Phase 5 step 5)
3. Take that URL, replay it twice → double-credit (Phase 4 step 6)
```

## Symptom: "I have a successful callback for a $1 payment"

```
Highest-value capture. Test in order:
1. Replay it (Phase 5) — does it credit again?
2. Change orderId to a pending $1000 order (Phase 5) — relay
3. Lower amount to $0 — does merchant credit anyway?
4. Strip signature — accepted?
5. Wait 24h, replay — temporal idempotency window?
```

## Symptom: "The gateway is well-known (VNPay, MoMo, Stripe)"

```
1. The gateway itself is probably correct. The bug lives on the merchant integration.
2. Read the gateway's official docs — confirm signature format.
3. Look for merchant-side mistakes:
   - Does the merchant verify the signature on callback?
   - Does the merchant verify the amount matches the order?
   - Does the merchant treat return_url and notify_url as equivalent?
   - Does the merchant have idempotency on (orderId, transactionId)?
4. Test each of the four merchant-side mistakes (Phase 4).
```

## Symptom: "There's a coupon / voucher / discount system"

```
1. Apply coupon, capture the request, mutate the discount value.
2. Apply, then immediately retract the coupon — race the order.
3. Apply two coupons not meant to stack.
4. Send coupon in `coupon_code` AND `discount_amount` simultaneously.
5. Apply a percentage coupon, change to a higher percentage or absolute amount.
6. Apply a coupon for a different product category — does it accept?
```

## Symptom: "Order has multiple items / sub-items"

```
1. Swap main item ↔ sub-item.
2. Set sub-item price to 0; main item price to 0; both prices to 0.
3. Add an extra sub-item to a single-item order.
4. Send the same item twice with conflicting prices.
5. Reference an item from a different merchant / store.
```

## Symptom: "I only have read-only access — can't actually create test orders"

```
1. Use the cancel/error flow — it exposes callback URL pattern + accepted parameter set without committing.
2. Browse JS bundles for fetch() / XHR / GraphQL endpoints not used by the UI yet.
3. Use Wayback Machine + gh-recon for historical request samples from the same merchant.
4. Look at OTHER merchants on the same gateway — the integration pattern is shared.
5. Read the gateway's public docs end-to-end. Often the docs leak example payloads with real-looking tokens.
6. If absolutely nothing else: file a bug bounty submission for "lack of test environment" and request sandbox access.
```

## Symptom: "Gateway has no sandbox / test mode"

```
1. Use the gateway's cancel flow on real merchants for free signature validation (Phase 2).
2. Find a low-priced product on the target merchant ($0.01 / 1 IDR if any) and pay once for callback capture.
3. Test on a sister merchant of the same gateway that DOES have sandbox access — the signature scheme is gateway-wide, not merchant-specific.
4. If the gateway has a "demo" or "showcase" merchant on its own site, use that.
5. Request sandbox access via the gateway's dev-portal signup. Many give credentials to anyone with an email.
```

## Symptom: "Signature is generated client-side in JS for every request"

```
HIGH OPPORTUNITY — client-side signing means the secret is in the browser.
1. Inspect the JS bundle: search for "hmac", "sha256", "md5", "secret", "key", "sign(".
2. The "secret" found in JS is one of:
   a. The actual signing key (the bug — entire scheme is broken; sign anything)
   b. A public/derived key (verify by signing a known payload and comparing to a real request)
   c. A token negotiated per-session (intercept the negotiation; can you reuse it?)
3. If (a): write a tiny script that signs your mutated request with the captured secret. Now Phase 2 is trivially passable.
4. If (b)/(c): the real check happens server-side; treat it as normal Phase 2.
5. Look for race conditions in the negotiate-secret endpoint — can you reuse one user's secret for another user's order?
```

## Symptom: "I'm running out of ideas; nothing works"

```
1. Recheck Phase 1: did you find ALL endpoints? Particularly:
   - "Save for later" / draft order endpoints
   - Quote / estimate endpoints
   - Refund / partial-refund endpoints
   - Subscription / recurring-billing endpoints
2. Check side flows: gift cards, store credit, loyalty points, wallet top-up
3. Check the user account flow: email change, currency preference, country swap
4. Check business-logic edge cases:
   - What happens if you cancel during the 3DS challenge?
   - What happens if you close the browser tab mid-payment?
   - What happens if you pay twice for the same order?
   - What happens if a refund is processed during a new payment?
5. Time-box: if 8 hours have passed with no progress, the integration is probably correct. Move to a new target.
```

## Anti-Patterns

Things that look like bugs but usually aren't:

- "The signature changes when I change a param!" — that's the system working correctly. The bug is when it doesn't change.
- "I can read the secret in the JS bundle!" — usually that's a public key or a sandbox secret. Verify before exploiting.
- "The amount in the URL doesn't match the cart!" — sometimes this is intentional (tax, fees, currency conversion). Check the final invoice first.
- "Burp shows me the raw signature!" — yes, the signature is supposed to be visible. The question is whether you can forge or skip it.
