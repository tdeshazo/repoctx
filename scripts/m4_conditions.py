#!/usr/bin/env python3
"""Render distinct M4 comparison-condition inputs without running an agent."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import tempfile
from typing import Any, Optional


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "evals" / "m4"
CONDITIONS = ROOT / "conditions.json"
STOP_WORDS = {"a", "an", "and", "are", "does", "for", "from", "how", "in", "is",
              "it", "of", "on", "the", "to", "what", "when", "where", "which", "with"}
EXPECTED = {
    "ordinary_tools": ("interactive", "filesystem_tools", False, None, "none", "all"),
    "bounded_lexical": ("evidence", "independent_lexical_windows", False, None, "none", "all"),
    "repoctx_no_graph": ("evidence", "repoctx_context", True, 0, "none", "all"),
    "repoctx_graph": ("evidence", "repoctx_context", True, 1, "none", "all"),
    "artifact_aware": ("evidence", "declared_artifact_spans", False, None,
                       "repository_declarations", "declared_catalog"),
    "human_oracle": ("evidence", "required_evidence_spans", False, None, "human_gold", "all"),
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def compact(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def fixture_module():
    spec = importlib.util.spec_from_file_location("check_m4_tasks", PROJECT / "scripts/check_m4_tasks.py")
    require(spec is not None and spec.loader is not None, "fixture validator is unavailable")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def safe_relative(value: Any) -> str:
    require(isinstance(value, str) and bool(value), "catalog path must be nonempty")
    path = PurePosixPath(value)
    require(not path.is_absolute() and path.as_posix() == value and
            not any(part in {".", ".."} for part in path.parts), "unsafe catalog path")
    return value


def load_conditions(path: Path = CONDITIONS) -> tuple[dict[str, Any], dict[str, dict[str, Any]]]:
    config = json.loads(path.read_text(encoding="utf-8"))
    require(config.get("version") == "repoctx.m4.conditions/v1alpha1", "unsupported conditions version")
    require(type(config.get("max_output_bytes")) is int and config["max_output_bytes"] >= 4096,
            "invalid output bound")
    conditions = config.get("conditions")
    require(isinstance(conditions, list), "conditions must be an array")
    indexed = {}
    for condition in conditions:
        identifier = condition.get("id")
        require(identifier in EXPECTED and identifier not in indexed, "unknown or duplicate condition")
        fields = (condition.get("mode"), condition.get("supplier"), condition.get("repoctx"),
                  condition.get("graph_depth"), condition.get("curation"), condition.get("eligibility"))
        require(fields == EXPECTED[identifier], f"condition {identifier} is mislabeled")
        allowed = {"id", "mode", "supplier", "repoctx", "graph_depth", "curation", "eligibility"}
        if identifier == "artifact_aware":
            allowed.add("catalogs")
            require(isinstance(condition.get("catalogs"), dict) and condition["catalogs"],
                    "artifact-aware condition needs catalogs")
            for relative in condition["catalogs"].values():
                catalog = path.parent / safe_relative(relative)
                require(catalog.is_file(), "artifact catalog is missing")
                value = json.loads(catalog.read_text(encoding="utf-8"))
                require(value.get("version") == "repoctx.artifacts/v1alpha1", "invalid artifact catalog")
        require(set(condition) == allowed, f"condition {identifier} has unknown fields")
        indexed[identifier] = condition
    require(set(indexed) == set(EXPECTED), "comparison matrix is incomplete")
    return config, indexed


def terms(question: str) -> list[str]:
    return sorted({term for term in re.findall(r"[^\W_]+", question.casefold())
                   if len(term) >= 2 and term not in STOP_WORDS})


def source_evidence(repository: Path, path: str, start: int, end: int, **extra: Any) -> dict[str, Any]:
    data = (repository / path).read_bytes()
    require(0 <= start < end <= len(data), "invalid evidence range")
    evidence = {"file": path, "sha256": hashlib.sha256(data).hexdigest(),
                "start_byte": start, "end_byte": end,
                "text": data[start:end].decode("utf-8")}
    evidence.update(extra)
    return evidence


def lexical_payload(record: dict[str, Any], repository: Path) -> dict[str, Any]:
    query = terms(record["task"]["question"])
    candidates = []
    for path in record["visible"]:
        data = (repository / path).read_bytes()
        offset = 0
        lines = data.splitlines(keepends=True)
        for index, line in enumerate(lines):
            text = line.decode("utf-8")
            lowered = text.casefold()
            matched = [term for term in query if term in lowered]
            if matched:
                start = sum(len(value) for value in lines[:max(0, index - 1)])
                end = sum(len(value) for value in lines[:min(len(lines), index + 2)])
                candidates.append((len(set(matched)), len(matched), path, offset,
                                   source_evidence(repository, path, start, end,
                                                   matched_terms=sorted(set(matched)),
                                                   selection="bounded_lexical")))
            offset += len(line)
    candidates.sort(key=lambda item: (-item[0], -item[1], item[2], item[3]))
    selected = []
    seen = set()
    for _, _, path, _, evidence in candidates:
        key = (path, evidence["start_byte"], evidence["end_byte"])
        if key not in seen:
            selected.append(evidence)
            seen.add(key)
    return {"query_terms": query, "evidence": selected}


def oracle_payload(record: dict[str, Any], repository: Path) -> dict[str, Any]:
    evidence = []
    for item in record["evidence"]:
        if item["path"] not in record["visible"]:
            continue
        evidence.append(source_evidence(repository, item["path"], item["start_byte"], item["end_byte"],
                                        selection="human_oracle"))
    return {"evidence": evidence, "scope_limited": len(evidence) != len(record["evidence"])}


def artifact_payload(record: dict[str, Any], repository: Path, catalog_path: Path) -> dict[str, Any]:
    catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
    query = terms(record["task"]["question"])
    ranked = []
    for artifact in catalog["artifacts"]:
        spans = []
        score_terms = set()
        for span in artifact["sources"]:
            if span["path"] not in record["visible"]:
                continue
            evidence = source_evidence(repository, span["path"], span["start_byte"], span["end_byte"],
                                       selection="declared_artifact", artifact_id=artifact["id"])
            require(evidence["sha256"] == span["sha256"], "artifact catalog source hash is stale")
            haystack = (artifact["id"] + " " + span["path"] + " " + evidence["text"]).casefold()
            score_terms.update(term for term in query if term in haystack)
            spans.append(evidence)
        if spans and score_terms:
            ranked.append((len(score_terms), artifact["id"], spans, sorted(score_terms)))
    ranked.sort(key=lambda item: (-item[0], item[1]))
    evidence = []
    selected_ids = []
    for _, artifact_id, spans, matched in ranked:
        selected_ids.append(artifact_id)
        for span in spans:
            span["matched_terms"] = matched
            evidence.append(span)
    selected = set(selected_ids)
    relationships = [relationship for relationship in catalog["relationships"]
                     if relationship["from"] in selected and relationship["to"] in selected]
    return {"catalog_version": catalog["version"], "selected_artifacts": selected_ids,
            "evidence": evidence, "relationships": relationships}


def repoctx_payload(record: dict[str, Any], repository: Path, binary: Path,
                    depth: int, max_bytes: int) -> dict[str, Any]:
    require(binary.is_file(), "repoctx binary is required for this condition")
    with tempfile.TemporaryDirectory(prefix="repoctx-m4-condition-") as temp:
        index = Path(temp) / "repository.ir.json.gz"
        compiled = subprocess.run([str(binary), "compile", "-root", str(repository), "-o", str(index)],
                                  capture_output=True, text=True, timeout=60)
        require(compiled.returncode == 0, "repoctx compilation failed")
        command = [str(binary), "context", "-root", str(repository), "-query",
                   record["task"]["question"], "-depth", str(depth), "-max-bytes",
                   str(max_bytes), "-max-symbols", "64", str(index)]
        result = subprocess.run(command, capture_output=True, text=True, timeout=60)
        require(result.returncode == 0, "repoctx context generation failed")
        bundle = json.loads(result.stdout)
    return {"context_version": bundle["version"], "graph_depth": depth, "bundle": bundle}


def envelope(condition: dict[str, Any], record: dict[str, Any], payload: dict[str, Any],
             max_bytes: int, availability: str = "available") -> dict[str, Any]:
    value = {
        "version": "repoctx.m4.condition-output/v1alpha1",
        "condition": {key: condition[key] for key in
                      ("id", "mode", "supplier", "repoctx", "graph_depth", "curation")},
        "task": {key: record["task"][key] for key in ("id", "repository", "question")},
        "availability": availability,
        "payload": payload,
        "omissions": [],
        "usage": {"bytes": 0},
    }
    evidence = payload.get("evidence")
    while len(compact(value)) > max_bytes and evidence:
        evidence.pop()
        if value["omissions"] and value["omissions"][0]["reason"] == "output_limit":
            value["omissions"][0]["count"] += 1
        else:
            value["omissions"].append({"reason": "output_limit", "count": 1})
    require(len(compact(value)) <= max_bytes, "condition output exceeds byte limit")
    for _ in range(3):
        size = len(compact(value))
        if value["usage"]["bytes"] == size:
            break
        value["usage"]["bytes"] = size
    require(len(compact(value)) <= max_bytes, "condition output usage exceeds byte limit")
    return value


def render(record: dict[str, Any], condition: dict[str, Any], repository: Path,
           max_bytes: int, *, binary: Optional[Path] = None,
           config_root: Path = ROOT) -> dict[str, Any]:
    identifier = condition["id"]
    if identifier == "ordinary_tools":
        payload = {"tools": ["files", "search", "read"], "files": record["visible"],
                   "preselected_evidence": False}
    elif identifier == "bounded_lexical":
        payload = lexical_payload(record, repository)
    elif identifier == "human_oracle":
        payload = oracle_payload(record, repository)
    elif identifier == "artifact_aware":
        relative = condition["catalogs"].get(record["task"]["repository"])
        if relative is None:
            return envelope(condition, record, {"reason": "no declared catalog"}, max_bytes, "unavailable")
        payload = artifact_payload(record, repository, config_root / safe_relative(relative))
    else:
        require(binary is not None, "repoctx binary is required")
        payload = repoctx_payload(record, repository, binary, condition["graph_depth"], max_bytes - 3000)
    return envelope(condition, record, payload, max_bytes)


def load_records() -> dict[str, dict[str, Any]]:
    module = fixture_module()
    validation, records = module.validate(module.ROOT)
    require(validation["ok"], "M4 task corpus is invalid")
    return records


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["validate", "render"])
    parser.add_argument("--task")
    parser.add_argument("--condition", choices=sorted(EXPECTED))
    parser.add_argument("--repository", type=Path)
    parser.add_argument("--repoctx", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    config, conditions = load_conditions()
    if args.action == "validate":
        print(json.dumps({"ok": True, "conditions": list(conditions),
                          "max_output_bytes": config["max_output_bytes"]}, sort_keys=True))
        return 0
    if not args.task or not args.condition or not args.repository:
        parser.error("render requires --task, --condition, and --repository")
    records = load_records()
    require(args.task in records, "unknown task")
    result = render(records[args.task], conditions[args.condition], args.repository,
                    config["max_output_bytes"], binary=args.repoctx)
    data = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        require(not args.output.exists(), "output already exists")
        args.output.write_text(data, encoding="utf-8")
    else:
        print(data, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
