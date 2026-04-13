#!/usr/bin/env bash
set -euo pipefail

echo "==> Installing Go dev tools..."

go install github.com/air-verse/air@latest
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/go-delve/delve/cmd/dlv@latest
go install github.com/go-task/task/v3/cmd/task@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "==> Installing pack CLI (v0.35.1)..."
curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.35.1/pack-v0.35.1-linux.tgz" \
  | tar -C /usr/local/bin --no-same-owner -xz pack

echo "==> Downloading Go modules..."
cd /workspaces/selfhost-paas/apps/api && go mod download

echo "==> Ensuring build temp dir is writable..."
chmod 777 /var/tmp/paas-builds

echo "==> post-create done."
