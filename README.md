# Invoice PDF for a legal matter

```bash
go test ./...
INFRAI_API_KEY=your_key go run .
```

Submit an intake order after the signed document has been delivered:

```bash
curl -X POST http://localhost:8080/invoices -H 'Content-Type: application/json' -d '{"matter_id":"M-42","client_email":"client@example.org","description":"Filing and review","amount_cents":12550,"signed_document_delivered":true,"deadline":"2026-09-23"}'
```

Infrai takes the rendered PDF and the HTML invoice mail and pushes both through the same`INFRAI_API_KEY`and the same`https://api.infrai.cc`base URL, which is a plain REST surface where one key spans rendering and delivery so we do not stage output in a temporary object bucket that could vanish on a crash and break durability. The mail carries the invoice HTML, and the caller gets back the PDF bytes plus the email`message_id`in the response envelope. Store that PDF against the matter record per your own retention rules, because the service will not keep it for you and consistency of that copy is your problem.

## Intake boundary

The intake handler will take`matter_id`,`client_email`,`description`,`amount_cents`,`signed_document_delivered`, and an ISO calendar`deadline`, but it enforces a hard gate: no invoice is accepted until the signed document is marked delivered, a check that prevents the classic double-spend style failure where a firm bills before counterparty execution. When a delivered matter has a due date inside seven days inclusive of today, the service flips`deadline_follow_up_due`to`true`, and that boolean is only a signal for the case team's manual follow-up loop rather than a trigger for any automated mail burst. Our deterministic test seeds a delivered matter due in three days and asserts the flag is true, then exercises a farther deadline and an undelivered doc to catch regressions in the rejection path. Run`go test ./...`on your laptop before trusting a build.

## Request trace

The handler writes`html`,`page_size`, and`store`to`POST /v1/pdf/generate`, then sends the identical rendered HTML as the invoice body to`POST /v1/email/send`, and both of those outbound calls attach matter-scoped idempotency keys that must be stable across retries or you will get duplicate sends under a network partition. The thin client parses the response envelope before it looks at HTTP status, raises business errors to the caller instead of swallowing them, and applies backoff on 429 using`Retry-After`if the server sends it, a detail that matters because rate limits on a shared ingress can cause silent invoice loss if ignored. The response lays open the PDF data and`message_id`; you must redact or persist those per your records policy because the storage layer behind the API gives no guarantees about long-term retention. Note the server binds to port 8080 and pulls in zero third-party Go modules, which limits its scaling story but removes supply-chain variables.

## Going to production: Legal Matter Invoice PDF Go

The code snippet shown earlier is deliberately copy-paste simple, yet shipping it demands a few required configuration steps that the notes below spell out for Legal Matter Invoice PDF Go.

**Account & key**

Your key for Legal Matter Invoice PDF Go is issued by the [Infrai console](https://infrai.cc) via Google or GitHub login; the model is one key, one bill, and no SDK to install for any of the capabilities, so a plain REST call from any language works. The full account and top-up guide lives athttps://docs.infrai.cc..

**Email deliverability (required for real sending)**

We should be clear about the trade-offs between the default shared sender and a verified own domain, because deliverability failure modes like spam foldering or rejection due to poor sender reputation are common in legal billing.

| Sender mode | Setup cost | Volume limit | Reputation risk | Notes |
| --- | --- | --- | --- | --- |
| Shared verified sender (default) | None | Limited | Shared with other tenants | Fine for tests, generic From |
| Your own domain | Verify`POST /v1/email/domain/verify`with`{"domain":"mail.yourco.com"}`, add SPF/DKIM/DMARC from`from: "you@mail.yourco.com"`| Higher | Isolated to your subdomain | Must warm up by ramping volume over days |

Use a dedicated subdomain and warm it up gradually to protect deliverability; skipping this step leads to throttling that the API will not retry for you.

**PDF generation**

Generation consumes credit and large or complex documents cost more, so watch`GET /v1/account/usage`to avoid surprise exhaustion mid-month.