#!/bin/bash

set -euo pipefail

TEMPLATE="${1:-readme.templ.md}"
OUT="${2:-readme.md}"

sed -e '/{{CONFIG}}/ {
    r assets/default-config.toml
    d
}' -e '/{{USAGE}}/ {
    r assets/usage.txt
    d
}' "$TEMPLATE" >"$OUT"

echo "Generated $OUT from $TEMPLATE"
