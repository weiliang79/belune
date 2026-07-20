# Applying configuration changes

Saving a change in Belune never touches the running container. The container
keeps serving the configuration it was started with until you explicitly apply
the change — so a bad edit cannot take your app down by accident, and you can
stage several edits and apply them once.

The badge beside the application name tells you when something is outstanding
and which button applies it.

| Badge | Meaning | Apply with |
|---|---|---|
| *(none)* | The running container matches what is saved. | — |
| **Reload to apply** | Configuration changed. The image is still correct. | **Reload** |
| **Deploy to apply** | The build source changed. A new image is needed. | **Deploy** |

The badge appears only after an application has deployed successfully at least
once — before that there is no running container to disagree with.

## Which change needs which

**Reload** recreates the container from the image it is already running. That
applies anything the container is *started* with:

- environment variables
- volumes and file mounts
- CPU and memory limits
- the runtime profile (read-only rootfs, capabilities)
- the health-check path

**Deploy** builds or pulls a new image, then recreates the container. Required
for anything that changes what gets *built*:

- the source image (image applications)
- the branch, Dockerfile path, builder image, or build type override
- git credentials or the git integration

Deploy applies configuration too, so it is always a valid way to clear either
badge — it is just slower, and for a git application it builds from the latest
commit on the branch rather than the one you are running.

**Rebuild** is deliberately not offered by the badge. It rebuilds the *pinned*
commit, which is narrower: it picks up base-image and dependency updates without
moving to a newer commit. It will not pick up a changed branch or image.

## Why a reload does not clear "Deploy to apply"

Reload does no build and no pull. If you change an image tag and reload, the
container is recreated from the image that is already on the host — the old one.
The source badge therefore survives a reload on purpose, and clears only when a
real build or pull has happened.

The clearest case is a rollback: change the image, then roll back. The rollback
succeeds, but you still need a deploy to get the new image, so the badge stays.

## Changing where an application comes from

Settings → Source switches an application between building from git and running
a prebuilt image, without recreating it. Use this rather than deleting and
making a new one: deleting removes the persistent data volumes and cascades
domains, environment variables, file mounts, and deployment history, and
re-adding the domains re-issues certificates against Let's Encrypt's limit of
five duplicates per week.

**Kept:** domains and their certificates, volumes and their data, file mounts,
environment variables, resource limits, the runtime profile, the deploy hook,
and deployment history.

**Replaced:** the source itself. Switching to an image clears the repository,
branch, build settings, git credentials, and the push webhook secret — those
authenticate against a repository the application no longer has, and a push hook
that can never fire is worse than no hook. Switching to git clears the image
reference.

The switch stamps "Deploy to apply". The container keeps serving the old image
until you deploy, so a switch is not itself an outage.

It is refused while a deploy is running (the worker would build one source and
deploy the other) and while the application has preview environments, which are
git-only by construction and would be orphaned.

After switching to git, **Rebuild** will fail until the first deploy: it rebuilds
a recorded commit, and there is not one yet. Use **Deploy**.

## Health checks

Settings → Health Check decides how the platform judges an application healthy.
Three methods:

- **None** — no check.
- **HTTP** — after each deploy the control plane makes one request to a path on
  the container. A non-2xx response (or a mismatch with the expected status)
  fails the deploy and rolls back. Needs nothing installed in the image, but
  only works for HTTP services. This is the historical behaviour.
- **Command** — a native Docker `HEALTHCHECK`: a command run *inside* the
  container, continuously, on an interval. It works for anything (a database, a
  queue, a worker), and because it keeps running, the container's health drives
  the application's status — a failing check shows the app as **Unhealthy**
  (up, but not passing), distinct from Stopped or Error.

A command check depends on the tool it calls (`curl`, `wget`, `pg_isready`,
`redis-cli`, …) being present in the image; a minimal or distroless image may
not have it. The HTTP check has no such requirement.

The check is part of the container, so a change takes effect on the next deploy
or reload, not on save — the badge beside the application name says which. On
that deploy, a command check gates success the same way HTTP does: the deploy
waits for Docker to report `healthy`, and fails if the check does not pass in
time.

**Unhealthy** is corrected automatically: when the check next passes, the app
returns to Running; if the container stops or is removed, it moves to Stopped.

## Edits made during a deploy

A change saved *while* a deploy is running is not included in that deploy — the
configuration was already read. Its badge survives that deploy and clears on the
next one, so the indicator never claims an edit is live when it is not.

## Troubleshooting

**The badge will not clear.** Check the Deployments tab: the marker clears only
on a *successful* deploy. A failed deploy leaves the badge in place and sets the
application status to `error`.

**A change seems applied but the badge is still showing.** Some changes are
visible without being applied — editing an env var updates the Env Vars tab
immediately, but the running process still has the old value in its environment.
The badge is describing the container, not the page.

**No badge, but the app behaves as if the old config is live.** Restart is not
Reload. **Restart** starts the same container again and changes nothing about
its configuration; **Reload** creates a new one. If in doubt, use Reload.
