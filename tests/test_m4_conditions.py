"""Comparison-condition labeling, isolation, and payload tests."""

import hashlib
import importlib.util
import json
import os
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


CONDITIONS = load("m4_conditions", "scripts/m4_conditions.py")
FIXTURES = load("check_m4_tasks_for_conditions", "scripts/check_m4_tasks.py")


class M4ConditionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "m4"
        shutil.copytree(PROJECT / "evals/m4", self.root)
        shutil.copytree(PROJECT / "evals/m0", Path(self.temp.name) / "m0")
        validation, self.records = FIXTURES.validate(self.root)
        self.assertTrue(validation["ok"], validation["errors"])
        self.config, self.conditions = CONDITIONS.load_conditions(self.root / "conditions.json")
        self.export_count = 0

    def export(self, task_id):
        self.export_count += 1
        output = Path(self.temp.name) / "exports" / f"{self.export_count}-{task_id}"
        FIXTURES.export(self.records, task_id, output)
        return output / "repository"

    def render(self, task_id, condition, **kwargs):
        return CONDITIONS.render(self.records[task_id], self.conditions[condition],
                                 self.export(task_id), self.config["max_output_bytes"],
                                 config_root=self.root, **kwargs)

    def assert_bounded(self, value):
        self.assertLessEqual(len(CONDITIONS.compact(value)), self.config["max_output_bytes"])
        self.assertEqual(value["usage"]["bytes"], len(CONDITIONS.compact(value)))

    def test_registry_has_distinct_attribution(self):
        self.assertEqual(set(self.conditions), set(CONDITIONS.EXPECTED))
        self.assertFalse(self.conditions["ordinary_tools"]["repoctx"])
        self.assertEqual(self.conditions["repoctx_no_graph"]["graph_depth"], 0)
        self.assertEqual(self.conditions["repoctx_graph"]["graph_depth"], 1)
        self.assertEqual(self.conditions["artifact_aware"]["curation"], "repository_declarations")
        self.assertFalse(self.conditions["artifact_aware"]["repoctx"])
        self.assertEqual(self.conditions["human_oracle"]["curation"], "human_gold")
        self.assertFalse(self.conditions["human_oracle"]["repoctx"])

    def test_mislabeled_manual_condition_fails(self):
        path = self.root / "conditions.json"
        value = json.loads(path.read_text(encoding="utf-8"))
        oracle = next(item for item in value["conditions"] if item["id"] == "human_oracle")
        oracle["repoctx"] = True
        path.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "mislabeled"):
            CONDITIONS.load_conditions(path)

    def test_ordinary_lexical_artifact_and_oracle_outputs(self):
        ordinary = self.render("l-doc-default-currency", "ordinary_tools")
        self.assertFalse(ordinary["payload"]["preselected_evidence"])
        self.assertNotIn("evidence", ordinary["payload"])

        lexical = self.render("l-doc-default-currency", "bounded_lexical")
        self.assertTrue(lexical["payload"]["evidence"])
        for evidence in lexical["payload"]["evidence"]:
            source = self.exported_source("l-doc-default-currency", evidence["file"])
            self.assertEqual(hashlib.sha256(source).hexdigest(), evidence["sha256"])
            self.assertEqual(source[evidence["start_byte"]:evidence["end_byte"]].decode(), evidence["text"])

        artifact = self.render("l-change-late-cap", "artifact_aware")
        self.assertIn("ledger:component.pricing", artifact["payload"]["selected_artifacts"])
        self.assertEqual(artifact["condition"]["curation"], "repository_declarations")
        self.assertNotIn("answer", json.dumps(artifact))

        oracle = self.render("l-doc-default-currency", "human_oracle")
        self.assertEqual(oracle["condition"]["curation"], "human_gold")
        self.assertTrue(oracle["payload"]["evidence"])
        self.assertNotIn("answer", json.dumps(oracle))
        for value in (ordinary, lexical, artifact, oracle):
            self.assert_bounded(value)

    def exported_source(self, task_id, path):
        # Each call uses a new export because condition inputs must not share
        # mutable workspaces. Source bytes remain identical by fixture contract.
        return self.export(task_id).joinpath(path).read_bytes()

    def test_scope_and_artifact_eligibility_remain_explicit(self):
        denied = self.render("edge-denied-admin-token", "human_oracle")
        self.assertEqual(denied["payload"]["evidence"], [])
        self.assertTrue(denied["payload"]["scope_limited"])
        self.assertNotIn("violet-synthetic-marker", json.dumps(denied))

        unavailable = self.render("p-doc-default-lane", "artifact_aware")
        self.assertEqual(unavailable["availability"], "unavailable")
        self.assertEqual(unavailable["payload"]["reason"], "no declared catalog")

    def test_repoctx_conditions_use_explicit_graph_depths(self):
        binary = Path(self.temp.name) / "fake-repoctx"
        log = Path(self.temp.name) / "calls.jsonl"
        binary.write_text("""#!/usr/bin/env python3
import json, os, pathlib, sys
with open(os.environ['M4_FAKE_LOG'], 'a') as stream:
    stream.write(json.dumps(sys.argv[1:]) + '\\n')
if sys.argv[1] == 'compile':
    pathlib.Path(sys.argv[sys.argv.index('-o') + 1]).write_text('index')
else:
    print(json.dumps({'version':'repoctx.context/v1alpha3','symbols':[],
                      'units':[], 'evidence':[], 'relationships':[],
                      'snapshot':{'verified_files':0}}))
""", encoding="utf-8")
        binary.chmod(0o700)
        with mock.patch.dict(os.environ, {"M4_FAKE_LOG": str(log)}):
            no_graph = self.render("l-code-late-fee", "repoctx_no_graph", binary=binary)
            graph = self.render("l-code-late-fee", "repoctx_graph", binary=binary)
        calls = [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()]
        contexts = [call for call in calls if call[0] == "context"]
        self.assertEqual([call[call.index("-depth") + 1] for call in contexts], ["0", "1"])
        self.assertEqual(no_graph["payload"]["graph_depth"], 0)
        self.assertEqual(graph["payload"]["graph_depth"], 1)


if __name__ == "__main__":
    unittest.main()
