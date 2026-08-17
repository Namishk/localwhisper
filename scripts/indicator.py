#!/usr/bin/env python3
"""Send LocalWhisper status to the local GNOME Shell indicator."""

import subprocess
import sys

STATES = {"recording", "transcribing", "copied", "failed"}


def main():
    if len(sys.argv) != 3 or sys.argv[1] != "set" or sys.argv[2] not in STATES:
        raise SystemExit(f"usage: {sys.argv[0]} set {'|'.join(sorted(STATES))}")
    subprocess.run(
        [
            "gdbus",
            "emit",
            "--session",
            "--object-path",
            "/dev/localwhisper/Indicator",
            "--signal",
            "dev.localwhisper.Indicator.Status",
            sys.argv[2],
        ],
        check=True,
    )


if __name__ == "__main__":
    main()
