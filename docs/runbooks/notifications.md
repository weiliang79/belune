# Notification Channels Runbook

Notification **channels** route platform events out to external services —
Discord, Telegram, Slack, a generic webhook, ntfy, Gotify, or email — so an
operator hears about a failed deploy or a broken certificate without watching
the dashboard.

Channels are an **instance-wide, admin-only** concern (unlike per-user
[alert emails](./alerts.md), which each user opts into for themselves). Manage
them from **Notifications** in the sidebar.

> **Channels vs. the in-app bell.** The bell and per-user alert emails are
> unchanged. Channels are an additional delivery layer fed by the *same* events —
> enabling a channel never suppresses the bell, and disabling one never affects
> it.

---

## 1. What gets delivered

A channel delivers only the events it is **subscribed** to. The available events
are served from the backend registry (so this list never drifts from the code):

| Group | Event | Severity |
|---|---|---|
| Deployments | Deployment succeeded | ok |
| Deployments | Deployment failed | error |
| Databases | Backup failed | error |
| Databases | Restore completed | ok |
| Databases | Restore failed | error |
| TLS | Certificate expiring | warning |
| TLS | Certificate expired | error |
| TLS | Certificate failed | error |
| Volumes | Volume backup failed | error |
| Volumes | Volume restored | ok |
| Volumes | Volume restore failed | error |

Severity drives per-provider styling (Discord embed colour, ntfy priority and
tags, emoji prefixes).

Delivery is **asynchronous and retried** (three attempts with backoff on a
low-priority queue). It never blocks or fails the work that produced the event —
a broken webhook cannot stall a deploy. The outcome of the last attempt is shown
on the channel row: a green *Sent …* timestamp, or a red **Failed** with the
provider's error on hover.

---

## 2. Adding a channel

1. Go to **Notifications → Add Channel**.
2. Give it a name, pick a **Type**, and fill the type-specific fields
   (see below).
3. Tick the **Events** it should receive.
4. **Create**, then use the row's **Send test** button to confirm delivery.

The provider configuration (webhook URLs, bot tokens, access tokens) is
**encrypted at rest** with the instance keyring and never returned to the
browser. Because reads are masked, editing a channel re-enters its configuration
— or leave the config fields **blank to keep** the stored one and change only
the name or event subscriptions.

The **type is fixed once saved** (the stored config is provider-specific). To
switch providers, create a new channel and delete the old one.

### Links

Event notifications deep-link back into the dashboard. Those links are absolute,
built from **`PUBLIC_BASE_URL`**. If that is unset, notifications are still
delivered but without a link.

---

## 3. Per-provider setup

### Discord
- **Webhook URL** — Discord: *Server Settings → Integrations → Webhooks → New
  Webhook → Copy Webhook URL.*

### Slack
- **Webhook URL** — create a Slack app, enable *Incoming Webhooks*, then *Add New
  Webhook to Workspace* and copy the URL.

### Telegram
- **Bot token** — create a bot with [@BotFather](https://t.me/BotFather).
- **Chat ID** — message your bot once, then read the chat id from
  [@userinfobot](https://t.me/userinfobot) or the `getUpdates` API. Use a
  negative id for a group/channel.

### Webhook (generic)
- **URL** — receives a JSON `POST`:
  `{type, title, body, link, severity, occurred_at}`.
- **Signing secret** (optional) — when set, each request carries an
  `X-Belune-Signature: sha256=<hmac>` header (HMAC-SHA256 of the raw body), the
  same convention as the git webhooks. Verify it to reject forged requests.

### ntfy
- **Topic** — required (e.g. `belune-alerts`).
- **Server URL** — leave blank for the public `https://ntfy.sh`, or point at a
  self-hosted instance (`https://ntfy.example.com`).
- **Access token** (optional) — only needed for protected topics; sent as a
  `Bearer` token.

Severity maps to ntfy priority/tags: errors → high priority + 🚨, warnings →
default + ⚠️, ok → low + ✅.

### Gotify
- **Server URL** — your Gotify instance (`https://gotify.example.com`).
- **App token** — Gotify: *Apps → Create Application → copy the token.* Sent as
  the `X-Gotify-Key` header.

### Email
- **Recipients** — one or more comma-separated addresses.
- By default the channel uses the instance SMTP (**Server → Email (SMTP)**). If
  that isn't configured, the channel's test and events fail with a clear error
  rather than silently going nowhere — set SMTP first.
- **Use a custom mail server** (optional) — toggle this to route *this channel*
  through its own SMTP server (host/port/user/password/from/encryption), e.g. to
  send alerts through a different provider than the app's transactional mail. Its
  password is keyring-encrypted like every other channel secret. Left off, the
  instance SMTP is used.

> **Overlap with per-user alert emails.** An **email channel** subscribed to
> *Deployment failed* delivers to its fixed recipient list *in addition to* the
> per-user [deploy-failure alert emails](./alerts.md) a project owner may already
> receive. That is two separate paths on purpose — just be aware the same
> failure can produce two emails if both are configured for the same person.

---

## 4. Enabling / disabling

The **Enabled** switch on each row takes effect immediately — a disabled channel
is skipped at delivery time, even for events already queued. Disable a noisy or
temporarily-broken channel rather than deleting it to keep its configuration.

---

## 5. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Row shows **Failed** | Hover the status for the provider's exact error. Common: revoked webhook, wrong bot token, unreachable self-hosted server. |
| Test send says *no mailer configured* | Email channel with no SMTP host — configure [`smtp.md`](./smtp.md). |
| Delivered but no link in the message | `PUBLIC_BASE_URL` is unset. |
| Nothing arrives, no error | The channel isn't subscribed to that event, or it's disabled. |
| Duplicate emails | An email channel and per-user alert emails both cover the same event (see §3). |
