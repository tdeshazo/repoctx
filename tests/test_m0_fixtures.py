"""Regression checks for corpus integrity and evaluator/agent separation."""

import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("check_fixtures", ROOT / "scripts/check_fixtures.py")
CHECKER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKER)


class FixtureTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "corpus"
        shutil.copytree(ROOT / "evals/m0", self.root)

    def mutate(self, path, edit):
        target = self.root / path
        value = json.loads(target.read_text())
        edit(value)
        target.write_text(json.dumps(value), encoding="utf-8")

    def assert_invalid(self, phrase):
        result, _ = CHECKER.validate(self.root)
        self.assertFalse(result["ok"])
        self.assertIn(phrase, "\n".join(result["errors"]))

    def test_corpus_and_every_export(self):
        result, records = CHECKER.validate(self.root)
        self.assertTrue(result["ok"], result["errors"])
        self.assertEqual(result["distinct"], 12)
        self.assertEqual(result["counts"], CHECKER.EXPECTED_COUNTS)
        for fixture_id, (task, _, agent) in records.items():
            with self.subTest(fixture=fixture_id):
                output = Path(self.temp.name) / fixture_id
                CHECKER.export(self.root, records, fixture_id, output)
                files = sorted(path.relative_to(output).as_posix()
                               for path in output.rglob("*") if path.is_file())
                self.assertEqual(files, sorted(["input.json"] +
                                              ["repository/" + path for path in agent["files"]]))
                for source, path in task["source_map"].items():
                    if CHECKER.permitted(task, path):
                        self.assertEqual((output / "repository" / path).read_bytes(),
                                         (self.root / source).read_bytes())
                if fixture_id == "no-answer-restricted":
                    self.assertNotIn(b"restricted-fixture-marker",
                                     b"".join((output / file).read_bytes() for file in files))
                with self.assertRaises(FileExistsError):
                    CHECKER.export(self.root, records, fixture_id, output)

    def test_expected_span_drift_fails(self):
        self.mutate("expected/code-retry-policy.json", lambda value: value.update(spans=["missing"]))
        self.assert_invalid("expected spans disagree")

    def test_expected_relationship_drift_fails(self):
        self.mutate("expected/code-auth-boundary.json", lambda value: value.update(relationships=[]))
        self.assert_invalid("relationships disagree")

    def test_ungrounded_relationship_fails(self):
        def change(value):
            key = "expected_relationships" if "expected_relationships" in value else "relationships"
            value[key][0]["evidence"] = ["missing"]
        self.mutate("tasks/code-auth-boundary.json", change)
        self.mutate("expected/code-auth-boundary.json", change)
        self.assert_invalid("lacks supporting spans")

    def test_duplicate_trial_does_not_increase_count(self):
        self.mutate("manifest.json", lambda value: value["fixtures"].append(value["fixtures"][0]))
        self.assert_invalid("duplicate")
        result, _ = CHECKER.validate(self.root)
        self.assertEqual(result["distinct"], 12)

    def test_renamed_duplicate_is_not_a_new_task(self):
        manifest = json.loads((self.root / "manifest.json").read_text())
        item = copy.deepcopy(manifest["fixtures"][0])
        item["id"] = "repeated-trial"
        for key in ("task", "expected", "agent_input"):
            value = json.loads((self.root / item[key]).read_text())
            value["id"] = item["id"]
            item[key] = key + "-duplicate.json"
            (self.root / item[key]).write_text(json.dumps(value))
        self.mutate("manifest.json", lambda value: value["fixtures"].append(item))
        self.assert_invalid("repeated task/source pair")

    def test_metadata_leak_fails(self):
        self.mutate("agent_inputs/code-retry-policy.json", lambda value: value.update(answer="3"))
        self.assert_invalid("evaluator metadata")

    def test_invalid_budget_fails(self):
        self.mutate("tasks/code-retry-policy.json", lambda value: value["budgets"].update(max_bytes=-1))
        self.assert_invalid("must be positive")

    def test_empty_scoring_rule_fails(self):
        self.mutate("scoring.json", lambda value: value["rules"].update(scope=""))
        self.assert_invalid("nonempty rubrics")

    def test_path_escape_fails(self):
        self.mutate("tasks/code-retry-policy.json",
                    lambda value: value["source_map"].update({value["source_files"][0]: "../outside.go"}))
        self.assert_invalid("cannot escape")

    def test_symlink_source_fails(self):
        source = self.root / "sources/code-retry-policy.go.txt"
        source.unlink()
        source.symlink_to(ROOT / "evals/m0/sources/code-retry-policy.go.txt")
        self.assert_invalid("symlinks")

    def test_denied_answer_span_fails(self):
        self.mutate("tasks/code-retry-policy.json", lambda value: value.update(denied_scope=["src"]))
        self.assert_invalid("permitted source mapping")

    def test_byte_offsets_preserve_utf8_and_crlf(self):
        source = "sources/doc-api-contract.md"
        prefix = "# Café\r\n\r\n".encode("utf-8")
        text = "Compile preserves café bytes.\r\nNo execution."
        (self.root / source).write_bytes(prefix + text.encode("utf-8") + b"\r\n")
        def change(value):
            value["answer_spans"][0].update(start_byte=len(prefix),
                                           end_byte=len(prefix) + len(text.encode("utf-8")), text=text)
        self.mutate("tasks/doc-api-contract.json", change)
        result, _ = CHECKER.validate(self.root)
        self.assertTrue(result["ok"], result["errors"])
        self.mutate("tasks/doc-api-contract.json",
                    lambda value: value["answer_spans"][0].update(text=text.replace("\r\n", "\n")))
        self.assert_invalid("bytes/text disagree")

    def test_malformed_records_return_diagnostics(self):
        (self.root / "tasks/code-retry-policy.json").write_text("[]")
        self.assert_invalid("expected JSON object")


if __name__ == "__main__":
    unittest.main()
