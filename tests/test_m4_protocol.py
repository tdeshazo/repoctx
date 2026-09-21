"""Frozen M4 experiment-control tests."""

import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest import mock


PROJECT = Path(__file__).resolve().parents[1]


def load(name, relative):
    spec = importlib.util.spec_from_file_location(name, PROJECT / relative)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


PROTOCOL = load("m4_protocol", "scripts/m4_protocol.py")
FIXTURES = load("check_m4_tasks_for_protocol_tests", "scripts/check_m4_tasks.py")
CONDITIONS = load("m4_conditions_for_protocol_tests", "scripts/m4_conditions.py")


class M4ProtocolTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "m4"
        shutil.copytree(PROJECT / "evals/m4", self.root)
        shutil.copytree(PROJECT / "evals/m0", Path(self.temp.name) / "m0")

    def validate(self):
        return PROTOCOL.validate_protocol(self.root / "protocol.json")

    def mutate(self, callback):
        path = self.root / "protocol.json"
        value = json.loads(path.read_text(encoding="utf-8"))
        callback(value)
        path.write_text(json.dumps(value), encoding="utf-8")

    def test_protocol_pins_profiles_budgets_and_scoring(self):
        protocol, scoring = self.validate()
        self.assertEqual(protocol["compiler_profile"]["ir_contract"], "repoctx.ir/v1alpha4")
        self.assertEqual(protocol["context_profile"]["contract"], "repoctx.context/v1alpha3")
        self.assertEqual(protocol["budgets"]["condition_output_bytes"], 12000)
        self.assertEqual(set(scoring["profiles"]), {"end_to_end", "evidence_only"})

    def test_matched_permissions_and_no_tools_are_enforced(self):
        protocol, _ = self.validate()
        self.assertEqual(protocol["permissions"]["matched_repository"]["tools"],
                         ["repository_shell"])
        self.assertEqual(protocol["permissions"]["evidence_only"]["tools"], [])
        self.mutate(lambda value: value["permissions"]["evidence_only"]["tools"].append("read"))
        with self.assertRaisesRegex(ValueError, "no tools"):
            self.validate()

    def test_schedule_is_seeded_interleaved_and_uses_fresh_workspaces(self):
        protocol, _ = self.validate()
        validation, records = FIXTURES.validate(self.root)
        self.assertTrue(validation["ok"], validation["errors"])
        _, conditions = CONDITIONS.load_conditions(self.root / "conditions.json")
        first = PROTOCOL.build_schedule(protocol, records, conditions)
        second = PROTOCOL.build_schedule(protocol, records, conditions)
        self.assertEqual(first, second)
        self.assertEqual(len({trial["workspace_id"] for trial in first}), len(first))
        self.assertTrue(all(trial["fresh_workspace"] for trial in first))
        self.assertGreater(len({(trial["track"], trial["condition"])
                                for trial in first[:20]}), 2)
        permissions = {trial["permission_profile"] for trial in first
                       if trial["track"] == "end_to_end"}
        self.assertEqual(permissions, {"matched_repository"})
        self.assertTrue(all(trial["permission_profile"] == "evidence_only"
                            for trial in first if trial["track"] == "evidence_only"))
        change_workspaces = [trial["workspace_id"] for trial in first
                             if records[trial["task"]]["task"]["type"] == "change"]
        self.assertEqual(len(change_workspaces), len(set(change_workspaces)))

    def test_cache_state_and_artifact_eligibility_are_explicit(self):
        protocol, _ = self.validate()
        _, records = FIXTURES.validate(self.root)
        _, conditions = CONDITIONS.load_conditions(self.root / "conditions.json")
        schedule = PROTOCOL.build_schedule(protocol, records, conditions)
        self.assertEqual({trial["cache_state"] for trial in schedule}, {"cold", "warm"})
        artifact_tasks = {trial["task"] for trial in schedule
                          if trial["condition"] == "artifact_aware"}
        self.assertTrue(artifact_tasks)
        self.assertEqual({records[task]["task"]["repository"] for task in artifact_tasks},
                         {"ledger-lite"})
        self.assertTrue(all(trial["cache_preparation"] == "not_applicable"
                            for trial in schedule if trial["condition"] == "ordinary_tools"))
        self.assertTrue(all(trial["cache_trace_required"] ==
                            ["requested", "supplier_cache_before", "supplier_cache_after", "os_cache"]
                            for trial in schedule))

    def test_run_lock_pins_source_model_binary_and_harness(self):
        build = {"program": "repoctx", "release": "test", "revision": "a" * 40,
                 "modified": "false",
                 "contracts": {"ir": "repoctx.ir/v1alpha4",
                               "context": "repoctx.context/v1alpha3"}}
        with mock.patch.object(PROTOCOL, "git_state", return_value=("a" * 40, False)), \
                mock.patch.object(PROTOCOL, "binary_identity", return_value=("b" * 64, build)):
            lock = PROTOCOL.create_lock("model-name", "model-build-2026-09-21", Path("ignored"))
        self.assertEqual(lock["source"]["revision"], "a" * 40)
        self.assertTrue(lock["source"]["clean"])
        self.assertEqual(lock["repoctx"]["binary_sha256"], "b" * 64)
        self.assertEqual(lock["model"]["revision"], "model-build-2026-09-21")
        self.assertEqual(lock["harness"]["protocol_sha256"], PROTOCOL.digest(PROTOCOL.PROTOCOL))
        self.assertEqual(lock["harness"]["accounting_sha256"],
                         PROTOCOL.digest(PROTOCOL.ACCOUNTING))
        self.assertEqual(lock["harness"]["decision_rules_sha256"],
                         PROTOCOL.digest(PROTOCOL.DECISIONS))
        self.assertEqual(lock["harness"]["persistence_contract_sha256"],
                         PROTOCOL.digest(PROTOCOL.PERSISTENCE))
        self.assertEqual(lock["harness"]["failure_contract_sha256"],
                         PROTOCOL.digest(PROTOCOL.FAILURES))
        self.assertEqual(lock["harness"]["failure_cases_sha256"],
                         PROTOCOL.digest(PROTOCOL.FAILURE_CASES))
        self.assertIn("evals/m4/artifacts/ledger-lite.json", lock["source"]["input_sha256"])
        self.assertIn("scripts/m4_accounting.py", lock["source"]["input_sha256"])
        self.assertIn("scripts/m4_decisions.py", lock["source"]["input_sha256"])
        self.assertIn("scripts/m4_persistence.py", lock["source"]["input_sha256"])
        self.assertIn("evals/m4/persistence.json", lock["source"]["input_sha256"])
        self.assertIn("evals/m4/failures.json", lock["source"]["input_sha256"])
        self.assertIn("evals/m4/failure-cases.json", lock["source"]["input_sha256"])
        self.assertIn("scripts/m4_failures.py", lock["source"]["input_sha256"])
        self.assertTrue(lock["schedule"])

    def test_protocol_rejects_condition_budget_drift(self):
        self.mutate(lambda value: value["budgets"].update(condition_output_bytes=11999))
        with self.assertRaisesRegex(ValueError, "budgets disagree"):
            self.validate()

    def test_protocol_rejects_accounting_contract_drift(self):
        path = self.root / "accounting.json"
        value = json.loads(path.read_text(encoding="utf-8"))
        value["additive_usage"].append("cached_input_tokens")
        path.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "accounting contract drifted"):
            self.validate()

    def test_protocol_rejects_decision_rule_drift(self):
        path = self.root / "decisions.json"
        value = json.loads(path.read_text(encoding="utf-8"))
        value["practical_thresholds"]["tool_calls_relative_reduction"] = 0
        path.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "practical thresholds drifted"):
            self.validate()


if __name__ == "__main__":
    unittest.main()
