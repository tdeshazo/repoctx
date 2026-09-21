"""Paired persisted-evidence evaluation contract tests."""

import copy
import importlib.util
from pathlib import Path
import unittest
from unittest import mock


PROJECT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "m4_persistence", PROJECT / "scripts/m4_persistence.py"
)
PERSISTENCE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PERSISTENCE)


def trial(task_id, pair_id, condition, scenario):
    config = PERSISTENCE.validate_config()
    tasks = PERSISTENCE.task_identities()
    controls = {
        "task_sha256": tasks[task_id],
        "permission_profile": "matched_repository",
        **PERSISTENCE.control_identities(config),
    }
    reference_error = {
        "source_mutation": "stale",
        "missing_handle": "missing",
        "expired_handle": "expired",
    }.get(scenario, "none")
    if condition == "inline":
        reference_error = "not_applicable"
        initial = {"delivery_bytes": 1000, "bundle_bytes": 1000,
                   "reference_bytes": 0, "preview_bytes": 0}
        reads = [{"sequence": 1, "operation": "context", "status": "success",
                  "requested_bytes": 1000, "source_bytes": 1000,
                  "response_bytes": 1000, "repeated_evidence_bytes": 100,
                  "wall_ms": 3}]
        storage = {"peak_bytes": 0, "retained_ms": 0, "byte_milliseconds": 0}
        input_tokens = 1000
    else:
        initial = {"delivery_bytes": 200, "bundle_bytes": 1000,
                   "reference_bytes": 200, "preview_bytes": 0}
        status = "success" if scenario == "recovery" else reference_error
        reads = [{"sequence": 1, "operation": "read_context", "status": status,
                  "requested_bytes": 100,
                  "source_bytes": 100 if status == "success" else 0,
                  "response_bytes": 200 if status == "success" else 50,
                  "repeated_evidence_bytes": 0, "wall_ms": 2}]
        storage = {"peak_bytes": 1000, "retained_ms": 100,
                   "byte_milliseconds": 100000}
        input_tokens = 700
    return {
        "version": "repoctx.m4.persistence-trial/v1alpha1",
        "run_lock_sha256": "a" * 64,
        "pair_id": pair_id,
        "task_id": task_id,
        "condition": condition,
        "scenario": scenario,
        "controls": controls,
        "compaction": {"performed": True, "prior_evidence_removed": True,
                       "decisive_detail_present": False,
                       "checkpoint_bytes": 300},
        "pre_resume": {
            "source_mutated": scenario == "source_mutation",
            "handle_state": ("not_applicable" if condition == "inline" else
                             {"missing_handle": "missing", "expired_handle": "expired"}.get(
                                 scenario, "available"
                             )),
        },
        "initial_delivery": initial,
        "read_trace_complete": True,
        "reads": reads,
        "outcome": {
            "verified_task_success": True,
            "detail_recovered": scenario == "recovery",
            "stale_evidence_rejected": condition == "file_reference" and
                                       scenario == "source_mutation",
            "reference_error": reference_error,
            "evidence_used_after_rejection": False,
        },
        "provider_usage": {
            "raw": {"provider_total": input_tokens + 50},
            "normalized": {"input_tokens": input_tokens, "output_tokens": 50,
                           "cached_input_tokens": 100, "reasoning_tokens": 10},
        },
        "tool_calls": 1,
        "wall_ms": 20,
        "storage": storage,
    }


def complete_records():
    config = PERSISTENCE.validate_config()
    schedule = PERSISTENCE.scenario_schedule(config)
    records = []
    for index, task_id in enumerate(sorted(schedule)):
        scenario = schedule[task_id]
        pair_id = f"pair-{index + 1:02d}"
        for condition in PERSISTENCE.CONDITIONS:
            records.append(trial(task_id, pair_id, condition, scenario))
    return records


class M4PersistenceTests(unittest.TestCase):
    def test_complete_report_accounts_for_recovery_reads_and_whole_task_costs(self):
        report = PERSISTENCE.build_report(complete_records())
        self.assertEqual(report["paired_tasks"], 33)
        self.assertEqual(report["quality_paired_tasks"], 30)
        self.assertEqual(report["claim_status"], "experimental_unassessed")
        self.assertEqual(report["resilience"]["source_mutation"]["expected_rejections"], 1)
        self.assertEqual(report["resilience"]["missing_handle"]["expected_rejections"], 1)
        self.assertEqual(report["resilience"]["expired_handle"]["expected_rejections"], 1)
        persisted = report["conditions"]["file_reference"]
        self.assertGreater(persisted["reference_bytes"], 0)
        self.assertEqual(persisted["preview_bytes"], 0)
        self.assertEqual(persisted["reads"]["calls"], 33)
        self.assertGreater(persisted["storage"]["byte_milliseconds"], 0)
        self.assertEqual(
            persisted["provider_usage"]["input_plus_output_tokens"]["value"],
            33 * 750,
        )

    def test_pairs_require_identical_controls_and_both_conditions(self):
        records = complete_records()
        records[1]["controls"]["budgets_sha256"] = "f" * 64
        with self.assertRaisesRegex(ValueError, "drifted"):
            PERSISTENCE.build_report(records)

        records = complete_records()[1:]
        with self.assertRaisesRegex(ValueError, "incomplete"):
            PERSISTENCE.build_report(records)

    def test_sample_and_scenario_requirements_are_enforced(self):
        with self.assertRaisesRegex(ValueError, "underpowered"):
            PERSISTENCE.build_report(complete_records()[:20])

        records = complete_records()
        edge = next(item for item in records if item["scenario"] == "source_mutation")
        for record in records:
            if record["pair_id"] == edge["pair_id"]:
                record["scenario"] = "recovery"
        with self.assertRaisesRegex(ValueError, "scenario"):
            PERSISTENCE.build_report(records)

    def test_incomplete_reads_and_inconsistent_inline_storage_are_rejected(self):
        records = complete_records()
        records[1]["read_trace_complete"] = False
        with self.assertRaisesRegex(ValueError, "read trace"):
            PERSISTENCE.build_report(records)

        records = complete_records()
        inline = next(item for item in records if item["condition"] == "inline")
        inline["storage"]["peak_bytes"] = 1
        with self.assertRaisesRegex(ValueError, "inline trial"):
            PERSISTENCE.build_report(records)

    def test_task_and_compaction_identities_are_frozen(self):
        records = complete_records()
        records[0]["controls"]["task_sha256"] = "0" * 64
        with self.assertRaisesRegex(ValueError, "task identity"):
            PERSISTENCE.build_report(records)

        records = complete_records()
        records[0]["controls"]["compaction_policy_sha256"] = "0" * 64
        with self.assertRaisesRegex(ValueError, "compaction_policy_sha256 drifted"):
            PERSISTENCE.build_report(records)

        records = complete_records()
        records[0]["run_lock_sha256"] = "b" * 64
        with self.assertRaisesRegex(ValueError, "mix run locks"):
            PERSISTENCE.build_report(records)

    def test_unreported_model_usage_remains_missing(self):
        records = complete_records()
        for record in records:
            record["provider_usage"]["normalized"] = {
                field: None for field in PERSISTENCE.USAGE_FIELDS
            }
        report = PERSISTENCE.build_report(records)
        usage = report["conditions"]["file_reference"]["provider_usage"]
        self.assertIsNone(usage["input_plus_output_tokens"]["value"])
        self.assertEqual(usage["input_plus_output_tokens"]["missing"], 33)

    def test_config_rejects_checkpoint_leakage(self):
        config = copy.deepcopy(PERSISTENCE.validate_config())
        config["compaction_policy"]["decisive_detail_in_checkpoint"] = True
        with self.assertRaisesRegex(ValueError, "contract drifted"):
            with mock.patch.object(PERSISTENCE, "load_json", return_value=config):
                PERSISTENCE.validate_config(Path("ignored"))


if __name__ == "__main__":
    unittest.main()
