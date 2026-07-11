# TLS & Certificates Runbook

How HTTPS works in Belune, what each status on a domain means, and what to do
when a certificate does not appear.

---

## The three SSL modes

Set per-domain, under an application's **Domains** tab.

| Mode | What happens | Use when |
|---|---|---|
| **Automatic** | Caddy obtains a free certificate from Let's Encrypt over ACME and renews it for you. Nothing to upload. | The normal case: your domain points at this server and ports 80/443 are open. |
| **Custom** | Belune serves a certificate you uploaded under **Settings → Certificates**. | You are behind Cloudflare in Full (strict) mode, or your organisation issues its own certificates. |
| **Off** | Plain HTTP only. No certificate is requested for the hostname. | Internal-only hostnames, or a domain terminated somewhere else. |

> **DNS Challenge** was withdrawn. It requires a Caddy build carrying DNS
> provider modules, which the stock image does not have, so a domain set to it
> could only ever sit on "pending" forever. The API rejects it. If you need a
> wildcard certificate, issue it elsewhere and upload it as a **Custom**
> certificate.

---

## What Automatic issuance needs

Let's Encrypt validates that you control the domain by connecting to it **from
the public internet**. All three must be true, or issuance cannot succeed:

1. **The DNS record points at this server.** An `A` record (or `AAAA` for IPv6)
   for the exact hostname, resolving to this server's public IP.
2. **Port 80 is reachable from the internet.** This is the HTTP-01 challenge, and
   it is not optional — port 80 must be open even though your site runs on 443.
3. **Port 443 is reachable from the internet**, for the certificate to be served
   once issued.

Set `BELUNE_PUBLIC_IP` in your `.env` to this server's public address. Belune
then checks each domain's DNS against it and tells you *before* Let's Encrypt
does when a record points somewhere else. (Without it, Belune tries to detect the
address itself and skips the check when it can only find a private one — behind
NAT, for example.)

---

## Reading the TLS status

Every domain shows a TLS badge; click it for the issuer, expiry, last check, and
any error. **Settings → Certificates** lists every domain's status in one table,
which is the fastest way to find the one that is stuck.

The status is what Belune *observed on the wire* — it dials the proxy every
minute and inspects the certificate actually being served — not what the
configuration says should be happening.

| Status | Meaning | What to do |
|---|---|---|
| **Active** | A valid certificate for this hostname is being served. | Nothing. |
| **Pending** | No certificate yet. Normal for the first minute or two after adding a domain. | Wait. If it stays pending, work through "Automatic issuance needs" above. |
| **Failed** | Issuance failed, and Belune knows why — the reason is in the badge. | See the failures below. |
| **Expiring** | The certificate expires in under 14 days and has not renewed. | For Automatic, this means renewal has been failing for weeks; treat it as Failed. For Custom, upload a replacement. |
| **Expired** | The certificate is past its expiry. HTTPS is broken for this domain now. | Same as Expiring, urgently. |
| **Off** | `ssl_mode=off`. No certificate is wanted. | Nothing. |

Admins are notified when a domain first enters Failed, Expiring, or Expired.

**Recheck now** in the badge re-probes immediately, rather than waiting for the
next sweep — use it after fixing DNS or a firewall.

---

## Common failures

The reason shown in the badge comes from Caddy's own logs, so it is the ACME
server's actual complaint. The usual ones:

### "…resolves to X, not this server"

The hostname points somewhere else. Fix the DNS `A` record, then **Recheck now**.

If the domain is *deliberately* behind a proxy (Cloudflare's orange cloud, for
example), Automatic cannot work — the challenge never reaches you. Use the
Cloudflare walkthrough below instead.

### "…does not resolve"

There is no DNS record at all. Create one.

### `urn:ietf:params:acme:error:rateLimited`

Let's Encrypt rate-limits repeated failures, and the limit is per registered
domain, per week. It usually means a broken setup has been retrying for a while.
**Fix the underlying cause first** — otherwise the limit resets and immediately
trips again. Rate limits expire on their own; there is no way to clear one.

### `urn:ietf:params:acme:error:caa`

A CAA DNS record on the domain forbids Let's Encrypt from issuing. Either remove
it or add `letsencrypt.org` to it.

### Connection refused / timeout during the challenge

Port 80 is not reachable from the internet. Check the host firewall and, on a
cloud VM, the provider's security group — they are separate, and it is nearly
always the latter.

### "certificate served for X is valid only for Y"

A Custom certificate is selected whose SANs do not cover this hostname. A browser
would reject it. Upload a certificate that covers the hostname, or switch to
Automatic.

---

## Cloudflare Full (strict)

With Cloudflare proxying enabled ("orange cloud"), Cloudflare terminates TLS for
your visitors, and Automatic issuance cannot work — the ACME challenge reaches
Cloudflare, not you. **Flexible** mode would leave the hop between Cloudflare and
your server unencrypted. **Full (strict)** encrypts it and verifies the
certificate, which is what you want.

1. **Create an Origin CA certificate.** In the Cloudflare dashboard: *SSL/TLS →
   Origin Server → Create Certificate*. Accept the defaults (Cloudflare generates
   the private key). Cover the hostnames you need — for example `example.com` and
   `*.example.com`.

2. **Copy both blocks.** Cloudflare shows the *Origin Certificate* and the
   *Private Key* once. The private key is never shown again.

3. **Upload to Belune.** *Settings → Certificates → Upload Certificate*. Give it a
   name, paste the certificate into **Certificate (PEM)** and the key into
   **Private Key (PEM)**. Belune verifies the pair and reads the hostnames off it
   before storing it, encrypted.

4. **Point the domain at it.** On the application's **Domains** tab, edit the
   domain, set SSL mode to **Custom**, and pick the certificate you just uploaded.

5. **Set Cloudflare to Full (strict).** *SSL/TLS → Overview → Full (strict)*.

6. **Verify.** The domain's TLS badge should go **Active** within a minute, with
   the issuer showing as the Cloudflare Origin CA. If it does not, click
   **Recheck now** and read the reason.

Origin CA certificates are long-lived (up to 15 years), but they do not renew
themselves. Belune notifies admins when one is 14 days from expiry.

---

## The dashboard's own domain

Everything above is about domains you add to an *application*. Belune's own
dashboard is separate: it is served on the server's IP over plain HTTP until you
name a hostname for it under **Server → Configuration → Dashboard domain**.

Naming it there publishes it to Caddy, which is what allows a certificate to be
issued at all — certificates are only ever obtained for hostnames the proxy has
been told about. The badge beside the field shows the same statuses as an app
domain, so you can watch the certificate arrive (or see why it did not).

Clearing the field reverts to plain HTTP on the IP. The requirements are the same
as any Automatic domain: DNS pointing here, ports 80 and 443 open.

After it goes active, set `PUBLIC_BASE_URL=https://<your-domain>` so generated
links (invitations, password resets, webhooks) point at the right place.

Session cookies need no configuration: they are marked `Secure` automatically on
any HTTPS request, so they start being withheld from plain HTTP the moment the
panel is served over TLS.

---

## Certificates and Caddy restarts

Uploaded certificates live in the database, encrypted. Caddy holds them only in
memory, so a Caddy restart drops them — Belune's reconciler notices and pushes
them back, along with the HTTPS listener, within its next pass (30s). No action
is needed; this is why a certificate briefly disappearing from Caddy is not a
cause for alarm.

The private key is never written to Caddy's filesystem, and is never returned by
the API.

---

## Deleting a certificate

Deleting a certificate that a domain is still serving is refused — it would break
HTTPS for that hostname with no warning. The error names the domains still using
it. Point them elsewhere first.
