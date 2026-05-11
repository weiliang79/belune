# Alert Notifications Runbook

The platform sends email alerts for three events: deployment failures, build
failures, and resource quota threshold crossings. Alerts are delivered per-user
— each team member controls their own preferences from the **Account → Alert
Preferences** page.

---

## 1. Prerequisite: SMTP

Alerts are delivered via the platform's SMTP transport. If SMTP is not
configured, alert emails are silently discarded (the log-only fallback does
not queue them for later delivery).

Configure SMTP first: [`smtp.md`](./smtp.md).

---

## 2. User preferences

Each user manages their own alert subscriptions:

1. Log in and go to **Account** (top-left profile menu or sidebar).
2. Scroll to **Alert Preferences**.
3. Toggle the events you want:

| Toggle | When an email is sent |
|---|---|
| Deploy failures | A deployment transitions to `failed` state |
| Build failures | A build step exits non-zero |
| Quota threshold | Your resource usage crosses the configured percentage |

4. For the **Quota threshold** toggle, set the percentage (1–100) at which you
   want to be notified. Default is 80%.
5. Click **Save**.

Preferences are per-user and take effect immediately — no restart required.

---

## 3. Alert scope

All alerts (deploy failures, build failures, quota threshold) are sent to the
**project owner** — the user who owns the project that triggered the event.
Admin role does not automatically grant platform-wide alert coverage; admins
receive alerts only for projects they own.

There is no global on/off switch for alerts — individual users opt in or out.
If a user has no email address on file, alerts for that user are silently
skipped.

---

## 4. Quota threshold alerts

The quota worker evaluates resource usage periodically. An alert fires when:

- The user's CPU or memory usage across all running containers exceeds
  `quota_threshold_percent` of their quota limit.
- A cooldown period has elapsed since the last alert (to prevent flooding).

To check current quota usage: **Settings → Quotas** (admin) or look at the
resource bars on the project dashboard.

---

## 5. Troubleshooting

| Symptom | Check |
|---|---|
| No alert emails received | Verify SMTP is configured and working (`smtp.md` § 5) |
| Preferences not saving | Check browser console for API errors; confirm the API is reachable |
| Alert fires too frequently | Increase `quota_threshold_percent` in Account preferences |
| Admin not receiving alerts for all projects | Expected — alerts go to the project owner only, not all admins |
