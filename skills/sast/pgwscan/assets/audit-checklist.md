# Payment Gateway Audit Checklist

Engagement: ___________  Date: ___________  Tester: ___________

## Phase 1 — Recon and Flow Mapping

- [ ] Gateway provider identified: ____________________
- [ ] Public docs URL: ________________________________
- [ ] Sandbox / test credentials available: [ ] yes [ ] no
- [ ] gh-recon dorks executed for SDK and signing code: [ ]
- [ ] Full flow captured (cart → success): [ ]
- [ ] Number of signed steps: ___
- [ ] Number of unsigned steps: ___
- [ ] Cancel flow tested: [ ]
- [ ] Callback URL pattern captured (without paying): [ ]
- [ ] Critical-fields catalogue completed: [ ]

## Phase 2 — Signature Reverse-Engineering

- [ ] Signature hash function identified (MD5 / SHA1 / SHA256 / HMAC-X): ____
- [ ] Param order rule identified (alphabetical / declaration / unknown): ____
- [ ] Delimiter identified: ____
- [ ] Param name inclusion (values-only / name=value): ____
- [ ] Secret placement (prefix / suffix / middle): ____
- [ ] Shift test attempted: [ ]
- [ ] Shift test passed (hash unchanged after rebalance): [ ]
- [ ] Coverage sweep — params NOT in hash:
  - [ ] description / desc / transaction_info: ____________
  - [ ] return_url / cancel_url / notify_url: ____________
  - [ ] buyer_info / merchant_email: ____________
  - [ ] tax / discount / fee_*: ____________
  - [ ] Other: ____________

## Phase 3 — Order / Price Tampering

- [ ] Direct numeric mutation (1, 0, negative, scientific): [ ] result: ______
- [ ] Currency swap (lowest-value): [ ] result: ______
- [ ] Quantity manipulation: [ ] result: ______
- [ ] Total override (when separate): [ ] result: ______
- [ ] Sub-item / line-item swap: [ ] result: ______
- [ ] Coupon / voucher abuse: [ ] result: ______
- [ ] Hidden fields injected: [ ] result: ______
- [ ] Type juggling: [ ] result: ______
- [ ] Last-mile mutation on gateway URL: [ ] result: ______
- [ ] Race conditions: [ ] result: ______

## Phase 4 — Callback / Notify / Return Abuse

- [ ] Callback URL captured: [ ]
- [ ] Strip-and-replay (no signature): [ ] result: ______
- [ ] Status flip (-1 → success): [ ] result: ______
- [ ] Amount lowered with original signature: [ ] result: ______
- [ ] Cross-order replay (orderId swap): [ ] result: ______
- [ ] Double-credit (same callback twice): [ ] result: ______
- [ ] Direct-to-merchant callback (no gateway): [ ] result: ______
- [ ] Idempotency race (50 parallel): [ ] result: ______

## Phase 5 — Combined Exploit

- [ ] Working PoC chains: [ ] (describe below)
- [ ] Reproduction steps documented: [ ]
- [ ] Original + mutated request bodies saved: [ ]
- [ ] Response evidence captured (screenshots / Burp logs): [ ]
- [ ] Report drafted from `assets/report-template.md`: [ ]

## Findings Summary

| # | Phase | Severity | Title | Status |
|---|-------|----------|-------|--------|
| 1 |       |          |       |        |
| 2 |       |          |       |        |
| 3 |       |          |       |        |

## Open Questions

-
-
-

## Time Spent

| Phase | Hours |
|-------|-------|
| 1     |       |
| 2     |       |
| 3     |       |
| 4     |       |
| 5     |       |
| Total |       |
