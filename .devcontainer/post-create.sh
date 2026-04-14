#!/usr/bin/env bash
set -euo pipefail

# Allow go to auto-download a newer toolchain when a module requires it
export GOTOOLCHAIN=auto

# Unset TMPDIR so Go uses the system /tmp for build cache, not /var/tmp/paas-builds.
# That bind-mount is only for pack/buildpack deploy operations and has limited space.
unset TMPDIR

# Limit parallel package compilation to reduce peak memory usage inside the container.
export GOFLAGS="-p=2"

echo "==> Ensuring build temp dir is writable..."
# Must happen first — TMPDIR is set to this path in docker-compose, so Go uses it
# for its own build cache before we get to the bottom of this script.
sudo chmod 777 /var/tmp/paas-builds

echo "==> Granting Docker socket access..."
# Match the GID of the host docker socket, add vscode to that group, and open
# the socket so the current session can use it without a full login/logout cycle.
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)
sudo groupadd -g "$DOCKER_GID" docker 2>/dev/null \
  || sudo groupmod -g "$DOCKER_GID" docker 2>/dev/null \
  || true
sudo usermod -aG docker vscode
sudo chmod 666 /var/run/docker.sock

echo "==> Installing Go dev tools..."
go install github.com/air-verse/air@latest
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/go-delve/delve/cmd/dlv@latest
go install github.com/go-task/task/v3/cmd/task@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "==> Installing pack CLI (v0.35.1)..."
curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.35.1/pack-v0.35.1-linux.tgz" \
  | sudo tar -C /usr/local/bin --no-same-owner -xz pack

echo "==> Downloading Go modules..."
cd /workspaces/selfhost-paas/apps/api && go mod download

echo "==> Installing Docker CLI (best-effort)..."
# The Docker socket is bind-mounted; we just need the CLI binary.
# Wrapped in || true so a transient apt failure doesn't break the whole setup.
(
  curl -fsSL https://download.docker.com/linux/debian/gpg \
    | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] \
    https://download.docker.com/linux/debian bookworm stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  sudo apt-get update -qq
  sudo apt-get install -y -qq docker-ce-cli
) || echo "WARNING: Docker CLI install failed — run manually if needed: sudo apt-get install docker-ce-cli"

echo "==> post-create done."
