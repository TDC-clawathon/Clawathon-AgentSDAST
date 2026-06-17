# Modern Payment Flows

Goal: cover attack surface in non-classical payment flows — 3DS challenges, mobile wallets (Apple Pay / Google Pay), JWT-signed confirmations, in-app purchases (IAP), mobile deep links, and webhook timing.

For each: explain the flow, the bypass primitive, and what to test in blackbox.

---

## 1. 3-D Secure (3DS / 3DS2)

3DS adds a buyer-bank challenge step between the merchant and the gateway. Bypasses come from making the merchant THINK 3DS succeeded when it didn't.

### How the flow works

```
Merchant → Gateway → 3DS-server (issuer bank)
                ↓
  Buyer challenged (OTP, biometric, app push, or "frictionless" auto-pass)
                ↓
        ECI / CAVV / XID / 3DS-status returned to gateway
                ↓
        Gateway → Merchant: "3DS authenticated, eci=05"
```

The merchant sees a result code. If the merchant trusts the result without re-querying, you can forge it.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| T1 | Forge ECI value in callback | Send `eci=05` (full auth) when actual was `eci=07` (no auth) |
| T2 | Frictionless force | Set `challenge_indicator=01` (no challenge) — some merchants downgrade silently |
| T3 | Missing 3DS = success | Strip 3DS-related fields from callback — merchant defaults to "no 3DS needed" |
| T4 | 3DS abandonment = success | Cancel during the challenge — some merchants treat the inflight as success |
| T5 | Cross-card 3DS replay | Use 3DS auth result from card A in transaction with card B |
| T6 | 3DS for soft-decline only | Issuer soft-declines without 3DS — merchant retries with `force_3ds=false` and goes through |
| T7 | Bypass via card type | 3DS not enforced for "low-risk" categories (e.g., recurring, MIT) — flag your card-on-file as MIT |

### What to test

1. Capture the 3DS callback / status field. Note: who provides it (gateway / 3DS server / merchant)?
2. Test T3 first — strip 3DS fields entirely.
3. Test T1 by mutating ECI from `07` (failed) to `05` (full auth).
4. Look for `merchant_initiated`, `recurring`, `mit` flags — these often skip 3DS.

### Stop when
- Merchant cryptographically verifies CAVV/AAV from issuer
- Merchant rejects callbacks missing required 3DS fields
- 3DS status is server-fetched from gateway, not callback-trusted
- MIT/recurring flags require prior cardholder auth on file

---

## 2. Apple Pay / Google Pay (Tokenized Payments)

Mobile wallets send a payment token (Apple Pay token / Google Pay tap-to-pay cryptogram) instead of raw card. The token is single-use and bound to merchant_id.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| AP1 | Token replay | Reuse a captured Apple Pay token for a second transaction (token nonce should prevent — verify) |
| AP2 | Cross-merchant token | Use token bound to merchant A on merchant B (token has `merchant_id` field — does B verify?) |
| AP3 | Strip token, send PAN | Frontend says "use Apple Pay", attacker submits raw PAN of an unrelated card via API |
| AP4 | Modify amount with valid token | Token signed for `$10`, mutate request to `$1` — does merchant trust request amount over token amount? |
| AP5 | Token reuse after refund | Refund the token-payment, then reuse the (now-cancelled) token for a new purchase |
| AP6 | Wallet provisioning bypass | Provision a card you don't own to your wallet via leaked DPAN / one-time-code |
| AP7 | DPAN ↔ FPAN confusion | Substitute device PAN (DPAN) with funding PAN (FPAN) in subsequent calls |

### What to test

1. Capture the token from the Apple Pay / Google Pay payment request.
2. Test AP1: resubmit same token after a few seconds.
3. Test AP4: capture token, intercept request, mutate `amount` while keeping token unchanged.
4. Test AP3: complete a Apple Pay flow once, then bypass UI and POST raw card body to the same endpoint.

### Stop when
- Token is single-use server-side
- Token's `merchant_id` matched against acquiring merchant_id
- Amount in token cryptogram matched against request amount
- DPAN never substituted with FPAN

---

## 3. JWT-Signed Payment Confirmations

Modern APIs (Stripe-style webhooks v2, Adyen, BNPL providers) use JWT for callback signing. Bugs in JWT validation are well-known but appear surprisingly often in payment context.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| J1 | `alg: none` | Set header `{"alg":"none"}`, drop signature — accepted by buggy verifiers |
| J2 | HS256/RS256 confusion | Sign with HMAC using the public key as the secret (server expected RS256) |
| J3 | KID path traversal | Set `kid: ../../../etc/passwd` or `kid: /dev/null` — server uses KID to load signing key |
| J4 | JWK injection | Embed `jwk` header with attacker-controlled public key — server uses embedded key |
| J5 | Algorithm downgrade | Change RS512 → RS256 → HS256 → none, see what server accepts |
| J6 | Expired-token replay | `exp` not validated — replay a year-old token |
| J7 | Audience mismatch | Token from gateway A used against merchant B's webhook endpoint |
| J8 | Mutate payload, recompute signature with leaked secret | Webhook signing secret leaked in dev docs / GitHub / .env |

### What to test

1. Capture a JWT-signed callback. Decode header (jwt.io or `cut + base64 -d`).
2. Test J1, J2 first — these are the highest-success classes.
3. For J8: gh-recon the merchant for `whsec_`, `webhook_secret`, `STRIPE_WEBHOOK_SECRET` etc.
4. Test J6 by capturing today's webhook and replaying it next week.

### Stop when
- Algorithm is whitelisted (only RS256 OR HS256, not "any")
- KID is validated against a known-key list, not used as a path
- `exp`, `iat`, `aud`, `iss` all validated
- Webhook secret rotated regularly and not in plaintext storage

---

## 4. In-App Purchase (IAP) Receipts (Apple / Google / Steam)

IAP receipts are signed by the platform (Apple App Store, Google Play, Steam). Server validates by sending the receipt back to platform's verification endpoint.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| I1 | Sandbox receipt against production | Apple sandbox receipts validate-OK on Apple sandbox endpoint; merchant sends to wrong endpoint and accepts |
| I2 | Receipt replay (cross-user) | User A's valid receipt replayed for User B's account — receipt doesn't bind to user |
| I3 | Receipt replay (same user) | Same receipt unlocks the same product multiple times |
| I4 | Refunded receipt still credits | Apple revokes IAP, merchant doesn't poll for revocations |
| I5 | Local receipt forgery | iOS receipts are local-file before validation — replace with one from another product |
| I6 | Signature stripped | Some merchants validate via "signed_data + signature" — strip sig, accept |
| I7 | Subscription extension via receipt-replay | Auto-renewable subscription receipt replayed → fake renewal |

### What to test

1. Identify IAP validation endpoint (`POST /iap/validate`, `/in-app-purchase/verify`).
2. Test I1: send sandbox-tier receipt against production endpoint.
3. Test I2: capture your receipt, send with another user's session.
4. Test I3: capture, replay 50 times.

### Stop when
- Receipt validated against the correct platform endpoint per environment
- Receipt bound to user_id at first validation; subsequent validations require match
- Idempotency key = (transaction_id from receipt, user_id) atomic
- Subscription validity rechecked per request, not at validation-time only

---

## 5. Mobile Deep Link / App-to-App Handoff

Vietnamese mobile wallets (MoMo, ZaloPay) and many international ones use deep links: merchant app → wallet app → back to merchant app via custom URL scheme.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| D1 | Deep-link hijack | Register attacker's app for the merchant's URL scheme — receives the success callback |
| D2 | Return URL mutation | Inject attacker-controlled return URL during create-payment — attacker app gets the success token |
| D3 | Deep-link replay | Capture the deep link with success token, fire it from a different device/session |
| D4 | Universal link without app | If merchant app not installed, browser fallback may not enforce the same checks |
| D5 | Token in URL leaked via referrer / browser history | Token included in URL gets leaked to next page's Referer header |
| D6 | Race during handoff | Deep link fires twice (clipboard + auto-launch) → two callbacks |

### What to test

1. Capture the deep link URL (USB debugging on Android, idb / Charles on iOS).
2. Test D1 by registering a stub app with the same scheme.
3. Test D2 by setting a custom `redirect_url` / `return_url` to an attacker domain.
4. Test D5 by inspecting `document.referrer` after the redirect.

### Stop when
- Universal Links / App Links (verified domain ownership) used instead of custom schemes
- Token is short-lived and one-time
- Token bound to device / session
- Return URL validated against allowlist

---

## 6. Webhook Timing & Signing Quirks

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| WS1 | Timing attack on signature compare | Server uses non-constant-time `==` for HMAC compare → leak via response timing |
| WS2 | Replay outside window | `Stripe-Signature` includes `t=<timestamp>`; server doesn't enforce window → replay anytime |
| WS3 | Webhook ID reuse | Stripe `idempotency-key`; merchant doesn't dedupe → process twice |
| WS4 | Different signed payload, identical body | Whitespace / unicode normalization differences let two different signed bodies be treated as one |
| WS5 | gzip / chunked transfer split | Body parsed twice with different boundaries |
| WS6 | Replay with body modified post-sign | Some libraries verify HMAC against raw body but parse a different body — body-vs-signed-body mismatch |

### What to test

1. Capture webhook including signature header.
2. Test WS2 by replaying the exact webhook a day later.
3. Test WS6 by mutating the body but keeping the signature — compare error messages between "invalid signature" and "valid signature, processing" status.

### Stop when
- HMAC compare is constant-time (response time invariant)
- Timestamp window enforced (e.g., 5 minutes)
- Webhook idempotency keyed on event_id, atomic
- Verified body == parsed body (no transformation between)

---

## How to use this reference

Pull this in when:
- Phase 1 recon shows: 3DS challenge step, mobile-wallet payment, JWT-formatted callbacks, IAP / mobile-app-store flows, deep-link return paths
- Webhook signature uses Stripe / Adyen / Razorpay style (HMAC + timestamp)
- The target is a SaaS/subscription/digital-good business (high IAP and 3DS prevalence)

For most modern providers (Stripe, Adyen, BNPL providers like Klarna/Afterpay), bugs cluster in JWT validation (Section 3) and webhook timing (Section 6). For mobile-first Vietnamese targets, deep-link hijacking (Section 5) is the single highest-value class.
