#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
data_dir="${XDG_DATA_HOME:-$HOME/.local/share}/localwhisper"
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/localwhisper"
bin_dir="$HOME/.local/bin"
model_name="large-v3-turbo-q5_0"
whisper_dir="$data_dir/whisper.cpp"
src_dir="$data_dir/src"
repo_url="${LOCALWHISPER_REPO:-https://github.com/Namishk/localwhisper.git}"

# Fedora packages providing the commands and Python modules this installer needs.
fedora_packages="go git cmake ninja-build gcc-c++ make vulkan-loader-devel shaderc wl-clipboard curl openssl glib2 python3-gobject python3-cairo gtk3 gtk-layer-shell"

have_prerequisites() {
    for command in go git cmake ninja openssl wl-copy curl gdbus python3; do
        command -v "$command" >/dev/null 2>&1 || return 1
    done
    python3 -c "import cairo, gi; gi.require_version('Gtk', '3.0'); gi.require_version('GtkLayerShell', '0.1')" \
        >/dev/null 2>&1 || return 1
}

# Install prerequisites on Fedora. Other distributions must install the
# equivalents themselves; we only report what is missing.
if ! have_prerequisites; then
    if command -v dnf >/dev/null 2>&1; then
        printf 'Installing Fedora prerequisites (sudo required)...\n'
        # shellcheck disable=SC2086
        sudo dnf install -y $fedora_packages
    fi
    if ! have_prerequisites; then
        printf 'Missing prerequisites. Install the equivalents of: %s\n' "$fedora_packages" >&2
        exit 1
    fi
fi

# The script can be curled and run on its own; fetch the source when it is not
# sitting inside a checkout.
if [ ! -f "$repo_root/receiver/go.mod" ]; then
    printf 'Downloading source from %s\n' "$repo_url"
    if [ -d "$src_dir/.git" ]; then
        git -C "$src_dir" pull --ff-only
    else
        mkdir -p "$data_dir"
        rm -rf "$src_dir"
        git clone --depth 1 "$repo_url" "$src_dir"
    fi
    repo_root="$src_dir"
fi

mkdir -p "$data_dir" "$config_dir" "$bin_dir" "$HOME/.config/systemd/user"

(cd "$repo_root/receiver" && go build -o localwhisper ./cmd/localwhisper)

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
install -m 644 "$repo_root/systemd/localwhisper-overlay.service" "$HOME/.config/systemd/user/localwhisper-overlay.service"
systemctl --user daemon-reload
systemctl --user enable --now localwhisper.service
systemctl --user enable localwhisper-overlay.service
systemctl --user restart localwhisper-overlay.service

if command -v gnome-extensions >/dev/null 2>&1 && gnome-extensions info localwhisper-indicator@local >/dev/null 2>&1; then
    gnome-extensions disable localwhisper-indicator@local 2>/dev/null || true
    gnome-extensions uninstall localwhisper-indicator@local 2>/dev/null || true
fi

printf '\nLocalWhisper desktop is ready. Laptop IP: '
hostname -I | awk '{print $1}'
printf 'Pairing token: '
sed -n 's/^LOCALWHISPER_TOKEN=//p' "$config_dir/receiver.env"
printf '\nInstall the Android APK, enter both values, then bind Ctrl+Space to: %s\n' "$bin_dir/localwhisper-toggle"
