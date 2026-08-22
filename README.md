# LocalWhisper

Local, LAN-only phone dictation for Fedora Workstation. The Android app captures microphone audio only while requested. The Fedora laptop runs `whisper.cpp` with Vulkan, copies the result to the Wayland clipboard, and never sends audio or text to a cloud service.

```text
Android microphone -> local Wi-Fi -> Go receiver -> whisper.cpp/Vulkan -> wl-copy
```

## What you need

- Fedora Workstation on Wayland with a Vulkan-capable GPU.
- An Android phone running Android 8 or later.
- Both devices on the same trusted Wi-Fi network.
- About 2 GB of free disk space: the default `large-v3-turbo-q5_0` model is about 547 MiB.

Install laptop prerequisites once:

```sh
sudo dnf install go git cmake ninja-build gcc-c++ make vulkan-loader-devel shaderc wl-clipboard curl openssl glib2 python3-gobject python3-cairo gtk3 gtk-layer-shell
```

The receiver uses Go 1.26 or newer. Android users installing the published APK do not need Android Studio or the Android SDK.

## Install on Fedora

Clone this repository and run the installer:

```sh
git clone https://github.com/Namishk/localwhisper.git
cd localwhisper
./scripts/install-desktop.sh
```

The desktop installer builds the receiver, clones and builds `whisper.cpp` with Vulkan under `~/.local/share/localwhisper/whisper.cpp`, downloads the default model, creates a random pairing token, installs a user service, and starts it. It does not require root after the prerequisite package installation. `make install` and `make install-desktop` invoke the same script.

It prints the laptop IP and pairing token. Print them again at any time with:

```sh
hostname -I | awk '{print $1}'
sed -n 's/^LOCALWHISPER_TOKEN=//p' ~/.config/localwhisper/receiver.env
```

The token is private. The configuration file is mode `0600`; do not commit, share, or screenshot it.

## Install on Windows

Windows support has no status overlay; the transcription lands on the clipboard via `clip.exe`. You need Windows 10 or 11 with a Vulkan-capable GPU, plus [Go](https://go.dev/dl/), [Git](https://git-scm.com/download/win), [CMake](https://cmake.org/download/), and Visual Studio Build Tools (C++ workload) on PATH.

Clone this repository and run the installer in PowerShell:

```powershell
git clone https://github.com/Namishk/localwhisper.git
cd localwhisper
powershell -ExecutionPolicy Bypass -File scripts\install-windows.ps1
```

The installer builds the receiver, clones and builds `whisper.cpp` with Vulkan under `%LOCALAPPDATA%\localwhisper\whisper.cpp`, downloads the default model, creates a random pairing token in `%LOCALAPPDATA%\localwhisper\receiver.env`, and registers a logon scheduled task that starts the receiver. Pass `-NoAutostart` to skip the scheduled task.

It prints the laptop IP and pairing token. Print them again at any time with:

```powershell
(Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.IPAddress -notlike '127.*' } | Select-Object -First 1).IPAddress
Select-String -Path "$env:LOCALAPPDATA\localwhisper\receiver.env" -Pattern 'LOCALWHISPER_TOKEN=(.+)'
```

Bind **Ctrl+Space** (Settings > Accessibility > Keyboard, or AutoHotkey) to:

```powershell
powershell -ExecutionPolicy Bypass -File <repo>\scripts\toggle-windows.ps1
```

## Install the Android app

The mobile setup is only the Android APK: download `localwhisper-android-v1.1.0.apk` from the GitHub release and install it on the phone. Android may ask you to allow installs from the browser or file manager used for the download.

Open **LocalWhisper**, enter:

- **Laptop IP**: the IP printed by the installer, for example `192.168.1.20`.
- **Pairing token**: the token from `receiver.env`.

Press **Connect** and grant microphone and notification permission. The foreground service keeps only the WebSocket connection alive; it does not record until the laptop sends **Start**.

## Daily use

Bind this command to **Ctrl+Space** in Fedora Settings → Keyboard → Keyboard Shortcuts → Custom Shortcuts:

```sh
~/.local/bin/localwhisper-toggle
```

Press it once to start recording, speak, then press it again to stop. The transcription is copied to the Wayland clipboard; paste it with Ctrl+V.

The GTK status overlay displays a transparent status glyph at the bottom center of the primary display. Recording shows a microphone with radiating sound waves in warm pink; transcription shows animated equalizer bars in blue, cyan, and violet. A green circled check briefly confirms copied text, while failures and phone disconnections expand into a readable message beside an amber alert badge.

The animated glyphs pre-render their frames once at startup and play the cached animation at 24 FPS, keeping animation CPU use low and frame pacing even on common 120 Hz and 144 Hz displays. The overlay uses native layer-shell positioning on Hyprland, Sway, and KDE Wayland, with an XWayland fallback on GNOME. It respects GTK's reduced-motion setting.

The desktop installer enables the overlay automatically. To reinstall and restart it after a development change—without logging out—run:

```sh
make install-overlay
```

## Hyprland and Waybar

The receiver, Android app, clipboard, toggle command, and GTK overlay work on Hyprland. The desktop installer also adds `~/.local/bin/localwhisper-panel-status`, a Waybar custom module that reads only the local receiver status.

Add `custom/localwhisper` to an existing `modules-right` array in `~/.config/waybar/config.jsonc`, then add this module definition at the top level:

```jsonc
"modules-right": ["network", "pulseaudio", "custom/localwhisper", "battery"],

"custom/localwhisper": {
  "exec": "~/.local/bin/localwhisper-panel-status",
  "return-type": "json",
  "interval": 1,
  "format": "{}"
}
```

Add these colors to `~/.config/waybar/style.css`:

```css
#custom-localwhisper.recording { color: #ff6b6b; }
#custom-localwhisper.transcribing { color: #74c0fc; }
#custom-localwhisper.ready { color: #69db7c; }
#custom-localwhisper.disconnected { color: #a0a0a0; }
```

Bind Ctrl+Space in `~/.config/hypr/hyprland.conf`:

```ini
bind = CTRL, SPACE, exec, ~/.local/bin/localwhisper-toggle
```

Reload both after saving:

```sh
hyprctl reload
pkill -SIGUSR2 waybar
```

## Service and diagnostics

```sh
systemctl --user status localwhisper.service
systemctl --user status localwhisper-overlay.service
journalctl --user -u localwhisper.service -f
journalctl --user -u localwhisper-overlay.service -f
curl --fail http://127.0.0.1:8766/health
curl --silent http://127.0.0.1:8766/status
```

The control server listens only on `127.0.0.1:8766`. The phone WebSocket listener is LAN-reachable on port 8765. If Fedora blocks it, permit the port only on your trusted home zone:

```sh
firewall-cmd --zone=home --add-port=8765/tcp --permanent
firewall-cmd --reload
```

The most recent recorded WAV remains at `/tmp/localwhisper/latest.wav` for debugging. Test the receiver pipeline from a checkout with:

```sh
make build
./scripts/test-transcription.sh
```

## Configuration

`~/.config/localwhisper/receiver.env` is the single receiver configuration file:

```ini
WHISPER_BIN=/home/you/.local/share/localwhisper/whisper.cpp/build/bin/whisper-cli
WHISPER_MODEL=/home/you/.local/share/localwhisper/whisper.cpp/models/ggml-large-v3-turbo-q5_0.bin
WHISPER_THREADS=8
WS_ADDR=:8765
CONTROL_ADDR=127.0.0.1:8766
LOCALWHISPER_INDICATOR=/home/you/.local/share/localwhisper/indicator.py
LOCALWHISPER_TOKEN=your-private-random-token
```

After changing it, run `systemctl --user restart localwhisper.service`. To switch models later, download a compatible `ggml-*.bin` with the upstream `download-ggml-model.sh` helper, set `WHISPER_MODEL` to its path, then restart the service. `large-v3-turbo-q5_0` is multilingual and is the default balance of quality and speed.

To confirm Vulkan inference, record a short utterance and inspect the service log. `whisper-cli` should report a Vulkan device rather than a CPU-only backend. If Vulkan build fails, confirm that `vulkan-loader-devel` and `shaderc` are installed and that `vulkaninfo` sees your GPU.

## Build from source

```sh
make build
make test
make vet
```

For Android development, install Java 17 and Android SDK Platform 35, then build the debug app:

```sh
make android
```

The debug APK is `android/app/build/outputs/apk/debug/app-debug.apk`. To build your own installable release APK, copy `android/signing.properties.example` to `android/signing.properties`, create a private keystore, and run `make release`. Never commit the real properties file or keystore.

## Security and scope

The pairing token prevents arbitrary LAN devices from connecting. LocalWhisper does not provide TLS, public-internet access, cloud transcription, or multi-user access control. Use it only on a trusted local network.

## Platform roadmap

Linux is the currently supported desktop platform.

- TODO: Add a macOS desktop client with a LaunchAgent, Metal-enabled `whisper.cpp`, native clipboard integration, and a floating status window.
- TODO: Add a Windows desktop client with startup registration, Vulkan or CUDA inference, native clipboard integration, and a floating status window.
- TODO: Package the Go receiver and shared Android protocol with each desktop client while keeping audio and transcription local.

## License

LocalWhisper is released under the [MIT License](LICENSE). `whisper.cpp` and its models are downloaded separately and retain their upstream licenses.
