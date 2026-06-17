# Interactive Testing Protocol

Goal: standardize handoff format for the **few** steps in a payment audit that genuinely require the human pentester. **This protocol is NOT the default workflow.**

## When to use this protocol (and when NOT to)

**Default:** Claude automates whenever possible — `gh-recon`, doc reading, hash computation, payload mutation, sandbox/read-only request replay, report drafting. Do these silently and report results, not questions.

**Switch to interactive ONLY when the next step requires the human:**

| Trigger | Pattern to use |
|---------|----------------|
| First contact and user gave no usable artifacts (no request, no URL, no target) | Pattern A — Initial Intake |
| Browser-driven recon needed (cancel flow, JS app load, mobile deep link, login wall) | Pattern B — Capture Hand-off |
| Mutation would charge real money or create real production order | Pattern C — Single-Mutation Hand-off |
| Multi-field coverage scan that needs replay across many fields | Pattern D — Batched Sweep |
| Genuinely ambiguous next step where wrong guess wastes time | Pattern E — Pause-for-Clarification |
| Waiting for an external event (callback, OTP, settlement) | Pattern F — Asynchronous Wait |
| User returns after a gap and state context was lost | Pattern G — Resume-from-Session |

**Do NOT use this protocol when** Claude can complete the step alone:
- Reading public docs / `gh-recon` searches → just do it, report findings
- Computing the MD5/HMAC of a candidate concatenation to verify formula → compute it, report
- Generating a mutated payload from a template → generate it inline, then hand to user only if firing it has real consequences
- Drafting a PoC report from existing evidence → draft it, ask user to review

The whole point of automation is to eliminate ping-pong. Use these patterns when ping-pong is unavoidable, not by default.

---

## The 7 Interaction Patterns

### Pattern A — Initial Intake (start of engagement)

When the user first invokes the skill AND has not yet provided enough context to start automated analysis, Claude requests a structured set of artifacts. Use `assets/intake-form.md` as the template.

**When to use:** ONLY when the user's prompt lacks both a captured request AND a target URL Claude can probe. If the user already pasted a captured request or named a target, skip intake and start automating (Phase 1 recon, gh-recon, doc fetch). Run intake mid-flow only if a missing artifact is now blocking progress.

**Template Claude sends:**

```
Before I can audit, I need the following. Reply with what you have; mark MISSING for the rest.

REQUIRED:
1. Captured CREATE-ORDER request (Burp "Copy as curl" preferred — full headers + body)
2. Redirect URL the merchant returns after create-order (the URL pointing to the gateway)

STRONGLY RECOMMENDED:
3. Captured CALLBACK URL via cancel-flow (cancel through the gateway → paste the URL the browser lands on)
4. Gateway provider name (if known)
5. Sandbox / test credentials (if available)

OPTIONAL:
6. Any leaked SDK / sample code from gh-recon
7. Reference docs or PDF of the gateway's integration guide
8. Specific vulnerability classes you suspect

CONSTRAINTS to know:
- Test data only? Production allowed? Sandbox only?
- Tooling on hand (Burp, Caido, mitmproxy, Playwright)
```

**Wait state:** Claude does not begin Phase 2/3/4 until at least items 1-2 are provided OR the user explicitly says "proceed without".

---

### Pattern B — Capture Hand-off (Phase 1 recon)

When Claude needs the user to perform browser-driven recon and bring back captures.

**When to use:** Phase 1 cancel-flow recon, Phase 1 flow mapping, capturing JS bundles, recording mobile deep link.

**Template:**

```
HANDOFF — Browser capture required.

DO (in this order):
1. Open Burp Suite (or Caido / mitmproxy). Configure browser to proxy through it.
2. Add product to cart on <target>.
3. Proceed to checkout. Click "Pay with [gateway]".
4. STOP at the gateway page (do not enter card data).
5. Click "Cancel" on the gateway page.
6. Note the URL the browser redirects to.

CAPTURE and paste back to me:
- Burp HTTP history from step 2 onwards (or "Copy all to file")
- The cancel-redirect URL
- Screenshot of the gateway page (specifically: which payment methods are listed)

Estimated time: 5-10 minutes. Reply when ready or if you hit a blocker.
```

---

### Pattern C — Single-Mutation Hand-off (Phase 2-4 iteration)

The core loop. Claude proposes one mutated request → user replays → user reports → Claude diagnoses.

**When to use:** every Phase 2 hypothesis test, every Phase 3 single-field mutation, every Phase 4 callback test.

**Template:**

```
TEST <N> — <hypothesis being tested>.

ORIGINAL REQUEST (recap):
<paste a 1-line summary, e.g. "GET /checkout.php?...&price=100000&...&secure_code=abc">

MUTATION:
Field: <field name>
From: <original value>
To:   <mutated value>

EXPECTED IF VULNERABLE: <e.g., gateway accepts request, shows price=1 on selection page>
EXPECTED IF NOT VULNERABLE: <e.g., HTTP 400 "invalid signature">

REPLAY:
Open Burp Repeater, paste the request above, change ONLY the marked field, send.

REPORT BACK:
- HTTP status code
- First ~30 lines of response body (especially error messages)
- If 3xx: the Location header
- If gateway page renders: what amount/currency it shows
- If it took > 5 seconds: note that
```

**Anti-pattern:** Claude must NOT propose multiple mutations in the same TEST. One mutation per loop iteration unless explicitly batching (Pattern D).

---

### Pattern D — Batched Sweep (parameter coverage scan)

When Claude needs to test the same mutation pattern across many fields (e.g., the Phase 2 H4 coverage sweep).

**When to use:** identifying which params are NOT in the hash; testing a list of currency codes; testing every parameter for type-juggling.

**Template:**

```
SWEEP — <what's being scanned>.

For each field below, replay the original request changing ONLY that field. Report results in a table.

FIELDS TO SWEEP:
- transaction_info: change last 2 chars
- order_code: change last char
- buyer_info: change "John" → "Jane"
- order_description: change "x" → "y"
- affiliate_code: append "Z"
[etc.]

REPORT TABLE:
| Field            | Mutation              | HTTP status | Accepted? | Notes |
|------------------|-----------------------|-------------|-----------|-------|
| transaction_info | last 2 chars changed  |             |           |       |
| ...              |                       |             |           |       |

When complete, paste the table back. I'll diagnose which fields are unhashed.
```

---

### Pattern E — Pause-for-Clarification

When Claude has multiple plausible hypotheses and asking is cheaper than guessing.

**When to use:** ambiguous gateway response, missing context that changes the methodology branch, scope question.

**Template:**

```
PAUSE — Need clarification before continuing.

Observation: <what was just observed>
This could be either of:
A) <hypothesis A — implication: do X next>
B) <hypothesis B — implication: do Y next>

To distinguish, I need to know: <specific question>

Options for you to answer:
- [A_indicator] → tells me it's hypothesis A
- [B_indicator] → tells me it's hypothesis B
- "I don't know" → I'll propose a discriminating test
```

**Anti-pattern:** asking too often (creates friction). Use only when the next step diverges substantially based on the answer; otherwise proceed with the most likely hypothesis and note the assumption.

---

### Pattern F — Asynchronous Wait (callback / race / out-of-band)

When the test requires waiting for an external event.

**When to use:** waiting for gateway → merchant callback to arrive, scheduling parallel race tests, waiting for a sandbox to settle a transaction.

**Template:**

```
ASYNC WAIT — <what we're waiting for>.

DO:
1. Set up Burp Collaborator (or webhook.site, or local listener on port X).
2. Replay request <id> with notify_url=<your collaborator URL>.
3. Wait for the gateway to call your collaborator (typically 5 seconds to 5 minutes).

WHILE WAITING:
- You can come back any time. I'll be here.
- If nothing arrives within 10 minutes, the gateway probably didn't fire (config issue) — report and we'll diagnose.

WHEN IT ARRIVES:
- Paste the full HTTP request the gateway sent (headers + body)
- Note the time elapsed
- If multiple callbacks arrived, paste all of them in order
```

---

### Pattern G — Resume-from-Session

When the user comes back after a gap (next day, after a break) and Claude needs to re-establish state without re-doing recon.

**When to use:** start of a new session on the same target; user mentions "continuing from yesterday".

**Template Claude sends:**

```
RESUME — Recap before continuing.

To pick up where we left off, paste the last `audit-state.md` snapshot OR answer:

1. Target name
2. Phase status (which phases done, which in-progress)
3. Last hypothesis tested + result
4. Open hypotheses still to test
5. Any new captures since last session

If this is a fresh session, just say "fresh" and I'll re-run intake (Pattern A).
```

The `assets/audit-state-template.md` is a one-page snapshot the user can paste back to fast-forward.

---

## State Tracking

For long engagements, maintain a lightweight state file:
- Use `assets/audit-state-template.md` as a starting point
- Update at the end of each session (Claude proposes a delta, user saves)
- Format: short markdown — phase status, hypothesis log, captured artifacts list, next planned tests

Lightweight, not full project management. The goal is enough context to resume without redoing Phase 1.

---

## Format Conventions

When sending a HANDOFF / TEST / SWEEP:

- Always start with the pattern name in uppercase (HANDOFF, TEST, SWEEP, PAUSE, ASYNC WAIT, RESUME)
- Sequential test numbering (TEST 1, TEST 2, ...) per session
- Include "REPORT BACK:" section listing exactly what to paste
- Estimate time for handoffs > 5 minutes
- Always allow "I'm blocked" as a valid response — and have a diagnostic ready

When receiving evidence from the user:

- Acknowledge what was received before analyzing
- Echo the key observation back (e.g., "Status 200, response says 'invalid signature' — confirms field is hashed")
- Update state mentally; if state grew complex, propose `audit-state.md` snapshot
- Then propose next TEST

---

## Anti-Patterns to Avoid

| Anti-pattern | Why it's bad | Do instead |
|--------------|--------------|------------|
| Auto-execute the mutated request via curl | Triggers real backend state changes | Hand the curl command to the user |
| Propose 5 mutations in one TEST | User loses track of which one caused which response | One mutation per TEST iteration |
| "Try various values for price and tell me what works" | Vague handoff, user can't act on it | Specific values: "Try 1, 0, -1, 1e2, 0.001 in this order" |
| Generate a working full exploit script proactively | Same risks as auto-execute | Provide individual mutated requests; user assembles their own PoC |
| Assume state from prior turn without re-asking after long gap | Stale assumptions corrupt analysis | Pattern G resume |

---

## When to use this protocol

Pull this in when:
- The user invokes the skill for the first time on a new target (Pattern A)
- Any Phase 2/3/4 iteration is starting (Pattern C / D)
- The user pastes raw evidence and Claude needs to interpret + propose next step (Pattern C loop)
- The session is resumed after a break (Pattern G)
- Claude is about to propose a mutation that, if wrong, requires the user to redo the capture (Pattern E pause first)
