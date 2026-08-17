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
sudo dnf install go git cmake ninja-build gcc-c++ make vulkan-loader-devel shaderc wl-clipboard curl openssl glib2
```

The receiver uses Go 1.26 or newer. Android users installing the published APK do not need Android Studio or the Android SDK.

## Install on Fedora

Clone this repository and run the installer:

```sh
git clone https://github.com/Namishk/localwhisper.git
cd localwhisper
make install
```

The installer builds the receiver, clones and builds `whisper.cpp` with Vulkan under `~/.local/share/localwhisper/whisper.cpp`, downloads the default model, creates a random pairing token, installs a user service, and starts it. It does not require root after the prerequisite package installation.

It prints the laptop IP and pairing token. Print them again at any time with:

```sh
hostname -I | awk '{print $1}'
sed -n 's/^LOCALWHISPER_TOKEN=//p' ~/.config/localwhisper/receiver.env
```

The token is private. The configuration file is mode `0600`; do not commit, share, or screenshot it.

## Install the Android app

Download `localwhisper-android-v1.1.0.apk` from the GitHub release and install it on the phone. Android may ask you to allow installs from the browser or file manager used for the download.

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

The optional GNOME indicator appears at the bottom center of the primary display, slides in while recording/transcribing, and pulses while active. Install it from the checkout:

```sh
make install-indicator
```

Log out and back in after the first installation, then enable it:

```sh
gnome-extensions enable localwhisper-indicator@local
```

## Service and diagnostics

```sh
systemctl --user status localwhisper.service
journalctl --user -u localwhisper.service -f
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

## License

LocalWhisper is released under the [MIT License](LICENSE). `whisper.cpp` and its models are downloaded separately and retain their upstream licenses.
