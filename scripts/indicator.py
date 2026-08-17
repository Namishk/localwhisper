#!/usr/bin/env python3
"""Run or update the LocalWhisper desktop status overlay."""

import math
import os
import subprocess
import sys
import time

import cairo

IS_GNOME_WAYLAND = (
    os.environ.get("XDG_SESSION_TYPE") == "wayland"
    and "gnome" in os.environ.get("XDG_CURRENT_DESKTOP", "").lower()
)
if IS_GNOME_WAYLAND:
    os.environ.setdefault("GDK_BACKEND", "x11")

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
from gi.repository import Gdk, Gio, GLib, Gtk


SIGNAL_INTERFACE = "dev.localwhisper.Indicator"
SIGNAL_NAME = "Status"
SIGNAL_PATH = "/dev/localwhisper/Indicator"
STATES = {
    "recording": ("", "#ff6b6b", 0),
    "transcribing": ("", "#74c0fc", 0),
    "copied": ("", "#69db7c", 1800),
    "failed": ("Transcription failed", "#ffd43b", 3500),
    "disconnected": ("Mobile not connected", "#ffd43b", 3500),
}


class StatusGlyph(Gtk.DrawingArea):
    FRAME_COUNT = 24

    def __init__(self):
        super().__init__()
        self.state = "recording"
        self.color = "#ff6b6b"
        self.phase = 0
        self.set_size_request(52, 44)
        self.orb_frames = {
            state: self._render_orb_frames(state) for state in ("recording", "transcribing")
        }
        self.connect("draw", self._draw)

    def update(self, state, color, phase=0):
        self.state = state
        self.color = color
        self.phase = phase
        self.queue_draw()

    def _draw(self, _widget, context):
        width = self.get_allocated_width()
        height = self.get_allocated_height()
        center_x = width / 2
        center_y = height / 2
        context.set_source_rgba(*color_components(self.color))
        context.set_line_cap(1)

        if self.state in ("recording", "transcribing"):
            frame_index = round(self.phase / math.tau * self.FRAME_COUNT) % self.FRAME_COUNT
            context.set_source_surface(self.orb_frames[self.state][frame_index], 0, 0)
            context.paint()
            return False

        if self.state == "copied":
            context.set_line_width(4)
            context.move_to(center_x - 11, center_y)
            context.line_to(center_x - 3, center_y + 8)
            context.line_to(center_x + 12, center_y - 9)
            context.stroke()
            return False

        context.set_line_width(3)
        context.arc(center_x, center_y, 12, 0, math.tau)
        context.stroke()
        context.set_line_width(3)
        context.move_to(center_x, center_y - 7)
        context.line_to(center_x, center_y + 2)
        context.stroke()
        context.arc(center_x, center_y + 7, 1.5, 0, math.tau)
        context.fill()
        return False

    def _render_orb_frames(self, state):
        frames = []
        for frame_index in range(self.FRAME_COUNT):
            surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, 52, 44)
            context = cairo.Context(surface)
            phase = frame_index * math.tau / self.FRAME_COUNT
            self._draw_orb(context, 26, 22, state, phase)
            frames.append(surface)
        return frames

    def _draw_orb(self, context, center_x, center_y, state, phase):
        if state == "recording":
            palette = ((1, 0.2, 0.46), (0.72, 0.24, 1), (1, 0.5, 0.16))
        else:
            palette = ((0.12, 0.7, 1), (0.38, 0.28, 1), (0.15, 1, 0.78))

        halo = cairo.RadialGradient(center_x, center_y, 10, center_x, center_y, 23)
        halo.add_color_stop_rgba(0, *palette[1], 0.22)
        halo.add_color_stop_rgba(0.72, *palette[0], 0.12)
        halo.add_color_stop_rgba(1, *palette[0], 0)
        context.set_source(halo)
        context.arc(center_x, center_y, 23, 0, math.tau)
        context.fill()

        context.save()
        context.arc(center_x, center_y, 19, 0, math.tau)
        context.clip()
        context.set_operator(cairo.OPERATOR_ADD)
        for index, color in enumerate(palette):
            direction = -1 if index == 1 else 1
            angle = direction * phase * (0.48 + index * 0.13) + index * 2.05
            radius = 6 + index
            blob_x = center_x + math.cos(angle) * radius
            blob_y = center_y + math.sin(angle * 1.17) * radius
            blob = cairo.RadialGradient(blob_x, blob_y, 1, blob_x, blob_y, 18)
            blob.add_color_stop_rgba(0, *color, 0.88)
            blob.add_color_stop_rgba(0.5, *color, 0.42)
            blob.add_color_stop_rgba(1, *color, 0)
            context.set_source(blob)
            context.paint()
        context.restore()

        context.set_source_rgba(1, 1, 1, 0.24)
        context.set_line_width(1.2)
        context.arc(center_x, center_y, 19, 0, math.tau)
        context.stroke()


class Overlay:
    def __init__(self):
        self.hide_timeout = 0
        self.animation_timeout = 0
        self.pulse_timeout = 0
        self.animations_enabled = Gtk.Settings.get_default().get_property("gtk-enable-animations")
        self.window = Gtk.Window(type=Gtk.WindowType.TOPLEVEL)
        self.window.set_name("localwhisper-overlay")
        self.window.set_decorated(False)
        self.window.set_resizable(False)
        self.window.set_accept_focus(False)
        self.window.set_focus_on_map(False)
        self.window.set_keep_above(True)
        self.window.set_skip_pager_hint(True)
        self.window.set_skip_taskbar_hint(True)
        self.window.set_app_paintable(True)
        self.window.set_visual(self.window.get_screen().get_rgba_visual())

        content = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        content.set_name("localwhisper-pill")
        content.set_border_width(14)
        self.content = content
        self.symbol = StatusGlyph()
        self.text = Gtk.Label()
        self.text.set_name("localwhisper-text")
        content.pack_start(self.symbol, False, False, 0)
        content.pack_start(self.text, False, False, 0)
        self.window.add(content)
        self._load_styles()
        self._configure_positioning()

        self.connection = Gio.bus_get_sync(Gio.BusType.SESSION, None)
        self.connection.signal_subscribe(
            None,
            SIGNAL_INTERFACE,
            SIGNAL_NAME,
            SIGNAL_PATH,
            None,
            Gio.DBusSignalFlags.NONE,
            self._handle_signal,
        )

    def _load_styles(self):
        styles = b"""
            #localwhisper-pill.with-background {
                background-color: rgba(24, 24, 27, 0.96);
                border: 1px solid rgba(255, 255, 255, 0.16);
                border-radius: 999px;
            }
            #localwhisper-text { color: white; font-size: 16px; font-weight: 600; }
        """
        provider = Gtk.CssProvider()
        provider.load_from_data(styles)
        Gtk.StyleContext.add_provider_for_screen(
            Gdk.Screen.get_default(),
            provider,
            Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
        )

    def _configure_positioning(self):
        if IS_GNOME_WAYLAND or os.environ.get("XDG_SESSION_TYPE") != "wayland":
            return
        try:
            gi.require_version("GtkLayerShell", "0.1")
            from gi.repository import GtkLayerShell

            if not GtkLayerShell.is_supported():
                return
            GtkLayerShell.init_for_window(self.window)
            GtkLayerShell.set_layer(self.window, GtkLayerShell.Layer.OVERLAY)
            GtkLayerShell.set_anchor(self.window, GtkLayerShell.Edge.BOTTOM, True)
            GtkLayerShell.set_margin(self.window, GtkLayerShell.Edge.BOTTOM, 24)
            GtkLayerShell.set_keyboard_mode(self.window, GtkLayerShell.KeyboardMode.NONE)
        except (ImportError, ValueError):
            return

    def _handle_signal(self, _connection, _sender, _path, _interface, _signal, parameters):
        state = parameters.unpack()[0]
        self.show(state)

    def show(self, state):
        text, color, hide_after_ms = STATES.get(state, STATES["failed"])
        self.current_state = state
        self.current_color = color
        self.symbol.update(state, color)
        self.text.set_text(text)
        style = self.content.get_style_context()
        if text:
            style.add_class("with-background")
            self.content.set_border_width(14)
        else:
            style.remove_class("with-background")
            self.content.set_border_width(8)
        if self.hide_timeout:
            GLib.source_remove(self.hide_timeout)
            self.hide_timeout = 0
        self.window.show_all()
        self.text.set_visible(bool(text))
        GLib.idle_add(self._begin_show)
        if hide_after_ms:
            self._stop_pulsing()
        else:
            self._start_pulsing()
        if hide_after_ms:
            self.hide_timeout = GLib.timeout_add(hide_after_ms, self._begin_hide)

    def _begin_show(self):
        if not self.animations_enabled:
            self.content.set_opacity(1)
            self._set_vertical_offset(0)
            return GLib.SOURCE_REMOVE
        self.content.set_opacity(0)
        self._set_vertical_offset(18)
        self._animate(220, self._show_frame)
        return GLib.SOURCE_REMOVE

    def _show_frame(self, progress):
        eased = 1 - (1 - progress) ** 3
        self.content.set_opacity(eased)
        self._set_vertical_offset(round(18 * (1 - eased)))

    def _begin_hide(self):
        self.hide_timeout = 0
        self._stop_pulsing()
        if not self.animations_enabled:
            self._hide()
            return GLib.SOURCE_REMOVE
        self._animate(180, self._hide_frame, self._hide)
        return GLib.SOURCE_REMOVE

    def _hide_frame(self, progress):
        eased = progress * progress
        self.content.set_opacity(1 - eased)
        self._set_vertical_offset(round(18 * eased))

    def _animate(self, duration_ms, update, complete=None):
        if self.animation_timeout:
            GLib.source_remove(self.animation_timeout)
        started = time.monotonic()

        def frame():
            elapsed_ms = (time.monotonic() - started) * 1000
            progress = min(1, elapsed_ms / duration_ms)
            update(progress)
            if progress < 1:
                return GLib.SOURCE_CONTINUE
            self.animation_timeout = 0
            if complete:
                complete()
            return GLib.SOURCE_REMOVE

        self.animation_timeout = GLib.timeout_add(16, frame)

    def _start_pulsing(self):
        if not self.animations_enabled or self.pulse_timeout:
            return
        started = time.monotonic()

        def pulse():
            phase = (time.monotonic() - started) * math.tau / 0.9
            self.symbol.update(self.current_state, self.current_color, phase)
            return GLib.SOURCE_CONTINUE

        self.pulse_timeout = GLib.timeout_add(42, pulse)

    def _stop_pulsing(self):
        if self.pulse_timeout:
            GLib.source_remove(self.pulse_timeout)
            self.pulse_timeout = 0
        if hasattr(self, "current_state"):
            self.symbol.update(self.current_state, self.current_color)

    def _uses_layer_shell(self):
        if IS_GNOME_WAYLAND or os.environ.get("XDG_SESSION_TYPE") != "wayland":
            return False
        try:
            from gi.repository import GtkLayerShell

            return GtkLayerShell.is_layer_window(self.window)
        except (ImportError, ValueError):
            return False

    def _set_vertical_offset(self, offset):
        if self._uses_layer_shell():
            from gi.repository import GtkLayerShell

            GtkLayerShell.set_margin(self.window, GtkLayerShell.Edge.BOTTOM, 24 - offset)
            return
        self._position_window(offset)

    def _position_window(self, offset=0):
        display = Gdk.Display.get_default()
        monitor = display.get_primary_monitor() or display.get_monitor(0)
        geometry = monitor.get_workarea()
        width, height = self.window.get_size()
        x = geometry.x + (geometry.width - width) // 2
        y = geometry.y + geometry.height - height - 24 + offset
        self.window.move(x, y)

    def _hide(self):
        self.content.set_opacity(0)
        self.window.hide()


def emit(state):
    subprocess.run(
        [
            "gdbus",
            "emit",
            "--session",
            "--object-path",
            SIGNAL_PATH,
            "--signal",
            f"{SIGNAL_INTERFACE}.{SIGNAL_NAME}",
            state,
        ],
        check=True,
    )


def color_components(value):
    red = int(value[1:3], 16) / 255
    green = int(value[3:5], 16) / 255
    blue = int(value[5:7], 16) / 255
    return red, green, blue, 1


def main():
    if sys.argv[1:] == ["serve"]:
        Overlay()
        Gtk.main()
        return
    if len(sys.argv) == 3 and sys.argv[1] == "set" and sys.argv[2] in STATES:
        emit(sys.argv[2])
        return
    states = "|".join(sorted(STATES))
    raise SystemExit(f"usage: {sys.argv[0]} serve | set {states}")


if __name__ == "__main__":
    main()
