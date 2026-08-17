#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
data_dir="${XDG_DATA_HOME:-$HOME/.local/share}/localwhisper"
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/localwhisper"
bin_dir="$HOME/.local/bin"
model_name="large-v3-turbo-q5_0"
whisper_dir="$data_dir/whisper.cpp"

need_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf 'Missing required command: %s\n' "$1" >&2
        exit 1
    fi
}

for command in go git cmake ninja openssl wl-copy curl gdbus; do
    need_command "$command"
done

mkdir -p "$data_dir" "$config_dir" "$bin_dir" "$HOME/.config/systemd/user"

if [ ! -x "$repo_root/receiver/localwhisper" ]; then
    (cd "$repo_root/receiver" && go build -o localwhisper ./cmd/localwhisper)
fi

if [ ! -d "$whisper_dir/.git" ]; then
    git clone --depth 1 https://github.com/ggml-org/whisper.cpp.git "$whisper_dir"
fi

if [ ! -x "$whisper_dir/build/bin/whisper-cli" ]; then
    cmake -S "$whisper_dir" -B "$whisper_dir/build" -G Ninja -DGGML_VULKAN=ON -DCMAKE_BUILD_TYPE=Release
    cmake --build "$whisper_dir/build" --config Release
fi

model="$whisper_dir/models/ggml-$model_name.bin"
if [ ! -f "$model" ]; then
    "$whisper_dir/models/download-ggml-model.sh" "$model_name"
fi

if [ ! -f "$config_dir/receiver.env" ]; then
    token=$(openssl rand -hex 32)
    cat >"$config_dir/receiver.env" <<EOF
WHISPER_BIN=$whisper_dir/build/bin/whisper-cli
WHISPER_MODEL=$model
WHISPER_THREADS=8
WS_ADDR=:8765
CONTROL_ADDR=127.0.0.1:8766
LOCALWHISPER_INDICATOR=$data_dir/indicator.py
LOCALWHISPER_TOKEN=$token
EOF
    chmod 600 "$config_dir/receiver.env"
fi

install -m 755 "$repo_root/receiver/localwhisper" "$bin_dir/localwhisper"
install -m 755 "$repo_root/scripts/indicator.py" "$data_dir/indicator.py"
install -m 755 "$repo_root/scripts/toggle.sh" "$bin_dir/localwhisper-toggle"
install -m 755 "$repo_root/scripts/panel-status.sh" "$bin_dir/localwhisper-panel-status"
install -m 644 "$repo_root/systemd/localwhisper.service" "$HOME/.config/systemd/user/localwhisper.service"
systemctl --user daemon-reload
systemctl --user enable --now localwhisper.service

printf '\nLocalWhisper desktop is ready. Laptop IP: '
hostname -I | awk '{print $1}'
printf 'Pairing token: '
sed -n 's/^LOCALWHISPER_TOKEN=//p' "$config_dir/receiver.env"
printf '\nInstall the Android APK, enter both values, then bind Ctrl+Space to: %s\n' "$bin_dir/localwhisper-toggle"
