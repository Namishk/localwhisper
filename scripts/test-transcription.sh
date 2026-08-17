#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec "$root/receiver/localwhisper-integration" "$root/whisper.cpp/samples/jfk.wav"
