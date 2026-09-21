from __future__ import annotations

import os
import subprocess
import sys
from collections.abc import Sequence
from importlib import resources
from pathlib import Path


def _binary_path() -> Path:
    name = "repoctx.exe" if os.name == "nt" else "repoctx"
    return Path(str(resources.files(__package__).joinpath("bin", name)))


def main(argv: Sequence[str] | None = None) -> int:
    """Forward arguments to the bundled native repoctx command.

    POSIX uses ``execv`` so signals, standard streams and the native process's
    exit status are inherited directly. Windows uses ``subprocess.call`` for
    the equivalent inherited-stream behavior.
    """

    args = list(sys.argv[1:] if argv is None else argv)
    binary = _binary_path()
    if not binary.is_file():
        raise SystemExit(
            "The bundled repoctx binary is missing. Rebuild and reinstall the wheel."
        )

    command = [str(binary), *args]
    if os.name == "nt":
        return subprocess.call(command)

    os.execv(str(binary), command)
    return 127  # pragma: no cover - execv replaces the process.


__all__ = ["main"]
