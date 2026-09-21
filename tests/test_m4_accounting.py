"""Complete and non-duplicative M4 accounting tests."""

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


PROJECT = Path(__file__).resolve().parents[1]


def load(name, relative):
    spec = importlib.util.spec_from_file_location(name, PROJECT / relative)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


ACCOUNTING = load("m4_accounting", "scripts/m4_accounting.py")


def stage(status="success", wall=10):
    if status == "not_applicable":
        return {"status": status, "wall_ms": None, "cpu_ms": None, "peak_rss_bytes": None,
                "input_bytes": None, "output_bytes": None}
    return {"status": status, "wall_ms": wall, "cpu_ms": wall - 1,
            "peak_rss_bytes": 1024, "input_bytes": 100, "output_bytes": 200}


def record(trial_id, cache, *, evidence_only=False, outcome="completed", usage=None):
    before = "empty" if cache == "cold" else "prepared"
    after = "populated" if cache == "cold" else "reused"
    tools = [] if evidence_only else [{"sequence": 1, "tool": "repository_shell",
                                       "status": "success", "wall_ms": 4,
                                       "input_bytes": 20, "output_bytes": 40}]
    return {
        "version": "repoctx.m4.trial-accounting/v1alpha1",
        "trial_id": trial_id,
        "wall_ms": 50,
        "cache": {"requested": cache, "supplier_cache_before": before,
                  "supplier_cache_after": after, "os_cache": "unknown"},
        "outcome": outcome,
        "stages": {"compilation": stage(), "update": stage("not_applicable"),
                   "retrieval": stage(), "agent": stage(), "verification": stage()},
        "tool_trace_complete": True,
        "tool_calls": tools,
        "provider_usage": {"raw": {"provider_total": 125},
                           "normalized": usage or {"input_tokens": 100,
                                                   "output_tokens": 25,
                                                   "cached_input_tokens": 40,
                                                   "reasoning_tokens": 10}},
    }


class M4AccountingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.lock_path = Path(self.temp.name) / "run-lock.json"
        self.lock = {
            "version": "repoctx.m4.run-lock/v1alpha1",
            "schedule": [
                {"id": "trial-0001", "track": "end_to_end", "condition": "repoctx_graph",
                 "cache_state": "cold", "permission_profile": "matched_repository"},
                {"id": "trial-0002", "track": "evidence_only", "condition": "repoctx_graph",
                 "cache_state": "warm", "permission_profile": "evidence_only"},
            ],
        }
        self.lock_path.write_text(json.dumps(self.lock), encoding="utf-8")
        self.records = [record("trial-0001", "cold"),
                        record("trial-0002", "warm", evidence_only=True)]

    def test_contract_and_complete_report(self):
        config = ACCOUNTING.validate_config()
        self.assertEqual(config["additive_usage"], ["input_tokens", "output_tokens"])
        report = ACCOUNTING.build_report(self.lock_path, self.lock, self.records)
        self.assertEqual(report["summary"]["trials"], 2)
        self.assertEqual(report["summary"]["wall_ms_sum"], 100)
        self.assertEqual(len(report["groups"]), 2)
        self.assertEqual(report["summary"]["tools"]["calls"], 1)
        self.assertEqual(report["summary"]["usage"]["input_tokens"]["value"], 200)
        self.assertEqual(report["summary"]["usage"]["output_tokens"]["value"], 50)
        self.assertEqual(report["summary"]["usage"]["input_plus_output_tokens"]["value"], 250)
        self.assertEqual(report["summary"]["usage"]["cached_input_tokens"]["value"], 80)
        self.assertEqual(report["summary"]["usage"]["reasoning_tokens"]["value"], 20)
        self.assertEqual(report["trials"][0]["provider_usage"]["raw"], {"provider_total": 125})

    def test_missing_trial_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "incomplete"):
            ACCOUNTING.build_report(self.lock_path, self.lock, self.records[:1])

    def test_subset_usage_cannot_exceed_parent(self):
        self.records[0]["provider_usage"]["normalized"]["cached_input_tokens"] = 101
        with self.assertRaisesRegex(ValueError, "subset"):
            ACCOUNTING.build_report(self.lock_path, self.lock, self.records)

    def test_evidence_only_tool_call_is_rejected(self):
        self.records[1]["tool_calls"] = self.records[0]["tool_calls"]
        with self.assertRaisesRegex(ValueError, "permission"):
            ACCOUNTING.build_report(self.lock_path, self.lock, self.records)

    def test_incomplete_tool_trace_is_rejected(self):
        self.records[0]["tool_trace_complete"] = False
        with self.assertRaisesRegex(ValueError, "tool trace must be complete"):
            ACCOUNTING.build_report(self.lock_path, self.lock, self.records)

    def test_failures_timeouts_abstentions_and_null_usage_remain_visible(self):
        first = self.records[0]
        first["outcome"] = "timeout"
        first["stages"]["agent"] = stage("timeout")
        first["provider_usage"]["normalized"] = {field: None for field in ACCOUNTING.USAGE_FIELDS}
        second = self.records[1]
        second["outcome"] = "abstained"
        report = ACCOUNTING.build_report(self.lock_path, self.lock, self.records)
        self.assertEqual(report["summary"]["outcomes"], {"abstained": 1, "timeout": 1})
        usage = report["summary"]["usage"]["input_plus_output_tokens"]
        self.assertEqual((usage["observed"], usage["missing"]), (1, 1))
        self.assertEqual(report["summary"]["stages"]["agent"]["statuses"]["timeout"], 1)

    def test_unavailable_measurements_are_not_coerced_to_zero(self):
        for value in self.records:
            value["provider_usage"]["normalized"] = {
                field: None for field in ACCOUNTING.USAGE_FIELDS
            }
        report = ACCOUNTING.build_report(self.lock_path, self.lock, self.records)
        usage = report["summary"]["usage"]
        self.assertIsNone(usage["input_tokens"]["value"])
        self.assertIsNone(usage["input_plus_output_tokens"]["value"])


if __name__ == "__main__":
    unittest.main()
