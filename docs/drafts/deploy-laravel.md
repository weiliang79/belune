# Deploy Laravel on Belune (DRAFT — docs source material)

> **Status:** draft written 2026-07-16 during a planning discussion, before the v0.0.32
> template work. Intended as source material for the public "Deploy Laravel" docs page
> and the flagship Laravel template. Verify the marked items before publishing.

A production Laravel app is four long-running processes, not one:

| Process | Command | Why |
|---|---|---|
| Web | nginx + php-fpm (or FrankenPHP/Octane) | Serves HTTP |
| Queue worker | `php artisan queue:work` | Jobs, mail, notifications |
| Scheduler | `php artisan schedule:work` | Cron tasks (no host cron needed) |
| WebSockets | `php artisan reverb:start` | Laravel Reverb (first-party, Laravel 11+) |

No builder auto-starts the last three — Railpack's PHP provider builds the image and
boots the web process only. The recommended shape on Belune is **one application
running all four processes under supervisord** (Option A). The multi-app shape
(Option B) becomes practical once the `start_command` override ships, and the
long-term answer is the process-types feature.

---

## Option A — one app, supervisor container (works today)

Two files in the repo. Belune's build detector prefers a Dockerfile, so Railpack
steps aside automatically.

### `Dockerfile`

```dockerfile
FROM dunglas/frankenphp:php8.3
RUN install-php-extensions pcntl pdo_mysql redis opcache
RUN apt-get update && apt-get install -y supervisor && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .
RUN composer install --no-dev --optimize-autoloader \
 && php artisan config:cache && php artisan route:cache && php artisan view:cache

COPY docker/supervisord.conf /etc/supervisor/conf.d/laravel.conf
CMD ["supervisord", "-n"]
```

(Alternative base: `serversideup/php:8.3-fpm-nginx` — widely used, has its own
process supervision conventions. FrankenPHP shown here for the simpler single-file
example. Pick ONE for the template and stay consistent.)

### `docker/supervisord.conf`

```ini
[program:web]
command=frankenphp php-server --listen :8080 --root /app/public
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
redirect_stderr=true

[program:queue]
command=php /app/artisan queue:work redis --tries=3 --max-time=3600
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
redirect_stderr=true

[program:scheduler]
command=php /app/artisan schedule:work
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
redirect_stderr=true

[program:reverb]
command=php /app/artisan reverb:start --host=0.0.0.0 --port=8081
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
redirect_stderr=true
```

All programs log to stdout so the container-log collector captures every process in
the app's Logs tab — keep the `stdout_logfile=/dev/stdout` lines.

### Belune setup steps

1. **Create managed services** in the project: MySQL or Postgres, plus Redis.
2. **Create the application** from the git repo (Dockerfile is auto-detected).
3. **Environment variables:**

   ```
   APP_KEY=...
   DB_HOST / DB_PORT / DB_DATABASE / DB_USERNAME / DB_PASSWORD   ← from the managed DB
   REDIS_HOST / REDIS_PORT / REDIS_PASSWORD                      ← from managed Redis
   QUEUE_CONNECTION=redis
   CACHE_STORE=redis
   BROADCAST_CONNECTION=reverb
   REVERB_APP_ID=...   REVERB_APP_KEY=...   REVERB_APP_SECRET=...
   REVERB_HOST=app.example.com   REVERB_PORT=443   REVERB_SCHEME=https
   ```

4. **Domain routing** — two rules on one hostname (path-based routing, v0.0.29):
   - default route → container port **8080** (web)
   - path route `/app` → container port **8081** (Reverb websocket endpoint;
     Echo clients connect to `wss://app.example.com/app/<key>`)
5. **Trade-off to state in docs:** the four processes share one container's
   lifecycle and resource limits, and appear as one unit in the UI. Fine at
   self-hosted scale; process types will lift this later.

### ⚠️ Verify before publishing

- [ ] `VITE_REVERB_*` (and any `VITE_*`) are **build-time** variables — confirm the
      deploy worker passes app env vars into `BuildOptions.Env` (see launch-plan
      memory, builder-plumbing checklist). If not, the frontend example breaks.
- [ ] Path route `/app` → port 8081 works for websocket upgrade through Caddy.
- [ ] Full example deploys end-to-end on a real instance (queue job, scheduled task,
      and Echo event all observable).
- [ ] Exact FrankenPHP flags / `install-php-extensions` availability on the pinned tag.

---

## Option B — four apps, one repo (Railway-style; needs `start_command`)

Same repo, no supervisor, four Belune applications:

| App | Start command | Routed |
|---|---|---|
| `myapp-web` | *(image default — Railpack nginx/php-fpm boot)* | domain → web port |
| `myapp-queue` | `php artisan queue:work redis --tries=3` | — |
| `myapp-scheduler` | `php artisan schedule:work` | — |
| `myapp-reverb` | `php artisan reverb:start --host=0.0.0.0 --port=8081` | path/subdomain |

- Blocked today: applications have no start-command override (on the launch-plan
  checklist). Interim hack: four one-line Dockerfiles via `dockerfile_path`
  (`FROM` shared base, different `CMD`) — costs four builds per deploy.
- Use project-level env vars for the shared config; per-app overrides for the rest.
- This shape gets per-process logs/metrics/restarts at the cost of N builds.

## End state — process types (future feature)

One app declares `{web, queue, scheduler, reverb}`; one build, one container per
process, only web/reverb routed, per-process logs/metrics. Option A's template
migrates naturally (same commands, minus supervisord). See the compose/templates
design memory — process types share the "multiple containers per application"
substrate with replicas and compose import.

## Positioning note (for the docs page intro)

Laravel is the #1 framework in Belune's home region (Malaysia/SEA) and every
competitor treats its multi-process reality as the user's problem. This page +
the flagship template are the "Belune deploys Laravel properly" proof — worth a
dedicated blog post and sharing into Laravel/PHP communities at launch.
