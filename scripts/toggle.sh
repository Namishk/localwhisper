#!/usr/bin/env sh
set -eu

exec curl --fail --silent --show-error --request POST http://127.0.0.1:8766/toggle
