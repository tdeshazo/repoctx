from __future__ import annotations

import copy
import json
from pathlib import Path
from unittest import TestCase

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]


class ManifestSchemaTests(TestCase):
    def setUp(self):
        schema = json.loads((ROOT / "docs/manifest.schema.json").read_text())
        Draft202012Validator.check_schema(schema)
        self.validator = Draft202012Validator(schema)
        self.fixture = {
            "version": "repoctx.manifest/v1alpha3",
            "namespace": "demo",
            "source_roots": [{"id": "source", "path": "."}],
            "document_sources": [],
            "artifact_sources": [],
            "components": [{
                "id": "core",
                "source_roots": ["source"],
                "artifact_sources": [],
                "provider_inputs": [],
                "owners": [],
                "scopes": [{"kind": "subtree", "path": "."}],
                "lifecycle": "active",
                "supersedes": [],
                "sensitivity": "internal",
                "freshness": {"inputs": ["go.mod"]},
            }],
            "provider_inputs": [],
            "derived_views": [{
                "id": "index",
                "kind": "repository_ir",
                "components": ["core"],
            }],
            "capabilities": ["syntax_graph"],
        }

    def test_valid_shape(self):
        self.validator.validate(self.fixture)

    def test_closed_required_and_generated_fields(self):
        for key in self.fixture:
            fixture = copy.deepcopy(self.fixture)
            del fixture[key]
            with self.subTest(missing=key):
                self.assertFalse(self.validator.is_valid(fixture))
        for field in ("sha256", "start_byte", "snapshot_id", "authority"):
            fixture = copy.deepcopy(self.fixture)
            fixture[field] = "not-authored"
            with self.subTest(generated=field):
                self.assertFalse(self.validator.is_valid(fixture))

    def test_enums_paths_and_duplicate_references(self):
        changes = [
            ("version", "repoctx.manifest/v0"),
            ("namespace", "Demo"),
            ("capabilities", ["execute"]),
            ("source_roots", [{"id": "source", "path": "../escape"}]),
            ("source_roots", [{"id": "source\n", "path": "."}]),
        ]
        for key, value in changes:
            fixture = copy.deepcopy(self.fixture)
            fixture[key] = value
            with self.subTest(key=key):
                self.assertFalse(self.validator.is_valid(fixture))
        fixture = copy.deepcopy(self.fixture)
        fixture["components"][0]["source_roots"] = ["source", "source"]
        self.assertFalse(self.validator.is_valid(fixture))


class FrontmatterSchemaTests(TestCase):
    def setUp(self):
        schema = json.loads((ROOT / "docs/frontmatter.schema.json").read_text())
        Draft202012Validator.check_schema(schema)
        self.validator = Draft202012Validator(schema)
        self.fixture = {
            "version": "repoctx.frontmatter/v1alpha1",
            "entities": [{
                "id": "maintainers",
                "kind": "owner",
                "owners": [],
                "scopes": [],
                "lifecycle": "active",
                "supersedes": [],
                "sensitivity": "internal",
                "freshness": {"inputs": ["agent-context.yaml"]},
            }],
        }

    def test_valid_closed_shape(self):
        self.validator.validate(self.fixture)
        fixture = copy.deepcopy(self.fixture)
        fixture["entities"][0]["status"] = "trusted"
        self.assertFalse(self.validator.is_valid(fixture))

    def test_all_entity_kinds(self):
        for kind in (
            "document",
            "component",
            "decision",
            "contract",
            "requirement",
            "verification_obligation",
            "owner",
        ):
            fixture = copy.deepcopy(self.fixture)
            fixture["entities"][0]["kind"] = kind
            with self.subTest(kind=kind):
                self.validator.validate(fixture)
