# Email delivery & digests

WorldSignal can deliver matched Signals by **email** — either one message per Signal
(*instant*) or a batched **digest** (hourly or daily). Email is a first-class
[delivery channel](API.md) alongside webhooks and polling.

Two pieces make it work:

1. A **connector** — a saved, encrypted SMTP configuration (Gmail, Outlook, Zoho,
   SendGrid, or any custom server). You set this up **once**, in the console.
2. An **email subscription** — a delivery route whose channel is `EMAIL`, with a
   recipient list, an optional connector, and a delivery mode (instant / digest).

> The connector holds the *credentials*; the subscription holds *who gets what*.
> You must create a connector **before** an email subscription can send.

---

## 1. Create a connector

Open **Connectors** in the sidebar (requires the `settings:manage` permission —
ADMIN role). Click **Add connector**, pick your provider, and the host, port and
security mode are pre-filled. Fill in the *From address* and credentials, then
**Add & verify** — WorldSignal opens a real connection and authenticates before
saving, so you get immediate feedback.

Secrets are encrypted at rest (AES-256-GCM, using `WEBHOOK_SIGNING_SECRET`) and are
never returned by the API — only the last four characters are shown.

Set one connector **active**: email subscriptions that don't name a specific
connector use the active one.

### Provider setup

<a id="gmail"></a>
#### Gmail

Gmail does **not** accept your normal password over SMTP — you need an **App
Password** (which requires 2-Step Verification):

1. Turn on 2-Step Verification: <https://myaccount.google.com/signinoptions/two-step-verification>
2. Create an App Password: <https://myaccount.google.com/apppasswords>
3. In the connector form:
   - **Provider:** Gmail (host `smtp.gmail.com`, port `587`, STARTTLS — pre-filled)
   - **Username:** your full Gmail address
   - **Password / API key:** the 16-character App Password (spaces optional)
   - **From address:** the same Gmail address

Google Workspace accounts work the same way (an admin may need to allow app
passwords for the org).

<a id="outlook"></a>
#### Outlook / Microsoft 365

- Host `smtp.office365.com`, port `587`, STARTTLS (pre-filled).
- Username/From: your full address. Use an app password if MFA is enabled.
- SMTP AUTH must be enabled for the mailbox in the Microsoft 365 admin center
  (it is disabled by default on some tenants).

<a id="zoho"></a>
#### Zoho Mail

- Host `smtp.zoho.com` (use `smtp.zoho.in` for `zoho.in` accounts), port `587`, STARTTLS.
- Create an **app-specific password**: Zoho Account → Security → App Passwords.

<a id="sendgrid"></a>
#### SendGrid

- Host `smtp.sendgrid.net`, port `587`, STARTTLS.
- **Username is the literal string `apikey`.**
- **Password** is a SendGrid API key with the *Mail Send* permission.
- Verify your sender or domain in SendGrid first, or messages will be rejected.

<a id="custom"></a>
#### Custom SMTP

Choose **Custom SMTP** and enter your provider's host, port and security mode:

| Port | Security     | Notes                                  |
|------|--------------|----------------------------------------|
| 587  | STARTTLS     | Most common submission port.           |
| 465  | SSL/TLS      | Implicit TLS.                          |
| 25   | None/STARTTLS| Server-to-server; often blocked by ISPs. |

Use **None** only against a local test server such as
[MailHog](https://github.com/mailhog/MailHog) — never over the public internet.

### Test it

- **Test** opens a connection and authenticates (no email sent).
- **Send test** delivers a sample message to an address you choose — the
  end-to-end check.

---

## 2. Create an email subscription

Open **Subscriptions → Add subscription**:

- **Channel:** `EMAIL`
- **Recipients:** one or more comma-separated addresses.
- **Connector:** *Active connector (default)* or a specific one.
- **Delivery:**
  - **Instant** — one email per matched Signal, sent as it's produced.
  - **Hourly digest** — a single rollup email per hour.
  - **Daily digest** — a single rollup email per day.
- **Filter (JSON):** which Signals match — same shape as any subscription, e.g.
  ```json
  { "tags": ["DISASTER"], "minSeverity": "HIGH", "countries": ["US", "IN"] }
  ```

Digests only send when at least one Signal matched during the interval; an empty
interval produces no email. The first digest fires one interval after the first
matching Signal is queued, then on the cadence thereafter.

### Config reference

The subscription's `config` for the `EMAIL` channel:

| Field         | Type              | Description                                             |
|---------------|-------------------|---------------------------------------------------------|
| `to`          | string or string[]| Recipients (comma/semicolon separated string, or array).|
| `connectorId` | string (optional) | A specific connector id; omit to use the active one.    |
| `mode`        | `instant`\|`digest`| Delivery mode (default `instant`).                     |
| `interval`    | `hourly`\|`daily` | Digest cadence when `mode` is `digest` (default `daily`).|

Example (daily digest through the active connector):

```json
{ "to": "alerts@team.com, ops@team.com", "mode": "digest", "interval": "daily" }
```

---

## Reliability

Email sends run through the same Postgres-backed delivery queue as webhooks, so
they inherit **retries with exponential backoff** and **dead-lettering** after the
retry limit. A failed send is recorded on the delivery with its error message —
inspect it under **Deliveries**. Digest emails appear there too, as a single
delivery covering many Signals.

## Configuration

| Variable              | Default | Description                                                        |
|-----------------------|---------|--------------------------------------------------------------------|
| `APP_BASE_URL`        | _(empty)_ | Public console URL. When set, email titles link to the Signal.   |
| `DIGEST_TICK_SECONDS` | `60`    | How often the digest scheduler checks for due digests.            |

SMTP credentials are **not** environment variables — connectors are managed in the
console and encrypted at rest. See [CONFIGURATION.md](CONFIGURATION.md) for the
full environment reference.

## Troubleshooting

- **"no email connector configured"** — add a connector and set one active, or set
  `connectorId` on the subscription.
- **Gmail `535 5.7.8 Username and Password not accepted`** — you're using your
  login password; create an **App Password** instead (see above).
- **Outlook `5.7.57` / auth errors** — SMTP AUTH is disabled for the mailbox;
  enable it in the Microsoft 365 admin center.
- **Digest never arrives** — confirm the subscription is enabled, its filter
  matches recent Signals, a worker/`ROLE=all` instance is running (the digest
  scheduler runs with the workers), and check **Deliveries** for a failed row.
