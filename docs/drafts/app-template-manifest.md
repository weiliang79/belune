# App Template Manifest — Format Draft (v0.0.32 review)

> **Status:** draft for review, 2026-07-17 — written BEFORE the v0.0.32 plan so the
> format can be agreed first. Three real examples below (simplest → richest).
> Open questions for the user at the bottom.

## Design ground rules (from the design discussions)

- A template is a **native manifest**, not a compose file — it instantiates existing
  Belune primitives (prebuilt-image apps, managed DBs, volumes, env vars, domains).
- **Deterministic**: one-click deploy, no heuristics. Everything the wizard asks is
  declared in the manifest.
- Field names stay **compose-adjacent** (image / env / volumes) so authoring feels
  familiar and the future compose importer maps 1:1 into this same representation.
- Templates live in-repo (`templates/*.yaml` + `templates/logos/*.svg`), schema-validated
  in CI, embedded into the binary via `go:embed` — the catalog works offline/air-gapped.

## Placeholder syntax (the whole templating language — keep it this small)

| Placeholder | Meaning |
|---|---|
| `{{secret 32}}` | Generate a random secret (N chars) at instantiation |
| `{{input.KEY}}` | Value the wizard asks the user for (declared under `inputs:`) |
| `{{db.NAME.url}}` | Connection URL of managed database NAME |
| `{{db.NAME.host}}` / `.port` / `.user` / `.password` / `.database` | Discrete pieces (many apps want separate vars, not a URL) |
| `{{domain.url}}` / `{{domain.host}}` | The hostname the user picks in the wizard (apps like Ghost/Umami need `APP_URL`) |

---

## Example 1 — Uptime Kuma (simplest: one service, one volume, no DB)

```yaml
schema: 1
id: uptime-kuma
name: Uptime Kuma
description: Self-hosted uptime monitoring with status pages and alerts.
category: monitoring
logo: uptime-kuma.svg          # templates/logos/, served by the catalog API
website: https://uptime.kuma.pet
tags: [monitoring, status-page]

services:
  - name: uptime-kuma
    image: louislam/uptime-kuma:1        # tag; Belune pins the digest at deploy (v0.0.27 machinery)
    port: 3001                            # container port the domain routes to
    volumes:
      - name: data
        mount_path: /app/data
    health_check_path: /

notes: |
  First visit creates the admin account — do it before sharing the URL.
```

## Example 2 — Umami (service + managed Postgres + generated secret)

```yaml
schema: 1
id: umami
name: Umami
description: Privacy-friendly web analytics. A lightweight Google Analytics alternative.
category: analytics
logo: umami.svg
website: https://umami.is
tags: [analytics, privacy]

databases:
  - name: db
    engine: postgres
    version: "16"

services:
  - name: umami
    image: ghcr.io/umami-software/umami:postgresql-latest
    port: 3000
    env:
      DATABASE_URL: "{{db.db.url}}"
      APP_SECRET: "{{secret 32}}"
    health_check_path: /api/heartbeat
    depends_on: [db]                      # deploy ordering only (DB provisions first)

notes: |
  Default login is **admin / umami** — change the password immediately
  (Settings → Profile).
```

## Example 3 — Ghost (discrete DB vars + user input + domain placeholder)

```yaml
schema: 1
id: ghost
name: Ghost
description: Professional publishing platform for blogs and newsletters.
category: cms
logo: ghost.svg
website: https://ghost.org
tags: [blog, cms, newsletter]

inputs:
  - key: admin_email
    label: Admin email
    description: Used for transactional mail sender identity.
    required: true
    validation: email

databases:
  - name: db
    engine: mysql
    version: "8.4"

services:
  - name: ghost
    image: ghost:5
    port: 2368
    env:
      url: "{{domain.url}}"               # Ghost breaks without its public URL
      database__client: mysql
      database__connection__host: "{{db.db.host}}"
      database__connection__port: "{{db.db.port}}"
      database__connection__user: "{{db.db.user}}"
      database__connection__password: "{{db.db.password}}"
      database__connection__database: "{{db.db.database}}"
      mail__from: "{{input.admin_email}}"
    volumes:
      - name: content
        mount_path: /var/lib/ghost/content
    health_check_path: /ghost/api/admin/site/
    depends_on: [db]

notes: |
  Complete setup at `{{domain.url}}/ghost` right after deploy — the first
  visitor to that URL becomes the owner.
```

---

## Instantiation flow (what the wizard does with this)

1. User picks a template from the catalog → wizard shows description, logo, notes preview.
2. Wizard asks: **target project** (new project named after template = default, or existing —
   same rule as compose import), **hostname** (optional; skippable), and each declared `input`.
3. Engine resolves placeholders → creates managed DB(s) → waits for provisioning →
   creates application(s) from image with resolved env/volumes → adds domain (routes to
   the declared `port`) → deploys in `depends_on` order.
4. Objects are stamped with provenance: `source_kind=template`, `source_ref=umami@1`
   (manifest `schema` + catalog git revision).
5. Done-screen shows `notes` (rendered markdown, placeholders resolved).

Everything the engine calls already exists (create app / provision DB / volumes / domains
/ env encryption / digest pinning). The template engine is a resolver + orchestrator over
those service-layer calls.

## Deliberately NOT in schema v1

- `start_command` per service (field is on the launch checklist but catalog images
  don't need it — add to schema when the platform field ships)
- File mounts, resource limits, multiple domains per service (add on demand)
- Remote/community catalog fetching (in-repo only for launch; catalog PRs are the
  community mechanism)
- App-of-apps / template composition

## Decisions (reviewed with user 2026-07-17)

1. **Format: engine accepts BOTH `.json` and `.yaml`** — zero cost (JSON is a strict
   subset of YAML; one `yaml.v3` parser reads both). **Repo catalog convention = JSON**
   (user preference); `templates/schema.json` (JSON Schema) gives editor autocomplete +
   CI validation. Examples above stay YAML for readability of this doc only.
2. **DB placeholder naming stays as-is** (`{{db.<name>.url}}`, second segment = the
   database's declared name). Docs must explain this clearly — a docs task, not a schema
   change.
3. **Logos: manifest declares `logo_url` (author-provided URL); frontend falls back to a
   default lucide icon when absent.** v1 passes the URL through (hotlink). Follow-up
   (post-launch): vendor logos at build time — a script fetches `logo_url` into
   `templates/logos/` so air-gapped instances and privacy-sensitive users get
   self-contained images without changing the authoring UX.
4. **Multi-service templates: IN for v1.** Schema keeps the `services` list fully
   functional (e.g. Hoppscotch — prefer its AIO container where upstream offers one, but
   individual-container templates are expressible). `depends_on` orders deploys.
5. **Catalog list approved** (10–15, personally tested before inclusion): Uptime Kuma,
   Umami, Ghost, n8n, Vaultwarden, Gitea, Plausible, Metabase, Grafana, Homepage/Homarr,
   Excalidraw, IT-Tools, Stirling-PDF (Paperless-ngx deferred — heavier stack).
