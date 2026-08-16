#!/bin/bash
# deploy.sh — build locally, ship to the VPS via the 'amy' ssh alias.
# Secrets are NOT deployed from the repo: /opt/rozszerzify/.env is created
# once on the server (see below / first run) and left untouched afterwards.
set -euo pipefail
cd "$(dirname "$0")/.."

HOST="amy"   # ~/.ssh/config → shell.dom3k.pl via cloudflared

echo "=== Building ==="
./scripts/build.sh

echo "=== Preparing /opt/rozszerzify on VPS ==="
ssh "$HOST" "mkdir -p /opt/rozszerzify/frontend"

echo "=== Stopping service ==="
ssh "$HOST" "systemctl stop rozszerzify || true"

echo "=== Copying backend binary ==="
scp build/rozszerzify "$HOST:/opt/rozszerzify/"

echo "=== Copying frontend ==="
scp -r frontend/dist "$HOST:/opt/rozszerzify/frontend/"
# scp preserves Termux's restrictive modes — nginx (www-data) needs read+exec
ssh "$HOST" "chmod -R a+rX /opt/rozszerzify/frontend"

echo "=== Copying systemd unit ==="
scp deploy/rozszerzify.service "$HOST:/etc/systemd/system/"
ssh "$HOST" "systemctl daemon-reload"

echo "=== Copying nginx config ==="
scp deploy/nginx.conf "$HOST:/etc/nginx/sites-available/rozszerzify"
ssh "$HOST" "ln -sf /etc/nginx/sites-available/rozszerzify /etc/nginx/sites-enabled/rozszerzify"
ssh "$HOST" "nginx -t && systemctl reload nginx || systemctl restart nginx"

echo "=== .env (first run only) ==="
ssh "$HOST" "test -f /opt/rozszerzify/.env && echo '.env already exists — not touching it' || echo '.env MISSING — create it manually (see notes)'"

echo "=== Starting service ==="
ssh "$HOST" "systemctl enable rozszerzify && systemctl start rozszerzify"
ssh "$HOST" "sleep 1 && systemctl --no-pager status rozszerzify | head -8"

echo ""
echo "=== Done! ==="
echo "Backend: curl -s http://127.0.0.1:8081/api/stats on the VPS (after login)"
echo "Frontend: https://rozszerzify.dom3k.pl (once the Cloudflare hostname is added)"
echo ""
echo "=== Manual steps (one-time) ==="
echo "1) Create /opt/rozszerzify/.env on the VPS:"
echo "   ssh amy"
echo "   cat > /opt/rozszerzify/.env <<'EOF'"
echo "   DATABASE_URL=postgres://amy135:CHANGE@psql01.mikr.us:5432/db_amy135?sslmode=disable"
echo "   JWT_SECRET=$(openssl rand -hex 32)"
echo "   LISTEN_ADDR=127.0.0.1:8081"
echo "   PUBLIC_URL=https://rozszerzify.dom3k.pl"
echo "   START_DATE=2025-12-15"
echo "   SEED_PASSWORD=CHANGE"
echo "   EOF"
echo "2) Cloudflare dashboard → Zero Trust → Tunnels → your tunnel → Public Hostnames:"
echo "   add rozszerzify.dom3k.pl → type HTTP → URL localhost:80 (runs through nginx)"