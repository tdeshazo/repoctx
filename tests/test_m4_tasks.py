"""Integrity, separation, and executable-check tests for the M4 pilot."""

import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest


PROJECT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("check_m4_tasks", PROJECT / "scripts/check_m4_tasks.py")
CHECKER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKER)


class M4TaskTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "m4"
        shutil.copytree(PROJECT / "evals/m4", self.root)
        shutil.copytree(PROJECT / "evals/m0", Path(self.temp.name) / "m0")

    def manifest(self):
        return json.loads((self.root / "manifest.json").read_text(encoding="utf-8"))

    def write_manifest(self, value):
        (self.root / "manifest.json").write_text(json.dumps(value), encoding="utf-8")

    def assert_invalid(self, phrase):
        result, _ = CHECKER.validate(self.root)
        self.assertFalse(result["ok"])
        self.assertIn(phrase, "\n".join(result["errors"]))

    def test_corpus_and_every_export(self):
        result, records = CHECKER.validate(self.root)
        self.assertTrue(result["ok"], result["errors"])
        self.assertEqual(result["tasks"], 33)
        self.assertEqual(result["answerable_by_repository"],
                         {"ledger-lite": 10, "parcel-path": 10, "portal-kit": 10})
        self.assertEqual(result["types"], {"documentation_lookup": 9,
                                           "code_localization": 9,
                                           "cross_file_analysis": 6,
                                           "change": 6, "edge_case": 3})
        for task_id, record in records.items():
            with self.subTest(task=task_id):
                output = Path(self.temp.name) / "exports" / task_id
                CHECKER.export(records, task_id, output)
                agent = json.loads((output / "input.json").read_text(encoding="utf-8"))
                self.assertNotIn("answer", agent)
                self.assertNotIn("evidence", agent)
                self.assertNotIn("outcome", agent)
                files = sorted(path.relative_to(output / "repository").as_posix()
                               for path in (output / "repository").rglob("*") if path.is_file())
                self.assertEqual(files, record["visible"])
                with self.assertRaises(FileExistsError):
                    CHECKER.export(records, task_id, output)
        denied = Path(self.temp.name) / "exports" / "edge-denied-admin-token" / "repository"
        self.assertNotIn(b"violet-synthetic-marker",
                         b"".join(path.read_bytes() for path in denied.rglob("*") if path.is_file()))

    def test_export_failure_does_not_publish_partial_trial(self):
        result, records = CHECKER.validate(self.root)
        self.assertTrue(result["ok"], result["errors"])
        record = records["l-doc-default-currency"]
        (record["repository"]["root"] / record["visible"][0]).unlink()
        output = Path(self.temp.name) / "partial"
        with self.assertRaises(FileNotFoundError):
            CHECKER.export(records, "l-doc-default-currency", output)
        self.assertFalse(output.exists())

    def test_change_checks_fail_baseline_and_pass_requested_edit(self):
        result, records = CHECKER.validate(self.root)
        self.assertTrue(result["ok"], result["errors"])
        edits = {
            "l-change-late-cap": ("ledger/pricing.py", ", 10)", ", 12)"),
            "l-change-uppercase-reference": ("ledger/pricing.py", ".lower()", ".upper()"),
            "p-change-weekend-surcharge": ("parcel/config.py", "= 5", "= 7"),
            "p-change-label-length": ("parcel/labels.py", "[:20]", "[:24]"),
            "w-change-breadcrumb-separator": ("portal/navigation.py", '" / "', '" › "'),
            "w-change-default-theme": ("portal/config.py", '"light"', '"system"'),
        }
        for task_id, (relative, old, new) in edits.items():
            with self.subTest(task=task_id):
                output = Path(self.temp.name) / "checks" / task_id
                CHECKER.export(records, task_id, output)
                repository = output / "repository"
                passed, _ = CHECKER.run_change_check(records[task_id], repository)
                self.assertFalse(passed, "baseline unexpectedly satisfies held-out change")
                target = repository / relative
                content = target.read_text(encoding="utf-8")
                self.assertEqual(content.count(old), 1)
                target.write_text(content.replace(old, new), encoding="utf-8")
                passed, message = CHECKER.run_change_check(records[task_id], repository)
                self.assertTrue(passed, message)

    def test_derived_evidence_changes_when_source_changes(self):
        result, records = CHECKER.validate(self.root)
        self.assertTrue(result["ok"], result["errors"])
        before = records["l-doc-default-currency"]["evidence"][0]
        readme = self.root / "repositories/ledger-lite/README.md"
        readme.write_text("prefix\n" + readme.read_text(encoding="utf-8"), encoding="utf-8")
        result, records = CHECKER.validate(self.root)
        self.assertTrue(result["ok"], result["errors"])
        after = records["l-doc-default-currency"]["evidence"][0]
        self.assertNotEqual(before["sha256"], after["sha256"])
        self.assertEqual(after["start_byte"], before["start_byte"] + len("prefix\n"))

    def test_invalid_metadata_and_scope_fail(self):
        manifest = self.manifest()
        manifest["tasks"][0]["answer_hint"] = "leak"
        self.write_manifest(manifest)
        self.assert_invalid("unknown task field")

        manifest = self.manifest()
        del manifest["tasks"][0]["answer_hint"]
        denied = next(task for task in manifest["tasks"] if task["id"] == "edge-denied-admin-token")
        denied["denied"] = []
        self.write_manifest(manifest)
        self.assert_invalid("access-denied evidence")

    def test_permission_and_symlink_fail(self):
        provenance = self.root / "repositories/ledger-lite/PROVENANCE.json"
        value = json.loads(provenance.read_text(encoding="utf-8"))
        value["redistribution"] = "unknown"
        provenance.write_text(json.dumps(value), encoding="utf-8")
        self.assert_invalid("permission-cleared")

        value["redistribution"] = "cleared"
        provenance.write_text(json.dumps(value), encoding="utf-8")
        source = self.root / "repositories/ledger-lite/README.md"
        source.unlink()
        source.symlink_to(PROJECT / "evals/m4/repositories/ledger-lite/README.md")
        self.assert_invalid("symlink")


if __name__ == "__main__":
    unittest.main()
