"""Trace-backed M4 context-failure diagnosis tests."""

import copy
import importlib.util
from pathlib import Path
import unittest


PROJECT = Path(__file__).resolve().parents[1]


def load_module():
    spec = importlib.util.spec_from_file_location(
        "m4_failures", PROJECT / "scripts/m4_failures.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


FAILURES = load_module()


class M4FailureTests(unittest.TestCase):
    def setUp(self):
        self.config = FAILURES.validate_config()
        self.records = FAILURES.load_cases()

    def test_historical_failures_are_ranked_from_traces(self):
        report = FAILURES.build_report(self.records, cases_path=FAILURES.CASES)
        self.assertEqual(report["records"], 3)
        self.assertEqual(report["multi_label_records"], 3)
        self.assertEqual(report["tag_counts"], {
            "buried": 3, "over_retrieved": 1, "under_retrieved": 2,
        })
        self.assertEqual(report["ranked_failures"][0]["id"],
                         "dogfood-m4-07-window-does-not-fit")
        self.assertEqual(report["ranked_failures"][0]["priority"], 133)

    def test_tags_must_be_derived_from_trace_facts(self):
        record = copy.deepcopy(self.records[0])
        record["tags"] = ["missing"]
        with self.assertRaisesRegex(ValueError, "do not match trace facts"):
            FAILURES.validate_record(record, self.config)

    def test_missing_evidence_requires_an_inventory_cause(self):
        record = copy.deepcopy(self.records[0])
        record["required_evidence"]["available"] = False
        record["trace"].update(candidate_found=False, selected=False, materialized=False,
                               retained=False, irrelevant_selected_bytes=0, omissions=[])
        record["trace"]["incomplete"] = False
        record["tags"] = ["missing"]
        record["contributors"] = {layer: [] for layer in FAILURES.LAYERS}
        record["contributors"]["selection"] = ["window_size"]
        with self.assertRaisesRegex(ValueError, "inventory contributor"):
            FAILURES.validate_record(record, self.config)
        record["contributors"]["selection"] = []
        record["contributors"]["inventory"] = ["unavailable_input"]
        FAILURES.validate_record(record, self.config)

    def test_trace_events_must_follow_causal_order(self):
        record = copy.deepcopy(self.records[0])
        record["trace"].update(candidate_found=False, selected=True)
        with self.assertRaisesRegex(ValueError, "selection requires"):
            FAILURES.validate_record(record, self.config)

    def test_duplicate_record_ids_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "duplicate failure record ID"):
            FAILURES.build_report([self.records[0], self.records[0]])


if __name__ == "__main__":
    unittest.main()
