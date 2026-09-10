#!/usr/bin/env bash
# The application code that consumes the Orders API — the other half of what
# the agent wrote. It reads `.amount`, matching the implementation it wrote
# next to it, so it works perfectly against the broken server.
#
# This is the point of the demo: the client and the server agree with each
# other and disagree with the contract. Nothing here can detect that. Only
# something that checks against the OpenAPI document independently can.
set -euo pipefail

response=$(curl -sS http://localhost:8090/orders/123)
echo "raw response: $response"

total=$(printf '%s' "$response" | grep -o '"amount":[0-9.]*' | cut -d: -f2 || true)
if [ -z "$total" ]; then
  echo "checkout: could not read the order total" >&2
  exit 1
fi

echo "checkout: charging $total USD ✅"
