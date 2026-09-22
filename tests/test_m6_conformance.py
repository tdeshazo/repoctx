"""Cross-boundary conformance checks for the model-neutral adapter."""

import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "m6_bridge", ROOT / "examples/agent/tool_bridge.py"
)
BRIDGE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BRIDGE)


class AdapterConformanceTests(unittest.TestCase):
    def test_policy_and_caps_survive_adapter_boundary(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            root = base / "repository"
            root.mkdir()
            binary = base / "repoctx"
            index = base / "repo.ir.json"
            binary.touch()
            index.touch()
            tool = BRIDGE.RepositoryContextTool(
                str(binary),
                str(root),
                str(index),
                max_bytes=4096,
                denied_prefixes=("private", "vendor"),
            )
            payload = (
                b'{"version":"repoctx.context/v1alpha3",'
                b'"trust":{"role":"untrusted_repository_data"}}\n'
            )

            with patch.object(BRIDGE.subprocess, "run") as run:
                run.return_value.returncode = 0
                run.return_value.stderr = b""
                run.return_value.stdout = payload
                actual = tool.get_context(
                    "zephyr",
                    ["s:run"],
                    snapshot="sha256:" + "a" * 64,
                    depth=2,
                    unit_ids=["u:guide"],
                )

                self.assertEqual(actual.encode(), payload)
                self.assertEqual(
                    run.call_args.args[0],
                    [
                        str(binary.resolve()),
                        "context",
                        "-root",
                        str(root.resolve()),
                        "-max-bytes",
                        "4096",
                        "-depth",
                        "2",
                        "-query",
                        "zephyr",
                        "-format",
                        "json",
                        "-symbol",
                        "s:run",
                        "-unit",
                        "u:guide",
                        "-deny",
                        "private",
                        "-deny",
                        "vendor",
                        "-expect-snapshot",
                        "sha256:" + "a" * 64,
                        str(index.resolve()),
                    ],
                )

                run.return_value.stdout = payload.replace(b"v1alpha3", b"stale")
                with self.assertRaisesRegex(RuntimeError, "Unexpected context protocol"):
                    tool.get_context()

                run.return_value.stdout = b"x" * 4097
                with self.assertRaisesRegex(RuntimeError, "payload bound"):
                    tool.get_context()


if __name__ == "__main__":
    unittest.main()
