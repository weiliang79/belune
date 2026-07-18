# Contributing

## Development Setup

1. Clone the repository
2. Run `./scripts/dev-setup.sh`
3. Start the API: `task dev:api`
4. Start the frontend: `task dev:web`

## Project Structure

- `apps/api/` — Go backend
- `apps/web/` — React (TanStack Router) frontend
- `infra/` — Docker Compose and Caddy configs

## Code Style

- Go: `gofmt` + `golangci-lint`
- TypeScript: Prettier + ESLint
- Run `task fmt` and `task lint` before submitting changes

## Adding an app template

The one-click app catalog is a set of declarative manifests. Adding one is a
good first contribution — no Go or React changes required.

1. **Create the manifest.** Add a JSON file under
   `apps/api/internal/template/catalog/manifests/<id>.json`. The schema lives at
   `apps/api/internal/template/catalog/schema.json` (point your editor at it for
   autocomplete). A template instantiates native Belune objects — prebuilt-image
   applications, managed databases, volumes, env vars, and a domain — so prefer a
   single upstream image (use an all-in-one image where the project offers one).

2. **Use only these placeholders** (the whole templating language):

   | Placeholder | Resolves to |
   |---|---|
   | `{{secret 32}}` | A fresh random secret of N characters, generated at deploy |
   | `{{input.KEY}}` | A value the wizard collects (declare it under `inputs`) |
   | `{{db.NAME.url}}` | Connection URL of the managed database named NAME |
   | `{{db.NAME.host}}` / `.port` / `.user` / `.password` / `.database` | Individual connection fields |
   | `{{domain.url}}` / `{{domain.host}}` | The hostname the user picks in the wizard |

   Referencing `{{domain.*}}` makes the hostname required in the wizard. Managed
   databases support the `postgres`, `mysql`, `redis`, and `mongo` engines
   (the `other` engine is not yet available from templates).

3. **Configure the health check** (optional but recommended). Each service takes
   a `health_check` block that Belune probes after deploy — a pass marks the app
   healthy, a fail rolls the deploy back:

   ```jsonc
   "health_check": {
     "path": "/healthz",       // probed on the service `port`; must start with /
     "timeout_seconds": 300,   // optional; retry window before failing (default 120)
     "expect_status": 200      // optional; require this exact status (default: any 2xx)
   }
   ```

   Omit the whole block to skip health checking. Raise `timeout_seconds` for apps
   that run migrations on first boot (Metabase, for example). Set `expect_status`
   only when a healthy root legitimately returns a non-2xx code.

4. **Validate.** `go test ./internal/template/...` runs the schema and
   referential checks (undeclared placeholder references, `depends_on` cycles,
   duplicate names, etc.) over every manifest in the catalog. CI runs the same
   test, so a malformed manifest fails the build.

5. **Smoke-test it yourself.** Before opening the PR, deploy the template on a
   dev stack end to end: instantiate it, confirm the app reaches its UI and works,
   confirm any managed database provisions, and confirm deleting the project
   cleans everything up. Note the image tag and health-check path you verified in
   the PR description.

Logos are hotlinked from `logo_url` in v1; if the project has no stable logo URL,
omit the field and the UI falls back to a generic icon.
