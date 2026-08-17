#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
data_dir="${XDG_DATA_HOME:-$HOME/.local/share}/localwhisper"
service_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

python3 -c "import cairo, gi; gi.require_version('Gtk', '3.0'); gi.require_version('GtkLayerShell', '0.1')"
mkdir -p "$data_dir" "$service_dir"
install -m 755 "$repo_root/scripts/indicator.py" "$data_dir/indicator.py"
install -m 644 "$repo_root/systemd/localwhisper-overlay.service" "$service_dir/localwhisper-overlay.service"
systemctl --user daemon-reload
systemctl --user enable localwhisper-overlay.service
systemctl --user restart localwhisper-overlay.service

if command -v gnome-extensions >/dev/null 2>&1 && gnome-extensions info localwhisper-indicator@local >/dev/null 2>&1; then
    gnome-extensions disable localwhisper-indicator@local 2>/dev/null || true
    gnome-extensions uninstall localwhisper-indicator@local 2>/dev/null || true
fi
