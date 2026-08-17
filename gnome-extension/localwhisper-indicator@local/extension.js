import Clutter from 'gi://Clutter';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import St from 'gi://St';

import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

const STATES = {
    recording: {symbol: '●', text: 'Recording', color: '#ff6b6b', hideAfterMs: 0},
    transcribing: {symbol: '◌', text: 'Transcribing…', color: '#74c0fc', hideAfterMs: 0},
    copied: {symbol: '✓', text: 'Copied to clipboard', color: '#69db7c', hideAfterMs: 1800},
    failed: {symbol: '!', text: 'Transcription failed', color: '#ffd43b', hideAfterMs: 3500},
};

export default class LocalWhisperIndicatorExtension extends Extension {
    enable() {
        this._indicator = new St.BoxLayout({
            reactive: false,
            visible: false,
            style: 'background-color: rgba(24, 24, 27, 0.96); border: 1px solid rgba(255, 255, 255, 0.16); border-radius: 999px; padding: 12px 20px; spacing: 10px;',
        });
        this._symbol = new St.Label({style: 'font-size: 18px; font-weight: 700;'});
        this._text = new St.Label({style: 'color: white; font-size: 16px; font-weight: 600;'});
        this._indicator.add_child(this._symbol);
        this._indicator.add_child(this._text);
        Main.layoutManager.addTopChrome(this._indicator);

        this._monitorSignal = Main.layoutManager.connect('monitors-changed', () => this._position());
        this._signal = Gio.DBus.session.signal_subscribe(
            null,
            'dev.localwhisper.Indicator',
            'Status',
            '/dev/localwhisper/Indicator',
            null,
            Gio.DBusSignalFlags.NONE,
            (_connection, _sender, _path, _interface, _signal, parameters) => {
                const [state] = parameters.deep_unpack();
                this._show(state);
            }
        );
        this._hideTimeout = 0;
        this._pulseTimeout = 0;
    }

    disable() {
        if (this._hideTimeout) {
            GLib.source_remove(this._hideTimeout);
        }
        this._stopPulsing();
        if (this._signal) {
            Gio.DBus.session.signal_unsubscribe(this._signal);
        }
        if (this._monitorSignal) {
            Main.layoutManager.disconnect(this._monitorSignal);
        }
        this._indicator.destroy();
        this._indicator = null;
    }

    _show(stateName) {
        const state = STATES[stateName] ?? STATES.failed;
        this._symbol.set_text(state.symbol);
        this._symbol.set_style(`color: ${state.color}; font-size: 18px; font-weight: 700;`);
        this._text.set_text(state.text);
        if (this._hideTimeout) {
            GLib.source_remove(this._hideTimeout);
            this._hideTimeout = 0;
        }
        this._indicator.show();
        this._position();
        this._indicator.opacity = 0;
        this._indicator.translation_y = 18;
        this._indicator.ease({
            opacity: 255,
            translation_y: 0,
            duration: 220,
            mode: Clutter.AnimationMode.EASE_OUT_QUAD,
        });
        if (state.hideAfterMs) {
            this._stopPulsing();
        } else {
            this._startPulsing();
        }
        if (state.hideAfterMs) {
            this._hideTimeout = GLib.timeout_add(GLib.PRIORITY_DEFAULT, state.hideAfterMs, () => {
                this._hide();
                return GLib.SOURCE_REMOVE;
            });
        }
    }

    _hide() {
        this._hideTimeout = 0;
        this._stopPulsing();
        this._indicator.ease({
            opacity: 0,
            translation_y: 18,
            duration: 180,
            mode: Clutter.AnimationMode.EASE_IN_QUAD,
            onComplete: () => this._indicator.hide(),
        });
    }

    _startPulsing() {
        if (this._pulseTimeout) {
            return;
        }
        this._pulse();
        this._pulseTimeout = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 900, () => {
            this._pulse();
            return GLib.SOURCE_CONTINUE;
        });
    }

    _stopPulsing() {
        if (!this._pulseTimeout) {
            return;
        }
        GLib.source_remove(this._pulseTimeout);
        this._pulseTimeout = 0;
        this._symbol.ease({
            opacity: 255,
            scale_x: 1,
            scale_y: 1,
            duration: 120,
            mode: Clutter.AnimationMode.EASE_OUT_QUAD,
        });
    }

    _pulse() {
        this._symbol.ease({
            opacity: 115,
            scale_x: 1.22,
            scale_y: 1.22,
            duration: 300,
            mode: Clutter.AnimationMode.EASE_OUT_QUAD,
            onComplete: () => this._symbol.ease({
                opacity: 255,
                scale_x: 1,
                scale_y: 1,
                duration: 520,
                mode: Clutter.AnimationMode.EASE_IN_OUT_QUAD,
            }),
        });
    }

    _position() {
        const monitor = Main.layoutManager.primaryMonitor;
        const [, width] = this._indicator.get_preferred_width(-1);
        const [, height] = this._indicator.get_preferred_height(width);
        const x = Math.round(monitor.x + (monitor.width - width) / 2);
        const y = monitor.y + monitor.height - height - 24;
        this._indicator.set_position(x, y);
    }
}
