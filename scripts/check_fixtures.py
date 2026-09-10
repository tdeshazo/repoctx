#!/usr/bin/env python3
"""Validate M0 evaluation metadata; optionally export one agent-visible fixture."""

from __future__ import annotations

import argparse
import json
from pathlib import Path, PurePosixPath
from typing import Any


EXPECTED_COUNTS = {"documentation": 4, "code": 4, "mixed": 2, "no_answer": 2}
ROOT = Path(__file__).resolve().parents[1] / "evals" / "m0"


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def relative_path(value: Any) -> str:
    require(isinstance(value, str) and bool(value), "path must be a nonempty string")
    path = PurePosixPath(value)
    require(not path.is_absolute() and path.as_posix() == value, "path must be canonical and relative")
    require(not any(part in {".", ".."} for part in path.parts) and "\\" not in value,
            "path cannot escape the fixture")
    return value


def source_path(root: Path, value: Any) -> Path:
    relative = relative_path(value)
    path = root / relative
    require(not any(part.is_symlink() for part in [path, *path.parents] if part != root.parent),
            "fixture paths cannot contain symlinks")
    require(path.is_file() and path.resolve().is_relative_to(root.resolve()),
            f"missing or escaped fixture file: {relative}")
    return path


def read(root: Path, relative: str) -> dict[str, Any]:
    value = json.loads(source_path(root, relative).read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"{relative}: expected JSON object")
    return value


def within(path: str, prefixes: list[str]) -> bool:
    return any(path == prefix or path.startswith(prefix + "/") for prefix in prefixes)


def permitted(task: dict[str, Any], path: str) -> bool:
    return within(path, task["allowed_scope"]) and not within(path, task["denied_scope"])


def validate_fixture(root: Path, item: dict[str, Any]) -> tuple[dict, dict, dict]:
    fixture_id = item["id"]
    refs = [item[field] for field in ("task", "expected", "agent_input")]
    require(len(set(refs)) == 3, "task, expected and agent input must be separate files")
    task, expected, agent = [read(root, ref) for ref in refs]
    require(all(record.get("id") == fixture_id for record in (task, expected, agent)),
            "record IDs disagree")
    require(isinstance(task.get("question"), str) and bool(task["question"].strip()), "missing question")
    require(set(agent) == {"id", "question", "files"}, "agent input contains evaluator metadata")
    require(agent["question"] == task["question"], "agent/task questions disagree")
    require("scoring" not in task and "scoring" not in agent, "scoring belongs in scoring.json")

    sources = task["source_files"]
    mapping = task["source_map"]
    require(isinstance(sources, list) and bool(sources) and len(set(sources)) == len(sources),
            "source_files must be a distinct nonempty list")
    require(isinstance(mapping, dict) and set(mapping) == set(sources), "source_map must cover source_files")
    require(len(set(mapping.values())) == len(mapping), "source_map destinations must be distinct")
    for scope in ("allowed_scope", "denied_scope"):
        require(isinstance(task[scope], list), f"{scope} must be a list")
        for prefix in task[scope]:
            relative_path(prefix)
    require(bool(task["allowed_scope"]), "allowed_scope cannot be empty")
    data = {}
    for source, destination in mapping.items():
        relative_path(destination)
        data[source] = source_path(root, source).read_bytes()
        data[source].decode("utf-8")
    visible = [path for path in mapping.values() if permitted(task, path)]
    require(isinstance(agent["files"], list) and agent["files"] == visible,
            "agent file inventory disagrees with permitted source mapping")
    require(bool(visible), "fixture needs permitted source evidence")

    budgets = task["budgets"]
    for key in ("max_bytes", "max_symbols"):
        require(type(budgets[key]) is int and budgets[key] > 0, f"{key} must be positive")
    limit = budgets.get("max_tokens")
    require(limit is None or type(limit) is int and limit > 0, "invalid exact token budget")
    require((limit is None and budgets.get("tokenizer") is None) or
            (limit is not None and isinstance(budgets.get("tokenizer"), str) and bool(budgets["tokenizer"])),
            "exact token budget requires tokenizer identity")

    spans = task["answer_spans"]
    require(isinstance(spans, list), "answer_spans must be a list")
    ids = []
    for span in spans:
        require(isinstance(span, dict), "span must be an object")
        span_id = span["id"]
        require(isinstance(span_id, str) and bool(span_id) and span_id not in ids, "duplicate/invalid span ID")
        ids.append(span_id)
        source = span["file"]
        require(source in data and permitted(task, mapping[source]), "answer span is outside allowed scope")
        start, end = span["start_byte"], span["end_byte"]
        require(type(start) is int and type(end) is int and 0 <= start < end <= len(data[source]),
                "invalid answer byte range")
        require(data[source][start:end].decode("utf-8") == span["text"], "answer span bytes/text disagree")
    require(expected["spans"] == ids, "expected spans disagree with task spans")
    relationships = task["expected_relationships"]
    require(isinstance(relationships, list) and expected["relationships"] == relationships,
            "expected relationships disagree with task")
    for rel in relationships:
        require(rel["kind"] in {"calls", "imports", "references"}, "unknown relationship kind")
        require(rel["resolution"] in {"syntactic", "name_heuristic", "unresolved"}, "invalid resolution")
        require(rel["surface"] in {"index", "context"}, "missing relationship observation surface")
        require(all(isinstance(rel[key], str) and rel[key] for key in ("from", "to")), "empty endpoint")
        require(isinstance(rel["evidence"], list) and bool(rel["evidence"]) and
                all(ref in ids for ref in rel["evidence"]), "relationship lacks supporting spans")

    outcome = expected["outcome"]
    require(outcome in {"answerable", "no_answer", "access_denied"}, "invalid outcome")
    if outcome == "answerable":
        require(item["category"] != "no_answer" and bool(spans), "answerable fixture needs evidence")
        require(isinstance(expected["answer"], str) and bool(expected["answer"]), "missing expected answer")
    else:
        require(item["category"] == "no_answer" and not spans and not relationships and
                expected["answer"] is None and bool(expected["explanation"]), "invalid no-answer expectation")
        if outcome == "access_denied":
            require(any(within(path, task["denied_scope"]) for path in mapping.values()),
                    "access_denied fixture needs a denied source")
        else:
            require(not task["denied_scope"], "no_answer must be evaluated on its complete permitted corpus")
    return task, expected, agent


def validate(root: Path) -> tuple[dict, dict[str, tuple[dict, dict, dict]]]:
    errors = []
    counts = dict.fromkeys(EXPECTED_COUNTS, 0)
    records = {}
    ids = set()
    signatures = set()
    try:
        manifest = read(root, "manifest.json")
        scoring = read(root, "scoring.json")
        require(manifest.get("version") == "repoctx.m0.fixtures/v2", "unsupported manifest version")
        require(scoring.get("version") == "repoctx.m0.scoring/v2", "unsupported scoring version")
        require(isinstance(scoring.get("rules"), dict) and
                all(isinstance(rule, str) and bool(rule.strip()) for rule in scoring["rules"].values()),
                "scoring rules must contain nonempty rubrics")
        require(set(scoring["rules"]) == {"answer_span_recall", "relationship_recall", "scope",
                                         "budget", "agent_outcome"}, "missing scoring rules")
        fixtures = manifest["fixtures"]
        require(isinstance(fixtures, list), "fixtures must be a list")
        for item in fixtures:
            label = item.get("id", "<missing>") if isinstance(item, dict) else "<invalid>"
            try:
                require(isinstance(label, str) and label not in ids and label != "<missing>",
                        "duplicate or missing fixture ID")
                ids.add(label)
                category = item["category"]
                require(category in counts, "unknown fixture category")
                task, expected, agent = validate_fixture(root, item)
                signature = (task["question"].strip().casefold(),
                             tuple(sorted((dest, source_path(root, src).read_bytes())
                                          for src, dest in task["source_map"].items())))
                require(signature not in signatures, "repeated task/source pair is not a distinct fixture")
                signatures.add(signature)
                counts[category] += 1
                records[label] = (task, expected, agent)
            except (ValueError, TypeError, KeyError, OSError, AttributeError) as exc:
                errors.append(f"{label}: {exc}")
        require(manifest["distinct_task_count"] == len(records), "declared distinct count disagrees with valid tasks")
        require(all(counts[key] >= minimum for key, minimum in EXPECTED_COUNTS.items()),
                f"category minimums not met: {counts}")
    except (ValueError, TypeError, KeyError, OSError, AttributeError) as exc:
        errors.append(f"corpus: {exc}")
    return {"ok": not errors, "counts": counts, "distinct": len(records), "errors": errors}, records


def export(root: Path, records: dict, fixture_id: str, output: Path) -> None:
    require(fixture_id in records, "unknown fixture ID")
    task, _, agent = records[fixture_id]
    # Validate and read everything before creating the destination. Export only
    # permitted bytes; never give the agent access to the evaluator directory.
    files = [(path, source_path(root, source).read_bytes())
             for source, path in task["source_map"].items() if permitted(task, path)]
    output.mkdir(parents=True, exist_ok=False)
    for path, data in files:
        target = output / "repository" / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
    (output / "input.json").write_text(json.dumps(agent, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--fixture", help="export only this fixture after validating the corpus")
    parser.add_argument("--output", type=Path, help="new directory for repository/ and input.json")
    args = parser.parse_args()
    if bool(args.fixture) != bool(args.output):
        parser.error("--fixture and --output must be supplied together")
    result, records = validate(args.root)
    if result["ok"] and args.fixture:
        try:
            export(args.root, records, args.fixture, args.output)
            result["exported"] = args.fixture
        except (ValueError, OSError) as exc:
            result["ok"] = False
            result["errors"].append(f"export: {exc}")
    print(json.dumps(result, sort_keys=True))
    return 0 if result["ok"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
