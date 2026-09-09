from __future__ import annotations

from pathlib import Path
from unittest import TestCase
from unittest.mock import patch

import repoctx_cli


class LauncherTests(TestCase):
    def test_posix_exec_forwards_arguments(self) -> None:
        binary = Path("/tmp/repoctx")
        with patch.object(repoctx_cli, "_binary_path", return_value=binary), patch.object(
            repoctx_cli.Path, "is_file", return_value=True
        ), patch.object(repoctx_cli.os, "name", "posix"), patch.object(
            repoctx_cli.os, "execv"
        ) as execv:
            self.assertEqual(repoctx_cli.main(("help", "--verbose")), 127)

        execv.assert_called_once_with(str(binary), [str(binary), "help", "--verbose"])

    def test_windows_subprocess_forwards_arguments_and_exit_status(self) -> None:
        binary = Path("C:/repoctx.exe")
        with patch.object(repoctx_cli, "_binary_path", return_value=binary), patch.object(
            repoctx_cli.Path, "is_file", return_value=True
        ), patch.object(repoctx_cli.os, "name", "nt"), patch.object(
            repoctx_cli.subprocess, "call", return_value=23
        ) as call:
            self.assertEqual(repoctx_cli.main(["stats", "index.ir.gz"]), 23)

        call.assert_called_once_with([str(binary), "stats", "index.ir.gz"])

    def test_missing_binary_is_actionable(self) -> None:
        with patch.object(repoctx_cli, "_binary_path", return_value=Path("/missing/repoctx")):
            with self.assertRaisesRegex(SystemExit, "bundled repoctx binary is missing"):
                repoctx_cli.main()


if __name__ == "__main__":
    import unittest

    unittest.main()
