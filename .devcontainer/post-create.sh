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

echo "==> Fixing ownership of cached Go directories..."
# Named volumes for the Go caches (see .devcontainer/docker-compose.yml) are
# created root-owned on first mount when the target paths don't pre-exist in
# the image. Chown them so the vscode user can write.
sudo mkdir -p /home/vscode/go/pkg/mod /home/vscode/.cache/go-build /home/vscode/go/bin
sudo chown -R vscode:vscode /home/vscode/go /home/vscode/.cache

echo "==> Granting Docker socket access..."
# Match the GID of the host docker socket, add vscode to that group, and open
# the socket so the current session can use it without a full login/logout cycle.
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)
EXISTING_GROUP=$(getent group "$DOCKER_GID" | cut -d: -f1 || true)
if [ -n "$EXISTING_GROUP" ]; then
  DOCKER_GROUP="$EXISTING_GROUP"
else
  sudo groupadd -g "$DOCKER_GID" docker
  DOCKER_GROUP="docker"
fi
sudo usermod -aG "$DOCKER_GROUP" vscode
sudo chmod 666 /var/run/docker.sock

echo "==> Installing Go dev tools..."
# Skip when the binary is already present in the cached ~/go/bin volume.
# Remove the binary from the volume to force a re-install at @latest.
install_go_tool() {
  local bin_name="$1" pkg="$2"
  shift 2
  if command -v "$bin_name" >/dev/null 2>&1; then
    echo "  - $bin_name already installed, skipping"
    return
  fi
  go install "$@" "$pkg"
}
install_go_tool air            github.com/air-verse/air@latest
install_go_tool migrate        github.com/golang-migrate/migrate/v4/cmd/migrate@latest -tags 'postgres'
install_go_tool dlv            github.com/go-delve/delve/cmd/dlv@latest
install_go_tool task           github.com/go-task/task/v3/cmd/task@latest
install_go_tool golangci-lint  github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "==> Installing sqlc (v1.30.0)..."
# sqlc's go.mod has replace directives, which `go install @latest` refuses,
# so we pull the prebuilt binary from GitHub releases.
SQLC_ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
curl -sSL "https://github.com/sqlc-dev/sqlc/releases/download/v1.30.0/sqlc_1.30.0_linux_${SQLC_ARCH}.tar.gz" \
  | sudo tar -C /usr/local/bin --no-same-owner -xz sqlc

echo "==> Installing pack CLI (v0.35.1)..."
curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.35.1/pack-v0.35.1-linux.tgz" \
  | sudo tar -C /usr/local/bin --no-same-owner -xz pack

echo "==> Installing railpack CLI..."
# Uses railpack's official installer; drops a binary in /usr/local/bin/railpack.
curl -sSL https://railpack.com/install.sh | sudo bash

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
