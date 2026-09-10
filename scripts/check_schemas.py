#!/usr/bin/env python3
"""Validate representative IR/context payloads against the published schemas.

M0 requires full Draft 2020-12 validation. ``jsonschema`` is a test/development
dependency (declared in ``requirements-test.txt``); a missing dependency is a
machine-readable failure rather than a passing contract subset.
"""

from __future__ import annotations

import argparse
import gzip
import json
import sys
from pathlib import Path
from typing import Any


def load_payload(path: Path) -> Any:
    raw = path.read_bytes()
    if path.suffix == ".gz":
        raw = gzip.decompress(raw)
    return json.loads(raw)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--ir-schema", type=Path, required=True)
    parser.add_argument("--context-schema", type=Path, required=True)
    parser.add_argument("--ir", type=Path, required=True)
    parser.add_argument("--context", type=Path, required=True)
    args = parser.parse_args(argv)

    checks: list[dict[str, Any]] = []
    try:
        ir = load_payload(args.ir)
        context = load_payload(args.context)
        # Loading schemas is itself a check that the declared artifacts are
        # valid JSON. Full validation below is conditional only on the optional
        # dependency, never on network access or a package installation.
        ir_schema = json.loads(args.ir_schema.read_text(encoding="utf-8"))
        context_schema = json.loads(args.context_schema.read_text(encoding="utf-8"))
        del ir_schema, context_schema
        try:
            from jsonschema import Draft202012Validator  # type: ignore
        except ImportError as exc:
            raise RuntimeError("jsonschema is required for full Draft 2020-12 validation; install requirements-test.txt") from exc
        for artifact, schema_path, payload in ((args.ir, args.ir_schema, ir), (args.context, args.context_schema, context)):
            schema = json.loads(schema_path.read_text(encoding="utf-8"))
            validator = Draft202012Validator(schema)
            errors = sorted(error.message for error in validator.iter_errors(payload))
            checks.append({"artifact": str(artifact), "schema": str(schema_path), "mode": "jsonschema-draft2020-12", "errors": errors})
    except Exception as exc:  # noqa: BLE001 - report machine-readable failure.
        checks.append({"artifact": "schema-check", "mode": "error", "errors": [f"{type(exc).__name__}: {exc}"]})

    result = {"ok": all(not check["errors"] for check in checks), "checks": checks}
    print(json.dumps(result, sort_keys=True))
    return 0 if result["ok"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
