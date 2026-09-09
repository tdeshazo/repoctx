from __future__ import annotations

import os
import shutil
import stat
import subprocess
import sys
from pathlib import Path

from setuptools import setup
from setuptools.command.build_py import build_py as _build_py

try:
    from setuptools.command.bdist_wheel import bdist_wheel as _bdist_wheel
except ImportError:  # pragma: no cover - setuptools may delegate to wheel.
    try:
        from wheel.bdist_wheel import bdist_wheel as _bdist_wheel
    except ImportError:  # pragma: no cover - pyproject requires wheel.
        _bdist_wheel = None


ROOT = Path(__file__).parent.resolve()


def _binary_name() -> str:
    return "repoctx.exe" if sys.platform == "win32" else "repoctx"


class build_py(_build_py):
    """Build the native module-root Go command into the Python package."""

    def run(self) -> None:
        super().run()
        self._build_go_binary()

    def _build_go_binary(self) -> None:
        output = Path(self.build_lib) / "repoctx_cli" / "bin" / _binary_name()
        if output.parent.exists():
            shutil.rmtree(output.parent)
        output.parent.mkdir(parents=True, exist_ok=True)

        # Tree-sitter's Go bindings are CGO-backed. Keep the caller's compiler,
        # flags and Go environment, but never allow a CGO-disabled build to
        # produce a misleading package with a missing native parser.
        env = os.environ.copy()
        env["CGO_ENABLED"] = "1"
        subprocess.check_call(
            [
                "go",
                "build",
                "-buildvcs=false",
                "-trimpath",
                "-o",
                str(output),
                ".",
            ],
            cwd=ROOT,
            env=env,
        )

        output.chmod(output.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


cmdclass = {"build_py": build_py}


if _bdist_wheel is not None:

    class bdist_wheel(_bdist_wheel):
        def finalize_options(self) -> None:
            super().finalize_options()
            self.root_is_pure = False

        def get_tag(self) -> tuple[str, str, str]:
            _python, _abi, platform = super().get_tag()
            return "py3", "none", platform

    cmdclass["bdist_wheel"] = bdist_wheel


setup(cmdclass=cmdclass)
