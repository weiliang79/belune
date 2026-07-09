# SMTP Configuration Runbook

How to configure outbound email for password-reset links, user invitations, and
alert notifications. All three features share a single SMTP transport — if
SMTP is not configured they silently degrade to log-only mode (useful for local
development but not for production).

---

## 1. Required env vars

Set these in `/opt/belune/.env` before (re)starting the stack.

```env
# Public URL of the dashboard — used to build reset / invite links.
# Must be the URL users actually open in a browser (with scheme, no trailing slash).
PUBLIC_BASE_URL=https://belune.example.com

# SMTP server
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=apikey                      # or full email address, depends on provider
SMTP_PASSWORD=<secret>
SMTP_FROM_EMAIL=noreply@example.com
SMTP_FROM_NAME=Belune
SMTP_TLS_MODE=starttls                # starttls | tls | none
```

After editing `.env`, restart the API:

```sh
docker compose -f /opt/belune/docker-compose.yml restart belune
```

---

## 2. TLS modes

| `SMTP_TLS_MODE` | Port | When to use |
|---|---|---|
| `starttls` | 587 | Default. Most hosted providers (Gmail relay, Mailgun, SES). |
| `tls` | 465 | Implicit TLS. Some older servers and self-hosted Postfix setups. |
| `none` | any | Internal relays on a trusted LAN. Never use over the public internet. |

---

## 3. Provider quick-start

### Mailgun (recommended for simple setups)

```env
SMTP_HOST=smtp.mailgun.org
SMTP_PORT=587
SMTP_USER=postmaster@mg.example.com
SMTP_PASSWORD=<mailgun-smtp-password>
SMTP_TLS_MODE=starttls
```

### Amazon SES

```env
SMTP_HOST=email-smtp.us-east-1.amazonaws.com
SMTP_PORT=587
SMTP_USER=<SES-SMTP-user>
SMTP_PASSWORD=<SES-SMTP-password>
SMTP_TLS_MODE=starttls
```

Ensure the `SMTP_FROM_EMAIL` address is verified in SES, and that the account
is out of sandbox mode if you're emailing non-verified recipients.

### Gmail relay (G Suite / Workspace)

```env
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=you@yourdomain.com
SMTP_PASSWORD=<app-password>          # Google account → Security → App passwords
SMTP_TLS_MODE=starttls
```

Personal `@gmail.com` accounts can use this for low-volume testing, but Google
rate-limits them aggressively. Use an app-specific password, not your account
password.

### Self-hosted Postfix (internal relay)

```env
SMTP_HOST=postfix.internal
SMTP_PORT=25
SMTP_TLS_MODE=none
# leave SMTP_USER and SMTP_PASSWORD empty
```

---

## 4. Log-only fallback (development)

When `SMTP_HOST` is empty, the email service logs rendered message subjects and
recipients at INFO level instead of dialling. This is the default behaviour and
lets the rest of the platform start without a real mail server.

Confirm it is active:

```sh
docker compose -f /opt/belune/docker-compose.yml logs belune | grep "smtp"
# Expected: level=INFO msg="SMTP host not configured, falling back to log-only mode"
```

---

## 5. Smoke-test a live connection

Trigger a test email by requesting a password reset for the admin account from
the login page (`/forgot-password`). Then check:

```sh
# Confirm the email was dispatched (SMTP path)
docker compose -f /opt/belune/docker-compose.yml logs belune | grep "email sent"

# Or, in log-only mode, inspect the rendered message
docker compose -f /opt/belune/docker-compose.yml logs belune | grep "subject="
```

---

## 6. Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Reset link never arrives | Wrong `SMTP_HOST` / credentials | Check logs for `smtp dial` errors |
| "TLS handshake failed" | Port/TLS-mode mismatch | Match port to TLS mode (587+starttls, 465+tls) |
| "535 Authentication failed" | Wrong credentials | Regenerate password/API key; SES users: use SMTP credentials, not API keys |
| Link in email points to wrong URL | `PUBLIC_BASE_URL` not set or wrong | Set to the exact URL users open, no trailing slash |
| SES bounces with "Email address not verified" | SES sandbox or unverified sender | Verify the `SMTP_FROM_EMAIL` address in the SES console |
