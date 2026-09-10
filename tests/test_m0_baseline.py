from __future__ import annotations

import importlib.util
import json
import os
import sys
from pathlib import Path
from unittest import TestCase


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("m0_baseline", ROOT / "scripts" / "m0_baseline.py")
assert SPEC is not None and SPEC.loader is not None
m0_baseline = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(m0_baseline)


class BaselineRunnerTests(TestCase):
    def test_timeout_retains_decoded_output_in_serializable_failure(self) -> None:
        command = [
            sys.executable,
            "-c",
            "import sys, time; print('stdout', flush=True); print('stderr', file=sys.stderr, flush=True); time.sleep(10)",
        ]

        result = m0_baseline.run_command(command, cwd=ROOT, env=os.environ.copy(), timeout=1)

        self.assertEqual(result["exit_code"], 124)
        self.assertFalse(result["ok"])
        self.assertIsInstance(result["stdout_tail"], str)
        self.assertIsInstance(result["stderr_tail"], str)
        self.assertIn("stdout", result["stdout_tail"])
        self.assertIn("stderr", result["stderr_tail"])
        self.assertIn("command timed out", result["stderr_tail"])
        json.dumps(result)
