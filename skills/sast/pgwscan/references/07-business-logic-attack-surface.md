# Business-Logic Attack Surface

Goal: cover money-flow bugs that live OUTSIDE the create-order → pay → callback path. These are typically business-logic bugs that a clean Phase 2/3/4 audit would miss.

Scope of this reference:
1. Refund / cancellation abuse
2. Subscription / recurring billing
3. Marketplace / split-payment / commission
4. Gift card / store credit / loyalty points
5. Wallet top-up

For each: list the attack patterns, the watch-points (response, state, balance), and the stop conditions.

---

## 1. Refund / Cancellation Abuse

Refunds reverse a payment but the goods/service often cannot be reversed. Bugs let the attacker keep both.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| R1 | Full refund + keep digital good | Refund processed but the download / license / credit is not revoked |
| R2 | Refund without the original payment | API accepts refund of an unrelated transactionId or a self-supplied amount |
| R3 | Refund > original amount | Backend doesn't cap refund_amount ≤ original payment |
| R4 | Multiple refunds for one payment | Idempotency by request-id but not by `(orderId, original_tx_id)` |
| R5 | Refund another user's payment | Refund endpoint authorizes the requester but not the payment ownership |
| R6 | Negative refund (=charge) | Submit `refund_amount=-100` → some backends process as a charge in the merchant's favour, OR vice-versa as a credit to attacker |
| R7 | Refund during pending state | Trigger refund while payment is in "settling" — both refund AND payment go through |
| R8 | Refund through wallet, charge through bank | Refund credited to attacker's wallet (instant) before bank chargeback decision |
| R9 | Partial refund manipulation | Multiple partial refunds totalling > original (one or two slip through atomicity gap) |

### What to test

1. Capture a normal refund request. Note: who can call it (admin only? customer self-service?), what fields are signed.
2. Test R1-R5 in priority order.
3. For R6 specifically: submit negative numbers AND fractional amounts (`-1`, `-0.01`, `0.001`).
4. For R9: fire 5 partial refunds in parallel summing to the original amount, then a 6th — does the 6th go through?

### Stop when
- Refund auth = ownership-bound to original payment
- Refund amount validated `0 < refund ≤ original.amount`
- Refund idempotency keyed on `(original_tx_id, refund_request_id)` and atomic
- Refund triggers a revoke-callback for digital goods

---

## 2. Subscription / Recurring Billing

Subscriptions have multiple state machines (trial → active → grace → cancelled → reactivated) — bugs live in the transitions.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| S1 | Trial extension via cancel-then-resubscribe | Cancel during trial → new account / new email → another trial |
| S2 | Downgrade keeping premium benefits | Downgrade to free plan but feature flags don't revoke immediately |
| S3 | Upgrade prorating exploitation | Upgrade Pro → Team mid-cycle, then immediately downgrade — refund > paid |
| S4 | Pause + use + resume without billing | Pause subscription, continue using, never resume — service still on |
| S5 | Card change during failed renewal | Card declined, swap to test card, retry — service kept active |
| S6 | Billing-cycle race | Cancel at renewal moment — service extends one cycle without payment |
| S7 | Free trial without card | Some flows let you start trial without card; never converts to paid |
| S8 | Coupon stacking on subscription | Apply launch-coupon + loyalty-coupon → 100%+ discount = free |
| S9 | Plan-id manipulation | POST `/subscribe` with plan_id="pro" but price_id="starter" — pay starter price for pro features |
| S10 | Subscription transfer | Transfer subscription to a different user account → both now have access |

### What to test

1. Map all state transitions: trial → active → grace → past_due → cancelled → reactivated → cancelled → ...
2. For each transition: what triggers it (time / event / API call)? Is the trigger user-controllable?
3. Test S1 by cycling cancel/resubscribe rapidly.
4. Test S2 by downgrading and immediately probing premium-only endpoints.
5. Test S6 by intercepting the renewal request and calling `/cancel` with surgical timing.

### Stop when
- Trial uses payment-method fingerprint, not email, for "previously trialed" check
- Plan downgrade revokes feature flags atomically with the billing change
- Coupons enforce stacking rules + max-discount cap server-side
- Plan_id and price_id are bound (cannot mix)
- Renewal triggers are server-clock, not user-input

---

## 3. Marketplace / Split-Payment / Commission

Marketplaces split a payment between merchant + platform fee. Bugs in the split = attacker keeps platform's cut OR redirects merchant's cut to attacker.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| M1 | Vendor share mutation | Buyer-side request includes `vendor_id` / `payee_id` — change to attacker's ID |
| M2 | Commission rate manipulation | `commission_pct` sent client-side — change to 0 |
| M3 | Split count manipulation | Split into 5 payees with attacker as one; original merchant gets nothing |
| M4 | Currency mismatch in split | One leg in USD, other in IDR — server uses one rate for total, another for splits |
| M5 | Refund routing | Buyer asks for refund → refund deducts from vendor's account, not platform's |
| M6 | Settlement timing race | Vendor asks for early payout while payment is being disputed — both succeed |
| M7 | Vendor onboarding bypass | Pay attacker as if onboarded vendor without going through KYC |
| M8 | Cross-vendor referral fee abuse | Refer-a-vendor bonus paid based on attacker-controlled metric |

### What to test

1. Capture a split-payment create request. Look for `payees[]`, `splits[]`, `transfers[]`, `recipients[]`.
2. Test M1 first — payee_id mutation is the most common bug.
3. Test M2/M3 by adding extra entries to the split array.
4. Test M5 by initiating refund and tracing which account is debited.

### Stop when
- payee_id derived server-side from product/order, not from request
- Commission is fixed per-merchant in DB, not in request
- Refund debits the platform, with a separate vendor-clawback flow
- Settlement holds during dispute

---

## 4. Gift Card / Store Credit / Loyalty Points

Internal currencies have weak controls compared to fiat — most bugs come from generation, redemption, or balance arithmetic.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| G1 | Gift card serial enumeration | Serial format predictable (sequential, dated, low-entropy) — brute-force find unredeemed cards |
| G2 | Redeem same gift card twice | Race condition between balance-check and balance-deduct |
| G3 | Cross-account transfer | Gift card transfer endpoint doesn't bind sender = current owner |
| G4 | Negative redemption | Redeem `-100` → balance increases |
| G5 | Apply gift card after order placed | Retroactively apply card to an already-paid order, refund the difference to bank |
| G6 | Loyalty point mint via cancel | Earn points on order, cancel order, points stay |
| G7 | Loyalty point arithmetic overflow | Earn so many points that the balance int32 overflows to negative |
| G8 | Combine gift card + refund | Pay with card, get refunded to gift card balance, redeem balance, refund again |
| G9 | Gift card on top of gift card | Buy gift card with gift card — fee waived loop |
| G10 | Activate gift card without purchase | Activation endpoint doesn't verify the card was paid for |

### What to test

1. Check serial format (entropy estimation: count chars × log2(charset)).
2. Test G2 with parallel requests.
3. Test G6 by completing-then-cancelling an order earning points.
4. Test G10 by guessing serials of cards purchased by others.

### Stop when
- Serial is high-entropy (≥ 80 bits), case-sensitive, with check-digit
- Redemption is atomic (balance check + deduct in single transaction)
- Earn / spend / refund flows have audit trail and reconciliation
- Activation requires the original purchase order_id

---

## 5. Wallet Top-Up

Wallet flows are like subscription + callback abuse combined — attacker tops up wallet from one bank account, then drains.

### Attack patterns

| # | Attack | Mechanism |
|---|--------|-----------|
| W1 | Top-up callback replay | Replay the success callback → wallet credited multiple times for one bank charge |
| W2 | Cross-wallet callback hijack | Callback says wallet_id=A but real payment was for wallet_id=B |
| W3 | Top-up reversal abuse | Bank chargeback succeeds but wallet not debited back |
| W4 | Currency mismatch | Top-up in IDR credited as USD |
| W5 | Negative top-up | Top-up `-100` → wallet decreases (bug for service) but if combined with refund, attacker gains |
| W6 | Top-up via gift card → withdraw to bank | Convert internal-only currency to fiat |
| W7 | Race between top-up and withdrawal | Top-up callback in flight, attempt to withdraw the not-yet-credited amount, then top-up succeeds → both credit and withdrawal happen |

### What to test

1. Map: top-up create → bank charge → callback → wallet credit → withdraw.
2. Apply Phase 4 callback patterns (strip-and-replay, status flip, cross-wallet relay) directly.
3. Test W3 by initiating top-up then immediately filing chargeback with bank.

### Stop when
- Callback idempotency keyed on `(bank_tx_id, wallet_id)` and atomic
- Chargebacks debit wallet automatically
- Currency is wallet-fixed, not request-controlled
- Internal currency cannot be converted to fiat without KYC

---

## How to use this reference

Pull this in when:
- The user mentions any of: refund, subscription, recurring, marketplace, split, vendor, gift card, store credit, loyalty, wallet, top-up
- Phase 1 recon discovered endpoints like `/refund`, `/subscriptions`, `/payouts`, `/giftcards`, `/wallet/topup`, `/payees`, `/transfers`
- The merchant is a marketplace, SaaS, or any platform with non-trivial money flow

Each section maps to a Phase 5 (combined exploit) — chain the business-logic bug with Phase 4 callback abuse for maximum impact.
