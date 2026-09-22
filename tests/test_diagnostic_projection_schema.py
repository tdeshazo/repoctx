from __future__ import annotations

import copy
import json
from pathlib import Path
from unittest import TestCase

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]


def source() -> dict[str, object]:
    return {
        "path": "docs/guide.md",
        "sha256": "a" * 64,
        "start_byte": 0,
        "end_byte": 3,
        "start_line": 1,
        "end_line": 1,
    }


def location() -> dict[str, object]:
    return {
        **source(),
        "start_byte_column": 0,
        "end_byte_column": 3,
    }


class DiagnosticSchemaTests(TestCase):
    def setUp(self):
        schema = json.loads((ROOT / "docs/diagnostics.schema.json").read_text())
        Draft202012Validator.check_schema(schema)
        self.validator = Draft202012Validator(schema)
        self.fixture = {
            "version": "repoctx.diagnostics/v1alpha1",
            "diagnostics": [{
                "code": "repoctx.invalid_input",
                "severity": "warning",
                "message": "input needs review",
                "pass": "manifest",
                "capability": "syntax_graph",
                "remediation": "fix_configuration",
                "location": location(),
            }],
        }

    def test_valid_report_and_closed_nested_shapes(self):
        self.validator.validate(self.fixture)
        for node in (self.fixture, self.fixture["diagnostics"][0], self.fixture["diagnostics"][0]["location"]):
            invalid = copy.deepcopy(self.fixture)
            target = invalid
            if node is self.fixture["diagnostics"][0]:
                target = invalid["diagnostics"][0]
            elif node is self.fixture["diagnostics"][0]["location"]:
                target = invalid["diagnostics"][0]["location"]
            target["unexpected"] = True
            self.assertFalse(self.validator.is_valid(invalid))

    def test_version_enums_and_record_ceiling(self):
        invalid = copy.deepcopy(self.fixture)
        invalid["version"] = "repoctx.diagnostics/v0"
        self.assertFalse(self.validator.is_valid(invalid))

        invalid = copy.deepcopy(self.fixture)
        invalid["diagnostics"][0]["severity"] = "critical"
        self.assertFalse(self.validator.is_valid(invalid))

        invalid = copy.deepcopy(self.fixture)
        invalid["diagnostics"] = [copy.deepcopy(self.fixture["diagnostics"][0])] * 4097
        self.assertFalse(self.validator.is_valid(invalid))


class ProjectionSchemaTests(TestCase):
    def setUp(self):
        schema = json.loads((ROOT / "docs/projection.schema.json").read_text())
        Draft202012Validator.check_schema(schema)
        self.validator = Draft202012Validator(schema)
        self.contract = {
            "version": "repoctx.projection/v1alpha1",
            "id": "caller-task",
            "imperative_core": [{"id": "goal", "kind": "requirement", "text": "preserve evidence"}],
            "task_contract": [],
            "repository_evidence": [{"id": "doc", "kind": "markdown", "text": "evidence", "source": source()}],
            "execution_affordances": [],
            "verification_obligations": [],
            "source_maps": [{"lane": "repository_evidence", "item_id": "doc", "source": source()}],
        }
        self.projection = {
            "version": "repoctx.projection/v1alpha1",
            "target": "agent",
            "escape": "none",
            "capabilities": {
                "available": ["imperative_core", "repository_evidence"],
                "unavailable": ["escaping", "execution_affordances", "source_maps", "task_contract", "verification_obligations"],
            },
            "imperative_core": self.contract["imperative_core"],
            "task_contract": [],
            "repository_evidence": self.contract["repository_evidence"],
            "execution_affordances": [],
            "verification_obligations": [],
            "source_maps": [],
            "omissions": [{"lane": "", "feature": "source_maps", "count": 1, "reason": "unsupported"}],
            "incomplete": True,
        }

    def test_valid_input_and_output_shapes(self):
        self.validator.validate(self.contract)
        self.validator.validate(self.projection)

    def test_closed_shapes_and_bounded_arrays(self):
        for name, fixture in (("contract", self.contract), ("projection", self.projection)):
            invalid = copy.deepcopy(fixture)
            invalid["unexpected"] = True
            with self.subTest(name=name):
                self.assertFalse(self.validator.is_valid(invalid))

        invalid = copy.deepcopy(self.contract)
        invalid["imperative_core"] = [
            {"id": str(i), "kind": "kind", "text": "text"} for i in range(4097)
        ]
        self.assertFalse(self.validator.is_valid(invalid))

        invalid = copy.deepcopy(self.projection)
        invalid["omissions"][0]["reason"] = "executed"
        self.assertFalse(self.validator.is_valid(invalid))

        namespaced = copy.deepcopy(self.contract)
        namespaced["id"] = "namespace:task"
        self.validator.validate(namespaced)

        nullable = copy.deepcopy(self.contract)
        for field in (
            "imperative_core", "task_contract", "repository_evidence",
            "execution_affordances", "verification_obligations", "source_maps",
        ):
            nullable[field] = None
        self.validator.validate(nullable)

        invalid = copy.deepcopy(self.contract)
        invalid["repository_evidence"][0]["source"]["path"] = "../secret"
        self.assertFalse(self.validator.is_valid(invalid))
