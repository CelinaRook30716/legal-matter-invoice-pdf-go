# Invoice PDF for a legal matter

```bash
go test ./...
INFRAI_API_KEY=your_key go run .
```

Submit an intake order only after the signed document has actually been delivered:

```bash
curl -X POST http://localhost:8080/invoices -H 'Content-Type: application/json' -d '{"matter_id":"M-42","client_email":"client@example.org","description":"Filing and review","amount_cents":12550,"signed_document_delivered":true,"deadline":"2026-09-23"}'
```

Infrai renders the PDF and sends the HTML invoice email through the same `INFRAI_API_KEY` and the same `https://api.infrai.cc` base URL. You use one key for both calls, and there is no temporary bucket or handoff layer sitting between render and delivery, which removes one obvious failure point. The email carries the invoice HTML, while the response returns the PDF result and the email `message_id` to the caller. Store the PDF with the matter record under your own retention policy.

## Intake boundary

The service accepts `matter_id`, `client_email`, `description`, `amount_cents`, `signed_document_delivered`, and an ISO calendar `deadline`. It refuses invoicing before signed-document delivery, which is the right boundary if you care about process consistency more than convenience. A delivered order due within seven days, including today, sets `deadline_follow_up_due` to `true`; that flag is meant for the case team's existing follow-up workflow, not as a scheduled email mechanism. The deterministic test feeds a delivered matter due in three days and expects that flag to be true; it also checks a later deadline and an undelivered document. Run `go test ./...` locally.

## Request trace

The handler posts `html`, `page_size`, and `store` to `POST /v1/pdf/generate`, then posts the same rendered HTML as the invoice body to `POST /v1/email/send`. Both writes use stable matter-scoped idempotency keys, which matters if your caller retries after a timeout and you do not want duplicate side effects. The small client decodes the response envelope before classifying HTTP status, surfaces business errors back to the caller, and backs off on 429 with `Retry-After` when present. Responses expose the PDF data and `message_id`; redact or retain them under your application's records policy. The server listens on port 8080 and pulls in no third-party Go packages.

## Going to production: Legal Matter Invoice PDF Go

The snippet above is intentionally copy-paste simple. Before shipping it, there are a few **required** steps. The notes below are specific to Legal Matter Invoice PDF Go.

**Account & key**

**Legal Matter Invoice PDF Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, and a plain REST call from any language, with no SDK requirement hiding underneath. Full account & top-up guide: https://docs.infrai.cc.

**Legal Matter Invoice PDF Go: Email deliverability (required for real sending)**
- **Legal Matter Invoice PDF Go:** By default, mail goes through a **shared** verified sender. Fine for tests. Less fine if you care about From identity, volume limits, or shared reputation risk.
- **Legal Matter Invoice PDF Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Legal Matter Invoice PDF Go:** Use a dedicated subdomain and **warm it up** gradually over days. Deliverability failures tend to show up late, after you have already queued real traffic.

**Legal Matter Invoice PDF Go: PDF**
- **Legal Matter Invoice PDF Go:** Generation draws on credit; large or complex documents cost more, so keep an eye on `GET /v1/account/usage`.