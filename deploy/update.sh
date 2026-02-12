#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> Pull latest code"
git pull

echo "==> Reset deploy/.env from backend/.env.example"
rm -f deploy/.env
cp backend/.env.example deploy/.env

echo "==> Build and restart containers"
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --build

echo "==> Done"
