#!/bin/sh
set -e

echo "Rebuilding Nuxt (.nuxt)..."
rm -rf .nuxt
npx nuxt prepare

exec "$@" 2>&1 | sh /app/log-pipe.sh
