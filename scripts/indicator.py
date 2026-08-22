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


def ease_out_cubic(progress):
    return 1 - (1 - progress) ** 3


def ease_out_back(progress, overshoot=1.70158):
    p = progress - 1
    return max(0.01, 1 + (overshoot + 1) * p**3 + overshoot * p**2)


class StatusGlyph(Gtk.DrawingArea):
    FRAME_COUNT = 36
    WIDTH = 52
    HEIGHT = 44
    BADGE_POP_SECONDS = 0.28

    MIC_COLOR = (1, 0.42, 0.42)
    RIPPLE_COLOR = (1, 0.42, 0.42)
    BAR_COLOR = (0.455, 0.753, 0.988)
    BARS = (
        (-10.5, 5.1, 0.62),
        (-3.5, 1.7, 1.0),
        (3.5, 3.4, 0.92),
        (10.5, 0.4, 0.58),
    )
    COPIED_COLOR = (0.41, 0.86, 0.49)
    ALERT_COLOR = (1, 0.83, 0.23)

    def __init__(self):
        super().__init__()
        self.state = "recording"
        self.color = "#ff6b6b"
        self.phase = 0
        self.badge_timeout = 0
        self.badge_started = 0.0
        self.set_size_request(self.WIDTH, self.HEIGHT)
        self.frames = {
            state: self._render_frames(state) for state in ("recording", "transcribing")
        }
        self.connect("draw", self._draw)

    def update(self, state, color, phase=0):
        self.state = state
        self.color = color
        self.phase = phase
        if self.badge_timeout:
            GLib.source_remove(self.badge_timeout)
            self.badge_timeout = 0
        if state not in ("recording", "transcribing"):
            self.badge_started = time.monotonic()
            self.badge_timeout = GLib.timeout_add(16, self._badge_frame)
        self.queue_draw()

    def _badge_frame(self):
        if time.monotonic() - self.badge_started >= self.BADGE_POP_SECONDS:
            self.badge_timeout = 0
        self.queue_draw()
        return GLib.SOURCE_CONTINUE if self.badge_timeout else GLib.SOURCE_REMOVE

    def _draw(self, _widget, context):
        if self.state in ("recording", "transcribing"):
            frame_index = round(self.phase / math.tau * self.FRAME_COUNT) % self.FRAME_COUNT
            context.set_source_surface(self.frames[self.state][frame_index], 0, 0)
            context.paint()
            return False
        progress = 1
        if self.badge_timeout:
            progress = min(1, (time.monotonic() - self.badge_started) / self.BADGE_POP_SECONDS)
        glyphs = {
            "copied": (self._draw_check, self.COPIED_COLOR, True),
            "failed": (self._draw_alert, self.ALERT_COLOR, True),
            "disconnected": (self._draw_phone_off, self.ALERT_COLOR, False),
        }
        glyph, color, ring = glyphs.get(self.state, glyphs["failed"])
        self._draw_badge(context, color, glyph, progress, ring)
        return False

    def _render_frames(self, state):
        frames = []
        for frame_index in range(self.FRAME_COUNT):
            surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, self.WIDTH, self.HEIGHT)
            context = cairo.Context(surface)
            phase = frame_index / self.FRAME_COUNT
            if state == "recording":
                self._draw_recording(context, self.WIDTH / 2, self.HEIGHT / 2, phase)
            else:
                self._draw_transcribing(context, self.WIDTH / 2, self.HEIGHT / 2, phase)
            frames.append(surface)
        return frames

    def _draw_recording(self, context, cx, cy, phase):
        for index in range(2):
            travel = (phase + index * 0.5) % 1
            context.set_source_rgba(*self.RIPPLE_COLOR, 0.5 * (1 - travel) ** 1.6)
            context.set_line_width(2)
            context.new_sub_path()
            context.arc(cx, cy - 2, 13 + travel * 9, 0, math.tau)
            context.stroke()

        breath = 1 + 0.045 * math.sin(math.tau * phase)
        context.save()
        context.translate(cx, cy)
        context.scale(breath, breath)
        context.translate(-cx, -cy)
        self._draw_mic(context, cx, cy)
        context.restore()

    def _draw_mic(self, context, cx, cy):
        cradle_y = cy - 2
        context.set_source_rgba(*self.MIC_COLOR, 0.95)
        context.set_line_width(3)
        self._rounded_rect(context, cx - 4.5, cy - 12, cx + 4.5, cy, 4.5)
        context.fill()
        context.new_sub_path()
        context.arc(cx, cradle_y, 8, -0.55, math.pi + 0.55)
        context.stroke()
        context.move_to(cx, cradle_y + 8)
        context.line_to(cx, cy + 9)
        context.stroke()
        context.move_to(cx - 5, cy + 9)
        context.line_to(cx + 5, cy + 9)
        context.stroke()

    def _draw_transcribing(self, context, cx, cy, phase):
        context.set_line_cap(1)
        context.set_line_width(3.5)
        for dx, offset, amplitude in self.BARS:
            level = (0.5 + 0.5 * math.sin(math.tau * phase + offset)) ** 1.6
            height = 7 + 12 * amplitude * level
            context.set_source_rgba(*self.BAR_COLOR, 0.95)
            context.move_to(cx + dx, cy - height / 2)
            context.line_to(cx + dx, cy + height / 2)
            context.stroke()

    def _draw_badge(self, context, color, glyph, progress=1, ring=True):
        cx = self.WIDTH / 2
        cy = self.HEIGHT / 2
        fade = min(1, progress * 3)
        context.save()
        context.translate(cx, cy)
        pop = ease_out_back(progress)
        context.scale(pop, pop)
        context.translate(-cx, -cy)
        if ring:
            context.set_source_rgba(*color, 0.16 * fade)
            context.arc(cx, cy, 14, 0, math.tau)
            context.fill()
            context.set_source_rgba(*color, 0.92 * fade)
            context.set_line_width(2)
            context.arc(cx, cy, 14, 0, math.tau)
            context.stroke()
        glyph(context, cx, cy, fade)
        context.restore()

    def _draw_check(self, context, cx, cy, alpha=1):
        context.set_source_rgba(*self.COPIED_COLOR, alpha)
        context.set_line_width(3)
        context.set_line_cap(1)
        context.set_line_join(1)
        context.move_to(cx - 6, cy + 0.5)
        context.line_to(cx - 1.5, cy + 5)
        context.line_to(cx + 6.5, cy - 5)
        context.stroke()

    def _draw_alert(self, context, cx, cy, alpha=1):
        context.set_source_rgba(*self.ALERT_COLOR, alpha)
        context.set_line_width(3)
        context.set_line_cap(1)
        context.move_to(cx, cy - 7)
        context.line_to(cx, cy + 2)
        context.stroke()
        context.arc(cx, cy + 6.5, 1.7, 0, math.tau)
        context.fill()

    def _draw_phone_off(self, context, cx, cy, alpha=1):
        # large slashed phone, no enclosing ring so it stays legible at pill size
        context.set_source_rgba(*self.ALERT_COLOR, alpha)
        context.set_line_width(2.5)
        context.set_line_cap(1)
        self._rounded_rect(context, cx - 8, cy - 11, cx + 2, cy + 11, 3)
        context.stroke()
        # dark underlay cuts the slash cleanly through the phone outline
        context.set_source_rgba(0.094, 0.094, 0.106, alpha)
        context.set_line_width(6.5)
        context.move_to(cx - 11, cy - 11)
        context.line_to(cx + 9, cy + 9)
        context.stroke()
        context.set_source_rgba(*self.ALERT_COLOR, alpha)
        context.set_line_width(3.5)
        context.move_to(cx - 11, cy - 11)
        context.line_to(cx + 9, cy + 9)
        context.stroke()

    @staticmethod
    def _rounded_rect(context, x0, y0, x1, y1, radius):
        context.new_sub_path()
        context.arc(x1 - radius, y0 + radius, radius, -math.pi / 2, 0)
        context.arc(x1 - radius, y1 - radius, radius, 0, math.pi / 2)
        context.arc(x0 + radius, y1 - radius, radius, math.pi / 2, math.pi)
        context.arc(x0 + radius, y0 + radius, radius, math.pi, 3 * math.pi / 2)
        context.close_path()


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
        style.add_class("with-background")
        self.content.set_border_width(14 if text else 9)
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
        self._animate(260, self._show_frame)
        return GLib.SOURCE_REMOVE

    def _show_frame(self, progress):
        eased = ease_out_back(progress)
        self.content.set_opacity(min(1, eased))
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

        self.pulse_timeout = GLib.timeout_add(33, pulse)

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
