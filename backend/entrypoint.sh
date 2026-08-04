#!/bin/sh
set -e

if [ ! -f go.sum ]; then
    echo "go.sum not found, running go mod tidy..."
    go mod tidy
fi

exec "$@"
