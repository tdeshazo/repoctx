"""Focused failure-path checks for the paired trial runner."""

import importlib.util
import hashlib
import json
from pathlib import Path
import sys
import unittest

SPEC = importlib.util.spec_from_file_location("paired_trial", Path(__file__).resolve().parents[1] / "scripts/paired_trial.py")
RUNNER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUNNER)
SCORE_SPEC = importlib.util.spec_from_file_location("score_paired_trial", Path(__file__).resolve().parents[1] / "scripts/score_paired_trial.py")
SCORER = importlib.util.module_from_spec(SCORE_SPEC)
SCORE_SPEC.loader.exec_module(SCORER)


class PairedTrialTests(unittest.TestCase):
    def test_final_output_bound_includes_escaping_and_unicode(self):
        result = {"exit_code": 0, "stdout": 'é\\"\n' * 5000, "stderr": "warning" * 5000}
        bounded = RUNNER.bounded_output(result, 800)
        self.assertLessEqual(len(RUNNER.dumps(bounded).encode("utf-8")), 800)
        self.assertTrue(bounded["truncated"])
        self.assertTrue(result["stdout"].startswith(bounded["stdout"]))
        self.assertTrue(result["stderr"].startswith(bounded["stderr"]))

    def test_timeout_retains_serializable_diagnostics(self):
        result = RUNNER.execute([sys.executable, "-c", "import time; print('partial', flush=True); time.sleep(10)"], 0.2)
        self.assertEqual(result["exit_code"], 124)
        self.assertIn("partial", result["stdout"])
        self.assertIn("timed out", result["stderr"])
        json.dumps(result)

    def test_coverage_requires_complete_source_verified_spans(self):
        source = b"abcdef"
        task = {"answer_spans": [{"file": "fixture", "start_byte": 1, "end_byte": 5}],
                "source_map": {"fixture": "source.txt"}}
        def evidence(start, end):
            return {"file": "source.txt", "start_byte": start, "end_byte": end,
                    "text": source[start:end].decode(), "sha256": hashlib.sha256(source).hexdigest()}
        bundle = {"evidence": [evidence(1, 3), evidence(3, 5)]}
        self.assertEqual(SCORER.coverage(bundle, task, {"source.txt": source}), 1)
        bundle["evidence"][1] = evidence(4, 5)
        self.assertEqual(SCORER.coverage(bundle, task, {"source.txt": source}), 0)
        bundle["evidence"][0]["text"] = "fabricated"
        with self.assertRaises(AssertionError):
            SCORER.coverage(bundle, task, {"source.txt": source})


if __name__ == "__main__":
    unittest.main()
