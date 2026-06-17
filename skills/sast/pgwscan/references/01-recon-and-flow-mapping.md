# Recon and Flow Mapping

Goal: build a complete picture of the merchant ↔ gateway integration before sending a single mutated request.

## Step 1 — Identify the Gateway Provider

Inspect every artifact for fingerprints:

| Artifact | What to look for |
|----------|------------------|
| Redirect URL host | `vnpayment.vn`, `momo.vn`, `pay.zalopay.vn`, `vpc.onepay.vn`, `checkout.gateway-X.vn`, `checkout.stripe.com`, `paypal.com/checkoutnow` |
| Selector page logos | Bank icons, wallet icons (Vietcombank, VietinBank, BIDV, MoMo, ZaloPay, ViettelPay) — combined logo set hints at the aggregator |
| Page JS bundles | Filenames like `vnpay.js`, `payment-sdk.min.js`, `checkout.js`; comments with vendor URLs |
| Cookie names | `JSESSIONID` (Java), `PHPSESSID` (PHP), proprietary `pgw_session` |
| `Server` and `X-Powered-By` headers | `nginx`, `Apache`, `Microsoft-IIS`, `Express` — narrows tech stack and likely SDK |
| URL path conventions | `/vpcpay`, `/vpc/payment`, `/checkout.php`, `/api/v1/charge`, `/payment-connect` |
| Param naming style | snake_case → PHP/Python; camelCase → Java/.NET; PascalCase → .NET classic |

Combine fingerprints — a Vietnamese checkout that redirects to `vpc.*` and uses snake_case is almost always an OnePay-derived integration.

## Step 2 — Search for Public Integration Docs

About 90% of payment gateways publish merchant integration docs. Spend 10 minutes here before any blackbox guessing.

Search patterns (Google):
```
site:gateway-host.com (integrate OR sdk OR signature OR checksum OR secure_code)
"gateway-name" merchant integration filetype:pdf
"gateway-name" "secure_code" OR "secureHash"
"gateway-name" sandbox credentials
```

GitHub dorks (use the `gh-recon` skill):
```
"gateway-name" extension:php   # find merchant SDKs
"secure_pass" "implode"        # find signing helpers
"gateway-name" "md5(" extension:js
"merchant_site_code" extension:py
org:gateway-name-official
```

Other sources:
- Developer portal (`developers.gateway.com`, `dev.gateway.vn`)
- Wayback Machine (older versions of public docs are often more verbose)
- Vendor-supplied PHP/Java/Node sample repos
- Stack Overflow questions tagged with the gateway name (often paste real signing code)

When the docs are found, the signature formula, param order, delimiter, and secret placement are usually given verbatim — Phase 2 collapses to confirming the docs match the live target.

## Step 3 — Map the Full Flow End-to-End

Use Burp Suite (or Caido / mitmproxy) to capture every request from cart → success page. Annotate each step:

```
Step  Method  URL                                        Signature?  Critical params
1     POST    /api/cart/add                              no          item_id, qty
2     POST    /api/checkout/create-order                 yes (sig)   total, currency, items[]
3     GET     /payment-connect/method?...&secureCode=…   yes         amount, merchantOrderId, secureCode
4     GET     gateway.com/checkout.php?merchant_site_…   yes         price, secure_code
5     POST    gateway.com/process-payment                gateway     card_no, expiry, cvv
6     GET     merchant.com/callback?orderId=…&status=…   yes (sig)   status, amount, signature
7     GET     merchant.com/order/success                 no          orderId
```

For each `signature`-protected step record:
- Which params are sent
- Which response fields confirm success
- Whether the redirect or the callback is the authoritative confirmation

## Step 4 — Use the Cancel / Error Path for Free Recon

The cancel flow is the cheapest way to expose the callback URL and the full set of server-recognized fields:

1. Click through to the gateway page (Step 3 above).
2. Click "Cancel" or "Hủy thanh toán".
3. The browser is redirected to `merchant.com/cancel?...` or `merchant.com/callback?status=-1&...`.
4. Capture the redirect URL — it usually contains the same parameter shape as a successful callback, just with a different `status`.
5. Look at the gateway page source before cancelling — the `cancel_url` and `notify_url` are often hidden form fields.

This gives you the callback URL pattern without spending any money. From there, Phase 4 attacks become possible.

## Step 5 — Catalogue Critical Fields

For every captured request, extract:

| Field role | Common names |
|------------|--------------|
| Amount | `amount`, `price`, `total`, `total_amount`, `payment_amount`, `transAmount` |
| Currency | `currency`, `currencyCode`, `curr`, `ccy` |
| Quantity | `quantity`, `qty`, `count` |
| Order ID | `orderId`, `order_code`, `merchantOrderId`, `transactionId`, `apptransid` |
| Merchant ID | `merchantId`, `merchant_site_code`, `appid`, `mid` |
| Status | `status`, `paymentStatus`, `responseCode`, `errorCode` |
| Signature | `signature`, `secureCode`, `secureHash`, `checksum`, `hmac`, `sig`, `mac` |
| URLs | `return_url`, `cancel_url`, `notify_url`, `callback_url`, `ipn_url` |
| Description (often unhashed!) | `desc`, `description`, `order_description`, `transaction_info`, `note`, `memo` |

The "description" family of fields is the highest-value attack surface in many gateways — it carries arbitrary user-controlled text and is frequently omitted from the signature.

## Output of Phase 1

A short text artifact (paste into your notes) covering:
1. Gateway provider name and version
2. Public docs URL (if any)
3. Annotated flow table (steps 1–N)
4. Callback URL pattern (from cancel flow)
5. List of critical fields with role mapping
6. Open hypotheses to test in Phase 2 (signature formula candidates, unhashed-field candidates)
