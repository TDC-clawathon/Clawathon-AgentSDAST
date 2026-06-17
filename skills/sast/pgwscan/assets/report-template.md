# [Severity] [Title — e.g. "Price Tampering via HMAC Shift on merchant-X.com Checkout"]

## Summary

One paragraph: what is the bug, what is the impact, who is affected.

## Severity

- CVSS: ___ (vector: ___)
- Business impact: financial loss / data exposure / service abuse / ___
- Exploitability: trivial / moderate / hard
- Affected users: all customers / paying customers / specific cohort

## Affected Component

- URL(s): _________________
- Endpoint(s): _________________
- Gateway provider: _________________
- Authentication required: yes / no
- Account type: regular customer / merchant / staff

## Reproduction Steps

1. ...
2. ...
3. ...

### Original Request

```http
[paste exact request — method, URL, headers, body]
```

### Original Response

```http
[paste relevant response]
```

### Mutated Request

```http
[paste mutated request that demonstrates the bug]
```

### Mutated Response

```http
[paste response confirming the bug]
```

## Evidence

- [ ] Burp Repeater session saved
- [ ] Screenshot of payment selection page with mutated amount
- [ ] Screenshot of merchant order confirmation showing credit
- [ ] Network log of callback receipt

[embed screenshots here]

## Root Cause

Brief technical explanation. Examples:
- The `desc` parameter is not included in the merchant's signature, allowing arbitrary content to flow into the gateway's `transaction_info`, which IS in the gateway signature, enabling the shift attack.
- The merchant's callback handler does not verify the `signature` parameter, allowing strip-and-replay.
- The merchant's callback handler trusts the `amount` field from the URL instead of re-querying the gateway by `transactionId`.

## Impact

What an attacker can achieve:
- Pay $1, get credited as if $1000 was paid
- Mark any pending order as paid without payment
- Double-spend the same successful payment across many orders
- Etc.

Financial estimate: ___

## Recommended Fix

- Include all user-influenceable fields in the merchant's signature (specifically: ___)
- On callback, always re-query the gateway using `transactionId` for canonical state
- Use HMAC-SHA256 with `name=value` joining and proper escape on the delimiter
- Bind callback idempotency key to `(orderId, transactionId)` and enforce atomically
- (etc.)

## Timeline

- Discovery date: ___
- Vendor notification: ___
- Fix deployed: ___
- Disclosure: ___

## References

- Related CVE / CWE: _________
- Gateway documentation: _________
- Similar bugs in other integrations: _________
