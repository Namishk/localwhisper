#!/usr/bin/env sh
set -eu

status=$(curl --fail --silent --max-time 1 http://127.0.0.1:8766/status 2>/dev/null || true)

case "$status" in
    *'"state":"RECORDING"'*)
        printf '%s\n' '{"text":"● Recording","class":"recording","tooltip":"LocalWhisper is recording"}'
        ;;
    *'"state":"TRANSCRIBING"'*)
        printf '%s\n' '{"text":"◌ Transcribing","class":"transcribing","tooltip":"LocalWhisper is transcribing"}'
        ;;
    *'"phone_connected":true'*)
        printf '%s\n' '{"text":"● Whisper","class":"ready","tooltip":"LocalWhisper phone connected"}'
        ;;
    *)
        printf '%s\n' '{"text":"● Whisper","class":"disconnected","tooltip":"LocalWhisper phone disconnected"}'
        ;;
esac
