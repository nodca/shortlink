#!/bin/bash
# Build frontend and copy to static directory for Go embedding
# Usage: ./scripts/build-frontend.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
WEB_DIR="$PROJECT_ROOT/internal/app/shortlink/web"
STATIC_DIR="$PROJECT_ROOT/internal/app/shortlink/httpapi/static"

echo "==> Building frontend..."
cd "$WEB_DIR"

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    echo "==> Installing dependencies..."
    npm ci
fi

# Build
npm run build

# Copy to static directory
echo "==> Copying to static directory..."
rm -rf "$STATIC_DIR"
mkdir -p "$STATIC_DIR"
cp -r dist/* "$STATIC_DIR/"

echo "==> Done! Static files:"
ls -la "$STATIC_DIR"
