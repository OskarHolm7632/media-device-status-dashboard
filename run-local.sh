#!/bin/sh
set -eu

test -n "${INFRAI_API_KEY:-}" || { echo "INFRAI_API_KEY is required" >&2; exit 1; }
exec go run ./cmd/media-dashboard
