# Audit State Snapshot

Lightweight state file for cross-session resume. Update at the end of each session; paste back at the start of the next.

## Engagement

- Target: ____________
- Gateway provider: ____________
- Started: YYYY-MM-DD
- Last updated: YYYY-MM-DD

## Phase Status

| Phase | Status | Notes |
|-------|--------|-------|
| 1 — Recon & flow mapping | not started / in progress / done | |
| 2 — Signature reverse | not started / in progress / done / N/A | |
| 3 — Order tampering | not started / in progress / done | |
| 4 — Callback abuse | not started / in progress / done | |
| 5 — Combined exploit + report | not started / in progress / done | |
| BL — Business logic (refund, sub, marketplace, gift, wallet) | not applicable / in progress / done | |
| MOD — Modern flows (3DS, Apple Pay, JWT, IAP, deep link) | not applicable / in progress / done | |

## Flow Map (Phase 1 output)

```
Step  Method  URL                                         Signature?  Critical params
1     ...     ...                                         ...         ...
```

## Signature Algorithm (Phase 2 output)

- Hash function: ____________ (e.g., MD5, HMAC-SHA256)
- Param order: ____________ (alphabetical / declaration / SDK-fixed)
- Delimiter: ____________ (`|`, `:`, ` `, `&`, none)
- Param name in hash: yes / no
- Secret placement: prefix / suffix / middle
- Params NOT in hash: ____________ (the high-value finding)

## Hypothesis Log

| # | Hypothesis | Test | Result | Decision |
|---|------------|------|--------|----------|
| H1 | <hypothesis> | <mutation> | accepted / rejected / inconclusive | <next step> |
| H2 | | | | |

## Captured Artifacts

- [ ] Create-order request (file/path or location)
- [ ] Gateway redirect URL
- [ ] Cancel-flow callback URL
- [ ] Successful callback
- [ ] JS bundle (if scraped)
- [ ] gh-recon notes
- [ ] Other: ____________

## Findings (running list)

| # | Phase | Severity | Title | Status |
|---|-------|----------|-------|--------|
| 1 | | | | confirmed / suspected / dismissed |

## Open Hypotheses (still to test)

- ___
- ___

## Next Planned Tests

- TEST <N>: ___
- TEST <N+1>: ___

## Blockers

- ___

## Time Spent

- Total so far: ___ hours
- Cap (if any): ___ hours
