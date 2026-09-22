"""Check the M1 replay's evidence verification and adapter unit forwarding."""

import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
import m1_retrieval as replay
sys.path.pop(0)

SPEC = importlib.util.spec_from_file_location("m1_bridge", ROOT / "examples/agent/tool_bridge.py")
BRIDGE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BRIDGE)


class M1ReplayTests(unittest.TestCase):
    def test_markdown_checks_actual_fenced_evidence(self):
        raw = b"alpha\r\nbeta"
        bundle = {"evidence": [{"id": "e:test", "file": "a.txt", "start_byte": 0, "end_byte": len(raw)}]}
        text = "````json\n" + json.dumps(bundle) + "\n````\n\n## Evidence e:test\n\n```\nalpha\r\nbeta\n```\n"
        parsed = replay.markdown_bundle(text, {"a.txt": raw})
        self.assertEqual(parsed["evidence"][0]["text"], raw.decode())
        with self.assertRaises(ValueError):
            replay.markdown_bundle(text.replace("beta\n```", "fabricated\n```"), {"a.txt": raw})

    def test_adapter_forwards_units_and_preserves_payload(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)
            binary, index = path / "binary", path / "index"
            binary.touch()
            index.touch()
            tool = BRIDGE.RepositoryContextTool(str(binary), directory, str(index))
            payload = b'{"version":"repoctx.context/v1alpha3","trust":{"role":"untrusted_repository_data"}}\n'
            with patch.object(BRIDGE.subprocess, "run") as run:
                run.return_value.returncode = 0
                run.return_value.stdout = payload
                self.assertEqual(tool.get_context(unit_ids=["u:sample"]), payload.decode())
                self.assertIn("-unit", run.call_args.args[0])
                self.assertIn("u:sample", run.call_args.args[0])
                run.return_value.stdout = payload.replace(b"v1alpha3", b"v1alpha2")
                with self.assertRaisesRegex(RuntimeError, "Unexpected context protocol"):
                    tool.get_context()
            with self.assertRaises(ValueError):
                tool.get_context(unit_ids="not-a-sequence-of-ids")


if __name__ == "__main__":
    unittest.main()
