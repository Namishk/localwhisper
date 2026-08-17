#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
status=$(curl --fail --silent --show-error --max-time 1 http://127.0.0.1:8766/status)
if printf '%s' "$status" | grep -q '"phone_connected":false'; then
    "$script_dir/indicator.py" set disconnected
    exit 1
fi

exec curl --fail --silent --show-error --request POST http://127.0.0.1:8766/toggle
