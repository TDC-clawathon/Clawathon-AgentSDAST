---
name: pgwscan
mode: pgwscan
kind: option
description: Payment-gateway–specific security checks (optional add-on to quickscan/deepscan). Audits merchant ↔ payment-gateway integration code for payment-bypass weakness classes — signature/HMAC coverage gaps, callback/IPN trust, server-side price/amount recompute, idempotency, replay/relay protection, and pricing/business-logic flaws.
deliverables: contributes payment-gateway findings to sast/report.md
references: 01-recon-and-flow-mapping, 02-hmac-bypass-methodology, 03-order-tampering-checklist, 04-callback-abuse-patterns, 05-replay-and-relay-attacks, 06-blackbox-decision-tree, 07-business-logic-attack-surface, 08-modern-payment-flows, 09-interactive-testing-protocol, 99-real-case-walkthrough
---

# Payment Gateway Security Checks

## AgentSAST integration (whitebox, source-code mode)
This is an **optional add-on** layered on quickscan/deepscan. When enabled, also
audit any payment-gateway **integration code** found under `./raw` and add the
results to `sast/report.md` (the base scan still owns `openapi.yaml`/`base_url.txt`).
Use `search_code` to locate payment/checkout/callback handlers (e.g. `amount`,
`price`, `signature`, `hmac`, `checksum`, `secureCode`, `ipn`, `notify_url`,
`callback`, `webhook`), then `read_file` them and look for these weakness classes:
- **Signature/HMAC coverage gaps** — params (amount/currency/orderId/status) not
  included in the signed payload; name-boundary ambiguity (see the *shift test*).
- **Callback/IPN trust** — crediting an order from callback data without
  re-querying the gateway; missing/replayable signature; status flips.
- **Server-side recompute** — trusting a client-supplied `price`/`amount`/`total`
  instead of recomputing from authoritative data; currency/quantity tampering.
- **Idempotency / replay** — same callback processed twice (double-credit);
  cross-amount relay of a low-value confirmation onto a high-value order.

The methodology below was written for human-driven **blackbox** pentests; in this
automated source-audit context, treat the references as a catalog of *what to
look for in the code* (pull them via `get_knowledge`). Ignore steps about
companion skills, browser/Burp/interactive handoffs, and engagement intake.

---

A pentester-focused methodology for finding payment-bypass vulnerabilities in payment gateway integrations. Distilled from real-world bug bounty findings.

## Scope

This skill covers blackbox auditing of:
- Merchant ↔ Payment Gateway integrations (server-to-server signing, redirect flows, callbacks)
- HMAC / MD5 / SHA signature schemes used to protect payment parameters
- Order creation and tampering (price, amount, quantity, currency, coupons, sub-items)
- Callback / notify / return URL abuse (replay, relay, status flip)
- Multi-gateway and multi-step checkout flows

This skill does **NOT** cover: PCI-DSS compliance auditing, card-data extraction, infrastructure attacks against the gateway itself, or whitebox source-code review (use a different methodology when source is available).

## When to Activate

Activate this skill when the user provides any of:
- A captured HTTP request to a checkout / payment / order endpoint
- A redirect URL to a third-party payment gateway
- A callback / notify / return URL containing `secureCode` / `signature` / `hash` / `checksum` / `hmac` parameters
- A request to "find payment bypass on site X"
- A description of an `amount`, `price`, `total`, or `currency` parameter in a payment-related request
- Any mention of HMAC / MD5 / SHA in an order-creation or payment-confirmation context

Do NOT activate for: general billing-system bugs unrelated to a payment gateway, or whitebox source-code review.

## Boundary with sibling skills

Route to a different skill when intent does not match blackbox audit:

| User intent | Use skill | Why |
|-------------|-----------|-----|
| "Build / integrate Stripe / Polar / SePay / VietQR" | `payment-integration` | This skill is audit-only, does not implement |
| "Review my payment-handler source code for bugs" | `code-review` | Whitebox source review |
| "STRIDE / OWASP audit of my checkout backend" | `ck-security` | Whitebox security audit |
| "Hunt for leaked merchant SDK on GitHub" (standalone) | `gh-recon` | Pure recon task; use this skill if blackbox audit is the larger goal |
| "OSINT on a payment company" (no checkout in scope) | `cti-expert` | OSINT-only |
| "PCI-DSS compliance audit" | none of these | Out of scope; refer user to a compliance auditor |

If the user request straddles two skills (e.g., "audit my own integration"), confirm with the user whether they want **blackbox** (this skill — no code access, simulate attacker) or **whitebox** (`code-review` / `ck-security` — read source). Do not assume.

## Five-Phase Methodology

Work through these phases in order. Skipping recon (Phase 1) is the most common mistake — it is the single biggest accelerator on real engagements.

### Phase 1 — Recon and Flow Mapping

Goal: understand the integration before touching any payload.

1. **Identify the payment gateway provider.** Look at the redirect domain, logos on the bank/wallet selector page, JS files served by the checkout page, cookie names, and `Server` header.
2. **Search for public integration documentation.** Roughly 90% of gateways publish merchant SDKs and integration guides. Find them via Google (`"site:gateway.com" integrate OR sdk OR signature`), the gateway's developer portal, and GitHub. **Use the `gh-recon` skill to dork GitHub** for leaked merchant SDK code, sample signing functions, and other integrators' source.
3. **Map the flow end-to-end.** Capture every request from "Add to cart" through to "Payment success" page. Annotate each step with: URL, method, params, response code, redirect target, and whether a signature is present.
4. **Use the cancel / error path to observe callbacks without paying.** Click "Cancel" on the gateway page — the callback / return URL is usually disclosed in the redirect or in a JS variable. Record the callback URL pattern.
5. **Identify critical fields** in each request: `amount`, `price`, `currency`, `quantity`, `orderId`, `merchantId`, `status`, `signature` / `secureCode` / `checksum` / `hmac`.

Output of Phase 1: a flow diagram (mental or written) with all endpoints, params, and signatures labelled. See `references/01-recon-and-flow-mapping.md`.

### Phase 2 — Signature / HMAC Reverse-Engineering

Goal: figure out the exact formula that produces the signature, OR find params not covered by it.

Pursue 5 hypotheses in parallel:
1. **Param order**: alphabetical, request order, or fixed declaration order?
2. **Delimiter**: `|`, `:`, `&`, `,`, space, or no separator?
3. **Param name inclusion**: `value` only, or `name=value` joined?
4. **Coverage**: which params are NOT in the hash? (the highest-value question)
5. **Secret placement**: prefix, suffix, or middle?

**The shift test** — the single most useful blackbox technique: take two adjacent params and move one character from the first into the second, keeping the signature unchanged. If the response still succeeds, the server reconstructs the hash by joining values without param-name boundaries — meaning you can rebalance characters between params while keeping the hash valid. This is how price tampering on signed requests becomes possible.

Always verify your hypothesis on a low-value sandbox transaction first if available; otherwise use the cancel flow to test signature acceptance without committing money.

See `references/02-hmac-bypass-methodology.md` for the full hypothesis-test matrix and `references/99-real-case-walkthrough.md` for an end-to-end worked example.

### Phase 3 — Order and Price Tampering

Goal: alter monetary values such that the server still accepts the order.

Test these vectors one at a time (single-field mutation; combine only after individual results are understood):
1. Direct `price` / `amount` / `total` decrease (including `0`, `0.01`, negative numbers, very large numbers, scientific notation `1e2`, `1e-10`)
2. `currency` swap (USD → VND, EUR → IDR — find the lowest-value currency the server accepts)
3. `quantity` to negative, zero, fractional, or extreme values
4. Sub-item ↔ main-item swap (move an expensive item into the sub-item array, a cheap item into the main slot)
5. Coupon / voucher value injection or replay
6. Hidden / undocumented fields discovered in JSON responses or JS bundles (e.g., `internal_price`, `override_amount`)
7. Add a second sub-item array entry the UI does not normally allow
8. Race conditions on coupon redemption (apply the same coupon in parallel before order finalizes)
9. Type juggling (`"100"` vs `100` vs `[100]` vs `{"$ne": null}`)
10. Last-mile mutation (change the amount on the gateway page after the merchant signed it)

For each mutation, watch: server validation message, redirect target, the `amount` displayed on the gateway page, and the final invoice. The "right" amount sometimes only diverges at one of those three stages.

See `references/03-order-tampering-checklist.md`.

### Phase 4 — Callback / Notify / Return URL Abuse

Goal: tell the merchant a payment succeeded when it did not, or for a different amount.

1. Capture the callback URL via the cancel flow or by intercepting the gateway → merchant call.
2. **Strip the signature parameter and replay**. If accepted, the merchant trusts callbacks unconditionally — full bypass.
3. **Flip the status field** (`-1` → `0`, `failed` → `success`, `pending` → `paid`).
4. **Lower the amount** while keeping the signature and observe whether the merchant credits based on the URL value or re-queries the gateway.
5. **Replay a successful callback from a small transaction** with the order ID swapped to a higher-value pending transaction (cross-amount relay).
6. **Reuse the same callback** twice for double-credit (idempotency check).
7. **Trigger callback directly without going through the gateway** if the merchant endpoint is internet-reachable.

See `references/04-callback-abuse-patterns.md` and `references/05-replay-and-relay-attacks.md`.

### Phase 5 — Combined Exploitation and Reporting

Goal: chain primitives into a full PoC and write it up.

1. Combine Phase 2 (HMAC bypass via shift) with Phase 3 (price reduction) to bypass signed-request protection.
2. Combine Phase 4 (callback replay) with Phase 3 (order swap) to credit a high-value order using a 1¢ payment.
3. Document the PoC using `assets/report-template.md`: include the original request, the mutated request, the server response that confirms acceptance, and a clean reproduction sequence.

## Workflow

**Default mode: automate as much as possible.** Claude does the research, hash computation, payload generation, doc reading, `gh-recon` searches, and payload mutation by itself. Only escalate to interactive (user-in-the-loop) handoffs when a step genuinely requires the human — see `references/09-interactive-testing-protocol.md` for the handoff format.

For every engagement, walk through these steps:

1. **Recon (automated).** If the user named the target, run `gh-recon`, fetch public docs, identify the gateway provider, parse captured requests provided by the user. Only ask questions if you literally cannot proceed (e.g., no captured request was provided and no public endpoint to probe).
2. **Phase 2-4 analysis (automated).** Compute hashes locally to verify candidate signature formulas. Generate mutated payloads from captured requests. Read references/03-08 to map symptoms to attack vectors. Build the next test as a ready-to-fire payload, not a question.
3. **Hand off ONLY when the next step requires the human** — see "When to escalate to interactive" below. When you do hand off, follow the format in `references/09-interactive-testing-protocol.md`.
4. **Track state** for long sessions (`assets/audit-state-template.md`) and report findings (`assets/report-template.md`) at the end.

### When to escalate to interactive (the only times)

Hand off to the user when, and only when:
- **Browser-driven step required** — cancel flow on the gateway page, 3DS challenge, Apple Pay biometric, OAuth login, mobile-app deep link, JS-heavy SPA that requires a real browser session
- **Burp intercept-and-drop required** — capturing requests mid-flow, dropping a request to observe partial state, modifying request in-flight
- **Production state change with real consequences** — replaying a payment-altering request that creates a real order, charges real money, sends real refund. Even on a sandbox where this is "free", the user owns the call.
- **Out-of-band channel needed** — OTP, SMS, email confirmation, push notification from a banking app
- **Information genuinely missing** — captures the user has but didn't paste, sandbox credentials only the user knows, scope details

Outside these cases, **proceed automatically**. Do not ask the user to do something Claude can do faster (running `gh-recon`, computing MD5, fetching public docs, building a mutated request payload, drafting the PoC report).

## Resource Map

- `references/01-recon-and-flow-mapping.md` — flow mapping, provider identification, doc search patterns
- `references/02-hmac-bypass-methodology.md` — full hypothesis-test matrix for signature reversing
- `references/03-order-tampering-checklist.md` — 10+ price/order mutation vectors
- `references/04-callback-abuse-patterns.md` — callback / notify / return URL attacks
- `references/05-replay-and-relay-attacks.md` — cross-amount replay, double-credit
- `references/06-blackbox-decision-tree.md` — which technique to try given a specific symptom
- `references/07-business-logic-attack-surface.md` — refund, subscription, marketplace, gift card, wallet attacks
- `references/08-modern-payment-flows.md` — 3DS, Apple Pay, Google Pay, JWT, in-app receipts, mobile deep links
- `references/09-interactive-testing-protocol.md` — handoff patterns (intake, capture, mutation, sweep, pause, async, resume)
- `references/99-real-case-walkthrough.md` — anonymized end-to-end PoC against gateway-X
- `assets/intake-form.md` — engagement intake form (send to user at start)
- `assets/audit-state-template.md` — state snapshot for cross-session resume
- `assets/audit-checklist.md` — engagement checklist template
- `assets/report-template.md` — PoC writeup template

## Companion Skills

- `gh-recon` — search GitHub for leaked merchant SDK code, signing helpers, integration samples
- `agent-browser` — drive the checkout flow programmatically when manual reproduction is repetitive

## Key Principles

- **Recon docs first, blackbox second.** Public integration documentation gives you the signature formula in minutes; blackbox guessing can take hours.
- **Single-field mutations before combos.** Never change two things at once until each is understood individually.
- **The cancel flow is free reconnaissance.** It exposes the callback URL and the full set of server-recognized fields without spending money.
- **Hypothesis-driven testing.** Form a specific hypothesis ("I think `desc` is not in the hash"), design one mutation that proves or disproves it, run it, log the result.
- **Find the param the hash forgot.** Most real bypasses come from one parameter the developer left out of the signature, not from cryptographic weakness.
- **Manual over automated.** Payment flows are stateful, idempotent in unpredictable ways, and easy to break beyond repair. Hand the mutated request to the user to replay; do not script the exploit yourself.
