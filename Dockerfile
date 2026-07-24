# ── Stage: build-base ────────────────────────────────────────────────────────
# Shared build/runtime environment for BOTH dev (the devcontainer builds this
# same target) and prod. It pins the whole build toolchain on a glibc >= 2.38
# base: the `belune` binary shells out to railpack/pack at runtime, and railpack
# fetches `mise`, which now requires glibc >= 2.38 (Debian trixie ships 2.41).
# Pinning these versions keeps dev and prod byte-for-byte consistent and stops an
# upstream release from silently breaking builds. See the plan in ~/.claude/plans.
FROM debian:trixie-slim AS build-base
ARG RAILPACK_VERSION=0.30.1
ARG PACK_VERSION=0.35.1
# Client only. Talks to the host daemon over the mounted socket; version need not
# match the host's exactly (the Docker CLI is compatible across daemon versions).
ARG DOCKER_CLI_VERSION=27.5.1
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates git tzdata curl \
    && rm -rf /var/lib/apt/lists/*
# railpack (pinned) — a musl-static binary, so it runs anywhere; the glibc need
# comes from the `mise` it downloads at build time.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) rarch="x86_64" ;; \
      arm64) rarch="arm64" ;; \
      *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/railwayapp/railpack/releases/download/v${RAILPACK_VERSION}/railpack-v${RAILPACK_VERSION}-${rarch}-unknown-linux-musl.tar.gz" \
      | tar -C /usr/local/bin -xz; \
    railpack --version
# pack (pinned) — Cloud Native Buildpacks CLI.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) passet="pack-v${PACK_VERSION}-linux.tgz" ;; \
      arm64) passet="pack-v${PACK_VERSION}-linux-arm64.tgz" ;; \
      *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/buildpacks/pack/releases/download/v${PACK_VERSION}/${passet}" \
      | tar -C /usr/local/bin --no-same-owner -xz pack; \
    pack version
# docker CLI (pinned) — client only, no daemon. railpack builds against the
# remote BuildKit daemon but loads the finished image into the host Docker via
# `docker load`, and it has no flag to do otherwise. Without this binary the
# build completes and then hangs at "sending tarball" forever, because nothing
# drains the image out of BuildKit. The daemon is the host's, reached over the
# mounted socket; only the client belongs in this image.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) darch="x86_64" ;; \
      arm64) darch="aarch64" ;; \
      *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://download.docker.com/linux/static/stable/${darch}/docker-${DOCKER_CLI_VERSION}.tgz" \
      | tar -C /usr/local/bin --no-same-owner --strip-components=1 -xz docker/docker; \
    docker --version

# ── Stage: frontend ──────────────────────────────────────────────────────────
# Pinned to the BUILD platform: the output is static assets, so there is nothing
# architecture-specific to produce and no reason to run npm under emulation.
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /web
COPY apps/web/package*.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build

# ── Stage: backend ───────────────────────────────────────────────────────────
# Match the Go version the devcontainer uses (go.mod requires >= 1.25); a stale
# 1.24 here is what made the prod image silently unbuildable.
#
# Runs on the BUILD platform and cross-compiles to $TARGETARCH. CGO_ENABLED=0
# makes that free, and it keeps multi-arch releases off QEMU, where compiling
# this module for arm64 is slow enough to be a CI liability.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
WORKDIR /app
# Install git for go mod download (private modules)
RUN apk add --no-cache git
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
# Copy compiled frontend into the Go embed directory
COPY --from=frontend /web/build ./web/dist/
# Provided by buildx; default keeps a plain `docker build` working.
ARG TARGETARCH
# VERSION must be declared to be readable here: an undeclared ${VERSION} is a
# shell expansion that is always empty, which is how every image built so far
# shipped reporting "dev".
ARG VERSION=dev
# CGO_ENABLED=0 → fully static binary, runs on any base (Alpine or Debian).
# The -X path must be the real symbol; the linker silently ignores -X for a
# symbol that does not exist, which is the other half of the same bug.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X github.com/weiliang79/belune/internal/version.Version=${VERSION}" \
    -o /belune ./cmd/server

# ── Stage: prod runtime ──────────────────────────────────────────────────────
# Inherits the pinned build toolchain from build-base and adds the belune binary.
FROM build-base AS prod
RUN groupadd -r belune && useradd -r -g belune -m -d /home/belune belune
COPY --from=backend /belune /usr/local/bin/belune
# Writable, persistable location for managed-database logical dumps. The default
# DatabaseBackupDir (/opt/belune/backups/databases) is not creatable by the non-root
# belune user, so point it at a dir we own here. Mount a volume on /data in
# docker-compose.prod.yml so dumps survive container recreation (needed for restore).
RUN mkdir -p /data/backups/databases && chown -R belune:belune /data
ENV DATABASE_BACKUP_DIR=/data/backups/databases
USER belune
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz || exit 1
ENTRYPOINT ["belune"]
