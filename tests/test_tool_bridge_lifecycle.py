"""Exercise checkpoint, resume, retention, and run ownership behavior."""

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "lifecycle_bridge", ROOT / "examples/agent/tool_bridge.py"
)
BRIDGE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BRIDGE)

REFERENCE_SPEC = importlib.util.spec_from_file_location(
    "reference_fixtures", ROOT / "tests/test_tool_bridge_reference.py"
)
REFERENCE = importlib.util.module_from_spec(REFERENCE_SPEC)
REFERENCE_SPEC.loader.exec_module(REFERENCE)


class CheckpointLifecycleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        temporary = Path(self.temporary.name)
        self.root = temporary / "repository"
        self.store = temporary / "caller-scope"
        self.root.mkdir()
        self.store.mkdir()
        self.binary = temporary / "repoctx"
        self.index = temporary / "repo.ir.json.gz"
        self.binary.touch()
        self.index.touch()
        self.now = [1_000]

    def tearDown(self):
        self.temporary.cleanup()

    def tool(self, run_id="run-a", *, resume_run=False, **options):
        return BRIDGE.RepositoryContextTool(
            str(self.binary),
            str(self.root),
            str(self.index),
            bundle_directory=str(self.store),
            run_id=run_id,
            resume_run=resume_run,
            clock=lambda: self.now[0],
            **options,
        )

    @staticmethod
    def retrieve(tool, payload, handle="bundle-1"):
        with patch.object(BRIDGE.subprocess, "run") as run:
            run.return_value.returncode = 0
            run.return_value.stdout = payload
            run.return_value.stderr = b""
            return json.loads(tool.get_context(delivery="file_reference", handle=handle))

    @staticmethod
    def checkpoint(tool):
        return tool.create_checkpoint(
            "Find the checkpoint lifecycle contract",
            unresolved_questions=["Which detail still needs evidence?"],
            next_retrieval={
                "query": "resume validation",
                "symbol_ids": ["s:alpha"],
                "unit_ids": ["u:roadmap"],
                "depth": 1,
            },
        )

    def test_checkpoint_resume_revalidates_snapshot_before_enabling_reads(self):
        payload = REFERENCE.context_payload()
        original = self.tool()
        self.retrieve(original, payload)
        encoded = self.checkpoint(original)
        checkpoint = json.loads(encoded)

        self.assertEqual(checkpoint["run_id"], "run-a")
        self.assertEqual(checkpoint["task"], "Find the checkpoint lifecycle contract")
        self.assertEqual(checkpoint["snapshot"]["id"], "sha256:" + "a" * 64)
        self.assertEqual(checkpoint["bundles"][0]["handle"], "bundle-1")
        self.assertEqual(checkpoint["bundles"][0]["relevant"]["unit_ids"], ["u:roadmap"])
        self.assertEqual(checkpoint["unresolved_questions"], [
            "Which detail still needs evidence?"
        ])
        self.assertEqual(checkpoint["next_retrieval"]["query"], "resume validation")
        self.assertNotIn("path", encoded)

        resumed = self.tool(resume_run=True)
        with patch.object(BRIDGE.subprocess, "run") as run:
            run.return_value.returncode = 0
            run.return_value.stdout = payload
            run.return_value.stderr = b""
            resumed.resume_checkpoint(encoded)
        arguments = run.call_args.args[0]
        self.assertIn("-expect-snapshot", arguments)
        self.assertIn("sha256:" + "a" * 64, arguments)
        self.assertEqual(json.loads(resumed.read_context("bundle-1"))["start_byte"], 0)

    def test_stale_snapshot_never_registers_checkpoint_evidence(self):
        payload = REFERENCE.context_payload()
        original = self.tool()
        self.retrieve(original, payload)
        encoded = self.checkpoint(original)
        resumed = self.tool(resume_run=True)

        with patch.object(BRIDGE.subprocess, "run") as run:
            run.return_value.returncode = 2
            run.return_value.stdout = b""
            run.return_value.stderr = b"snapshot mismatch"
            with self.assertRaises(BRIDGE.StaleEvidenceError):
                resumed.resume_checkpoint(encoded)
        with self.assertRaises(BRIDGE.MissingReferenceError):
            resumed.read_context("bundle-1")

    def test_resume_rejects_uncheckpointed_run_inventory(self):
        payload = REFERENCE.context_payload()
        original = self.tool()
        self.retrieve(original, payload)
        encoded = self.checkpoint(original)
        unexpected = self.store / "run-a" / "uncheckpointed.context.json"
        unexpected.write_bytes(b"not part of the checkpoint")
        unexpected.chmod(0o600)

        resumed = self.tool(resume_run=True)
        with self.assertRaisesRegex(BRIDGE.EvidenceLifecycleError, "inventory"):
            resumed.resume_checkpoint(encoded)
        with self.assertRaises(BRIDGE.MissingReferenceError):
            resumed.read_context("bundle-1")

    def test_missing_and_expired_references_have_explicit_errors(self):
        payload = REFERENCE.context_payload()
        original = self.tool(retention_seconds=10)
        self.retrieve(original, payload)
        encoded = self.checkpoint(original)
        bundle = self.store / "run-a" / "bundle-1.context.json"
        bundle.unlink()

        resumed = self.tool(resume_run=True, retention_seconds=10)
        with self.assertRaises(BRIDGE.MissingReferenceError):
            resumed.resume_checkpoint(encoded)

        bundle.write_bytes(payload)
        self.now[0] = 1_010
        expired = self.tool(resume_run=True, retention_seconds=10)
        with self.assertRaises(BRIDGE.ExpiredReferenceError):
            expired.resume_checkpoint(encoded)

    def test_cleanup_is_run_scoped_and_never_removes_active_bundles(self):
        payload = REFERENCE.context_payload()
        first = self.tool(retention_seconds=10)
        second = self.tool(run_id="run-b", retention_seconds=100)
        self.retrieve(first, payload, "first")
        self.retrieve(second, payload, "second")

        self.assertEqual(first.cleanup_expired(), [])
        self.assertTrue((self.store / "run-a" / "first.context.json").is_file())
        self.now[0] = 1_010
        with self.assertRaises(BRIDGE.ExpiredReferenceError):
            first.read_context("first")
        self.assertEqual(first.cleanup_expired(), ["first"])
        self.assertFalse((self.store / "run-a" / "first.context.json").exists())
        self.assertTrue((self.store / "run-b" / "second.context.json").is_file())

    def test_run_and_storage_limits_prevent_cross_owner_or_unbounded_writes(self):
        payload = REFERENCE.context_payload()
        original = self.tool(max_store_bytes=len(payload) - 1)
        with self.assertRaises(BRIDGE.StorageLimitError):
            self.retrieve(original, payload)
        self.assertEqual(list((self.store / "run-a").iterdir()), [])
        with self.assertRaisesRegex(BRIDGE.EvidenceLifecycleError, "already exists"):
            self.tool()

        bounded = self.tool(run_id="bounded", max_bundles=1)
        self.retrieve(bounded, payload, "one")
        with self.assertRaises(BRIDGE.StorageLimitError):
            self.retrieve(bounded, payload, "two")

        checkpoint_bounded = self.tool(run_id="checkpoint-bound", max_checkpoint_bytes=1024)
        self.retrieve(checkpoint_bounded, payload)
        with self.assertRaisesRegex(BRIDGE.EvidenceLifecycleError, "response budget"):
            checkpoint_bounded.create_checkpoint(
                "x" * 8_000,
                unresolved_questions=[],
                next_retrieval={"query": "", "symbol_ids": [], "unit_ids": [], "depth": 0},
            )

    def test_interrupted_publish_exposes_neither_target_nor_temporary_file(self):
        payload = REFERENCE.context_payload()
        tool = self.tool()
        with patch.object(BRIDGE.os, "write", side_effect=OSError("interrupted")):
            with self.assertRaisesRegex(OSError, "interrupted"):
                self.retrieve(tool, payload)
        self.assertEqual(list((self.store / "run-a").iterdir()), [])


if __name__ == "__main__":
    unittest.main()
