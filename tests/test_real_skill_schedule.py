"""Tests for the frozen real-repository skill comparison schedule."""

import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock


PROJECT = Path(__file__).resolve().parents[1]
SCRIPT = PROJECT / "evals/real-skill-comparison/prepare_schedule.py"
REVISION = "a" * 40
SPEC = importlib.util.spec_from_file_location("real_skill_schedule", SCRIPT)
SCHEDULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCHEDULE)
PROTOCOL = SCHEDULE.load_object(SCHEDULE.PROTOCOL_PATH, "protocol")


def digest(value):
    return hashlib.sha256(value.encode()).hexdigest()


def task_manifest():
    records = []
    for index in range(30):
        if index < 5:
            family = "known_file"
        elif index < 25:
            family = "navigation_or_relationship"
        else:
            family = "change"
        records.append({
            "id": f"task-{index:03d}",
            "family": family,
            "repository_revision": REVISION,
            "repository_tree_sha256": digest(f"tree-{index}"),
            "task_sha256": digest(f"prompt-{index}"),
            "grader_sha256": digest(f"grader-{index}"),
        })
    return {"version": SCHEDULE.TASK_VERSION, "tasks": records}


def run_lock():
    return {
        "version": SCHEDULE.LOCK_VERSION,
        "model": {
            "provider": "test-provider",
            "id": "test-model",
            "revision": "test-model-build-2026-09-22-1",
            "reasoning_effort": "high",
        },
        "repoctx": {
            "source_revision": "b" * 40,
            "binary_sha256": "c" * 64,
            "version_output_sha256": "d" * 64,
        },
        "skill": {"source_revision": "e" * 40, "snapshot_sha256": "f" * 64},
        "harness": {
            "project_revision": REVISION,
            "working_tree_clean": True,
            "generator_sha256": SCHEDULE.digest_file(SCRIPT),
            "client": "test-client",
            "client_version": "1.2.3",
        },
        "execution": {
            "permission_profile_sha256": "1" * 64,
            "tool_profile_sha256": "2" * 64,
            "budgets": {
                "max_context_tokens": 32768,
                "max_output_tokens": 8192,
                "max_tool_calls": 32,
                "max_trial_seconds": 600,
                "max_tool_output_bytes": 12000,
                "tool_timeout_seconds": 30,
            },
        },
    }


class RealSkillScheduleTests(unittest.TestCase):
    def test_schedule_is_reproducible_balanced_and_pair_local(self):
        records = SCHEDULE.validate_task_manifest(task_manifest(), PROTOCOL)
        first = SCHEDULE.build_schedule(records, PROTOCOL)
        second = SCHEDULE.build_schedule(records, PROTOCOL)

        self.assertEqual(first, second)
        self.assertEqual(len(first), 180)
        self.assertEqual(len({trial["workspace_id"] for trial in first}), 180)
        self.assertEqual(sum(trial["condition"] == "control" for trial in first), 90)
        self.assertEqual(sum(trial["condition"] == "treatment" for trial in first), 90)
        self.assertEqual(sum(first[index]["condition"] == "control"
                             for index in range(0, len(first), 2)), 45)
        self.assertEqual(sum(first[index]["condition"] == "treatment"
                             for index in range(0, len(first), 2)), 45)

        for index in range(0, len(first), 2):
            left, right = first[index:index + 2]
            self.assertEqual(left["pair_id"], right["pair_id"])
            self.assertEqual(left["task_id"], right["task_id"])
            self.assertEqual(left["repeat"], right["repeat"])
            self.assertNotEqual(left["condition"], right["condition"])
            self.assertTrue(all(not trial["prefilled_evidence"] and
                                trial["index_state_at_start"] == "empty"
                                for trial in (left, right)))

    def test_freeze_pins_inputs_and_writes_operator_artifact(self):
        with tempfile.TemporaryDirectory(prefix="repoctx-skill-schedule-test-") as directory:
            root = Path(directory)
            tasks_path = root / "tasks.json"
            lock_path = root / "run-lock.json"
            output_path = root / "frozen-schedule.json"
            tasks_path.write_text(json.dumps(task_manifest()), encoding="utf-8")
            lock_path.write_text(json.dumps(run_lock()), encoding="utf-8")

            with mock.patch.object(SCHEDULE, "git_state", return_value=(REVISION, False)):
                summary = SCHEDULE.freeze(tasks_path, lock_path, output_path)

            frozen = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(frozen["task_manifest_sha256"], SCHEDULE.digest_file(tasks_path))
            self.assertEqual(frozen["run_lock_sha256"], SCHEDULE.digest_file(lock_path))
            self.assertEqual(frozen["schedule_generator_sha256"], SCHEDULE.digest_file(SCRIPT))
            self.assertEqual(frozen["project_revision"], REVISION)
            self.assertEqual((summary["pairs"], summary["trials"]), (90, 180))

    def test_rejects_malformed_task_records_and_unpinned_model(self):
        malformed = task_manifest()
        malformed["tasks"][0]["grader_sha256"] = "not-a-digest"
        with self.assertRaisesRegex(ValueError, "grader_sha256 must be"):
            SCHEDULE.validate_task_manifest(malformed, PROTOCOL)

        malformed = task_manifest()
        malformed["answer_key"] = {"secret": "must not be in the manifest"}
        with self.assertRaisesRegex(ValueError, "only version and task identity"):
            SCHEDULE.validate_task_manifest(malformed, PROTOCOL)

        unpinned = run_lock()
        unpinned["model"]["revision"] = "latest"
        with self.assertRaisesRegex(ValueError, "model.revision must be pinned"):
            SCHEDULE.validate_run_lock(unpinned, REVISION, False)

    def test_rejects_dirty_or_mismatched_harness_identity(self):
        lock = run_lock()
        with self.assertRaisesRegex(ValueError, "working tree must be clean"):
            SCHEDULE.validate_run_lock(lock, REVISION, True)

        with self.assertRaisesRegex(ValueError, "does not pin the current project revision"):
            SCHEDULE.validate_run_lock(lock, "0" * 40, False)

        wrong_generator = copy.deepcopy(lock)
        wrong_generator["harness"]["generator_sha256"] = "9" * 64
        with self.assertRaisesRegex(ValueError, "does not pin this schedule generator"):
            SCHEDULE.validate_run_lock(wrong_generator, REVISION, False)

    def test_rejects_operator_artifacts_inside_agent_readable_tree(self):
        with tempfile.TemporaryDirectory(prefix="repoctx-skill-schedule-test-") as directory:
            root = Path(directory)
            tasks_path = root / "tasks.json"
            lock_path = root / "run-lock.json"
            tasks_path.write_text(json.dumps(task_manifest()), encoding="utf-8")
            lock_path.write_text(json.dumps(run_lock()), encoding="utf-8")

            with mock.patch.object(SCHEDULE, "git_state", return_value=(REVISION, False)):
                with self.assertRaisesRegex(ValueError, "frozen schedule must be outside"):
                    SCHEDULE.freeze(tasks_path, lock_path, PROJECT / "operator-only-test.json")
                with self.assertRaisesRegex(ValueError, "task manifest must be outside"):
                    SCHEDULE.freeze(PROJECT / "operator-only-tasks.json", lock_path,
                                    root / "schedule.json")
                with self.assertRaisesRegex(ValueError, "run lock must be outside"):
                    SCHEDULE.freeze(tasks_path, PROJECT / "operator-only-lock.json",
                                    root / "schedule.json")


if __name__ == "__main__":
    unittest.main()
