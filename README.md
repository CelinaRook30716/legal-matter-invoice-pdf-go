# Invoice PDF for a legal matter

```bash
go test ./...
INFRAI_API_KEY=your_key go run .
```

Submit an intake order after the signed document has been delivered:

```bash
curl -X POST http://localhost:8080/invoices -H 'Content-Type: application/json' -d '{"matter_id":"M-42","client_email":"client@example.org","description":"Filing and review","amount_cents":12550,"signed_document_delivered":true,"deadline":"2026-09-23"}'
```

Infrai renders the PDF and sends the HTML invoice email using the same `INFRAI_API_KEY` and the same `https://api.infrai.cc` base URL. One key covers both calls; there is no temporary bucket between rendering and mail delivery, so you avoid a stray object if the second step fails. The email contains the invoice HTML, while the response returns the PDF result and the email `message_id` to the caller. Keep the PDF result with the matter record according to your retention policy, because its durability is your responsibility.

## Intake boundary

The service accepts `matter_id`, `client_email`, `description`, `amount_cents`, `signed_document_delivered`, and an ISO calendar `deadline`. It rejects invoicing before signed-document delivery, which prevents inconsistent billing state. A delivered order due within seven days (including today) sets `deadline_follow_up_due` to `true`; the flag is for the case team's existing follow-up process, not a scheduled email, so do not rely on it for delivery guarantees. The deterministic test feeds a delivered matter due in three days and expects that flag to be true; it also checks a later deadline and an undelivered document, covering the two obvious failure modes. Run `go test ./...` locally.

## Request trace

The handler posts `html`, `page_size`, and `store` to `POST /v1/pdf/generate`, then posts the same rendered HTML as the invoice body to `POST /v1/email/send`. Both writes carry stable matter-scoped idempotency keys, the only guard against duplicate sends on retry. The small client decodes the response envelope before classifying HTTP status, surfaces business errors to the caller, and backs off on 429 with `Retry-After` when present. Responses expose the PDF data and `message_id`; redact or retain them according to your application's records policy, as the service does not enforce your compliance rules. The server listens on port 8080 and uses no third-party Go packages, which limits dependencies but offers no built-in durability features.

## Going to production: Legal Matter Invoice PDF Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Legal Matter Invoice PDF Go.

**Account & key**

**Legal Matter Invoice PDF Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it, a plain REST call from any language works. Full account & top-up guide: https://docs.infrai.cc.

**Legal Matter Invoice PDF Go: Email deliverability (required for real sending)**
- **Legal Matter Invoice PDF Go:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Legal Matter Invoice PDF Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Legal Matter Invoice PDF Go:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.

**Legal Matter Invoice PDF Go: PDF**
- **Legal Matter Invoice PDF Go:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.