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

# ── Stage: frontend ──────────────────────────────────────────────────────────
FROM node:24-alpine AS frontend
WORKDIR /web
COPY apps/web/package*.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build

# ── Stage: backend ───────────────────────────────────────────────────────────
# Match the Go version the devcontainer uses (go.mod requires >= 1.25); a stale
# 1.24 here is what made the prod image silently unbuildable.
FROM golang:1.26-alpine AS backend
WORKDIR /app
# Install git for go mod download (private modules)
RUN apk add --no-cache git
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
# Copy compiled frontend into the Go embed directory
COPY --from=frontend /web/build ./web/dist/
# CGO_ENABLED=0 → fully static binary, runs on any base (Alpine or Debian).
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${VERSION:-dev}" \
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
