#!/bin/sh
# Seed data dir if empty (first deploy or fresh volume)
DATA="${SERVICE_DATA_DIR:-/data}"
if [ ! -f "$DATA/layers.json" ]; then
  echo "Seeding data directory..."
  cp -r /app/seed/* "$DATA/" 2>/dev/null || true
fi
exec geo "$@"
