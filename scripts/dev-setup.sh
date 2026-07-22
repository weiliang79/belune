#!/usr/bin/env bash
set -euo pipefail

echo "==> Setting up development environment..."

# Check prerequisites
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed."; exit 1; }
command -v node >/dev/null 2>&1 || { echo "Node.js is required but not installed."; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Docker is required but not installed."; exit 1; }
command -v task >/dev/null 2>&1 || { echo "Task (taskfile.dev) is required but not installed."; exit 1; }

# Copy .env if not exists
if [ ! -f .env ]; then
    cp .env.example.dev .env
    echo "==> Created .env from .env.example.dev — please update with real values"
fi

# Install Go dependencies
echo "==> Installing Go dependencies..."
cd apps/api && go mod download && cd ../..

# Install Node dependencies
echo "==> Installing Node dependencies..."
cd apps/web && npm install && cd ../..

# Start infrastructure
echo "==> Starting infrastructure..."
task dev:infra

# Run migrations
echo "==> Running database migrations..."
task db:migrate:up

echo "==> Setup complete!"
echo "    Run 'task dev:api' and 'task dev:web' to start developing."
