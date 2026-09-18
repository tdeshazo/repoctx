from __future__ import annotations

import copy
import json
from pathlib import Path
from unittest import TestCase

from jsonschema import Draft202012Validator

ROOT = Path(__file__).resolve().parents[1]


class ArtifactSchemaTests(TestCase):
    def setUp(self):
        schema = json.loads((ROOT / "docs/artifacts.schema.json").read_text())
        Draft202012Validator.check_schema(schema)
        self.validator = Draft202012Validator(schema)
        self.fixture = json.loads((ROOT / "pkg/artifacts/testdata/claims.json").read_text())

    def test_reviewed_claims_and_empty_arrays(self):
        self.validator.validate(self.fixture)
        self.fixture["artifacts"] = []
        self.fixture["relationships"] = []
        self.validator.validate(self.fixture)

    def test_closed_fields_types_enums_and_paths(self):
        changes = [
            ("kind", "unknown"), ("lifecycle", "accepted"), ("owner", None),
            ("owner", ""), ("id", "demo:A"), ("id", "demo:a\n"),
            ("sources", []), ("applies_to", None), ("declared_inputs", None),
            ("command", "touch EXECUTED"), ("runner_check_id", "check"),
        ]
        for key, value in changes:
            with self.subTest(key=key, value=value):
                fixture = copy.deepcopy(self.fixture)
                fixture["artifacts"][0][key] = value
                self.assertFalse(self.validator.is_valid(fixture))
        for path in [".", "..", "a/../b", "/a", "a/", "a//b", "a\\b", "a:b", "a*", "a\n"]:
            with self.subTest(path=path):
                fixture = copy.deepcopy(self.fixture)
                fixture["artifacts"][0]["sources"][0]["path"] = path
                self.assertFalse(self.validator.is_valid(fixture))

    def test_required_fields_at_each_level(self):
        paths = [(), ("artifacts", 0), ("artifacts", 0, "sources", 0),
                 ("artifacts", 0, "applies_to", 0), ("artifacts", 0, "declared_inputs", 0),
                 ("relationships", 0)]
        for path in paths:
            node = self.fixture
            for part in path:
                node = node[part]
            for key in node:
                if key == "sha256" and "role" in node:
                    continue
                fixture = copy.deepcopy(self.fixture)
                target = fixture
                for part in path:
                    target = target[part]
                del target[key]
                with self.subTest(path=path, key=key):
                    self.assertFalse(self.validator.is_valid(fixture))

    def test_schema_subset_does_not_claim_grounding(self):
        # Cross-field coordinate agreement, byte caps and integer spelling need
        # the Go decoder; schema validity alone is explicitly insufficient.
        self.fixture["artifacts"][0]["sources"][0]["sha256"] = "0" * 64
        self.validator.validate(self.fixture)
