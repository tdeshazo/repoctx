#!/usr/bin/env python3
"""Validate, export, and independently check the M4 held-out pilot tasks."""

from __future__ import annotations

import argparse
import hashlib
import importlib
import json
from pathlib import Path, PurePosixPath
import shutil
import sys
from typing import Any


ROOT = Path(__file__).resolve().parents[1] / "evals" / "m4"
TASK_TYPES = {"documentation_lookup", "code_localization", "cross_file_analysis", "change", "edge_case"}
OUTCOMES = {"answerable", "contradictory_evidence", "no_answer", "access_denied"}
ANSWERABLE_TYPES = TASK_TYPES - {"edge_case"}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def relative_path(value: Any, *, allow_root: bool = False) -> str:
    require(isinstance(value, str) and bool(value), "path must be a nonempty string")
    if allow_root and value == ".":
        return value
    path = PurePosixPath(value)
    require(not path.is_absolute() and path.as_posix() == value, "path must be canonical and relative")
    require(not any(part in {".", ".."} for part in path.parts) and "\\" not in value,
            "path cannot escape the repository")
    return value


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"{path.name}: expected JSON object")
    return value


def inside(path: str, prefixes: list[str]) -> bool:
    return any(prefix == "." or path == prefix or path.startswith(prefix + "/") for prefix in prefixes)


def permitted(path: str, allowed: list[str], denied: list[str]) -> bool:
    return inside(path, allowed) and not inside(path, denied)


def repository_files(root: Path) -> list[str]:
    require(root.is_dir() and not root.is_symlink(), f"missing repository: {root.name}")
    files = []
    for path in root.rglob("*"):
        require(not path.is_symlink(), f"repository contains symlink: {path.name}")
        if path.is_file():
            path.read_bytes().decode("utf-8")
            files.append(path.relative_to(root).as_posix())
    return sorted(files)


def resolve_anchor(repo: Path, anchor: dict[str, Any]) -> dict[str, Any]:
    require(set(anchor) == {"id", "path", "start", "end"}, "invalid evidence anchor shape")
    relative = relative_path(anchor["path"])
    path = repo / relative
    require(path.is_file() and not path.is_symlink(), f"missing evidence file: {relative}")
    data = path.read_bytes()
    start = anchor["start"].encode("utf-8")
    end = anchor["end"].encode("utf-8")
    require(bool(start) and bool(end), "evidence anchors cannot be empty")
    start_at = data.find(start)
    end_at = data.find(end)
    require(start_at >= 0 and data.find(start, start_at + 1) < 0, "start anchor must be unique")
    require(end_at >= 0 and data.find(end, end_at + 1) < 0, "end anchor must be unique")
    end_at += len(end)
    require(end_at >= start_at + len(start), "evidence anchors are reversed")
    return {
        "id": anchor["id"], "path": relative,
        "sha256": hashlib.sha256(data).hexdigest(),
        "start_byte": start_at, "end_byte": end_at,
    }


def validate_repository(root: Path, spec: dict[str, Any]) -> tuple[dict[str, Any], dict[str, dict[str, Any]]]:
    require(set(spec) == {"id", "path", "evidence"}, "invalid repository record shape")
    relative = relative_path(spec["path"])
    repo = root / relative
    files = repository_files(repo)
    provenance = load_json(repo / "PROVENANCE.json")
    require(provenance == {"origin": "synthetic", "license": "Apache-2.0",
                           "copyright": "repoctx contributors", "redistribution": "cleared"},
            f"{spec['id']}: repository is not permission-cleared")
    evidence = {}
    for anchor in spec["evidence"]:
        resolved = resolve_anchor(repo, anchor)
        require(resolved["id"] not in evidence, "duplicate evidence ID")
        evidence[resolved["id"]] = resolved
    return {"id": spec["id"], "path": relative, "root": repo, "files": files}, evidence


def validate_check(check: dict[str, Any], repositories: dict[str, dict[str, Any]]) -> None:
    required = {"id", "repository", "module", "function", "args"}
    require(required.issubset(check) and set(check) <= required | {"equals", "contains"},
            "invalid change check shape")
    require(check["repository"] in repositories, "check names an unknown repository")
    require(isinstance(check["args"], list), "check args must be an array")
    require(("equals" in check) != ("contains" in check), "check needs exactly one expectation")
    parts = check["module"].split(".")
    require(all(part.isidentifier() for part in parts), "invalid check module")
    module_path = repositories[check["repository"]]["root"].joinpath(*parts).with_suffix(".py")
    require(module_path.is_file(), "check module does not exist")
    require(isinstance(check["function"], str) and check["function"].isidentifier(), "invalid check function")


def validate(root: Path = ROOT) -> tuple[dict[str, Any], dict[str, dict[str, Any]]]:
    errors: list[str] = []
    records: dict[str, dict[str, Any]] = {}
    counts = {kind: 0 for kind in TASK_TYPES}
    repo_counts: dict[str, int] = {}
    try:
        manifest = load_json(root / "manifest.json")
        require(manifest.get("version") == "repoctx.m4.tasks/v1alpha1", "unsupported manifest version")
        require(manifest.get("license") == "Apache-2.0", "unexpected corpus license")
        require(manifest.get("tuning_corpus") == "../m0/manifest.json", "tuning corpus must remain separate")
        tuning = load_json((root / manifest["tuning_corpus"]).resolve())
        tuning_ids = {item["id"] for item in tuning["fixtures"]}

        repositories: dict[str, dict[str, Any]] = {}
        evidence: dict[str, tuple[str, dict[str, Any]]] = {}
        for spec in manifest["repositories"]:
            repo, anchors = validate_repository(root, spec)
            require(repo["id"] not in repositories, "duplicate repository ID")
            repositories[repo["id"]] = repo
            repo_counts[repo["id"]] = 0
            for anchor_id, anchor in anchors.items():
                require(anchor_id not in evidence, "evidence IDs must be globally unique")
                evidence[anchor_id] = (repo["id"], anchor)
        require(len(repositories) >= 3, "at least three permission-cleared repositories are required")

        checks = {}
        for check in manifest["checks"]:
            validate_check(check, repositories)
            require(check["id"] not in checks, "duplicate check ID")
            checks[check["id"]] = check

        questions = set()
        used_checks = set()
        for task in manifest["tasks"]:
            task_id = task.get("id", "<missing>")
            try:
                allowed_keys = {"id", "repository", "type", "question", "outcome", "answer",
                                "evidence", "check", "allowed", "denied"}
                require(set(task) <= allowed_keys, "unknown task field")
                require(isinstance(task_id, str) and task_id not in records and task_id not in tuning_ids,
                        "duplicate, missing, or tuning-overlapping task ID")
                require(task["repository"] in repositories, "unknown task repository")
                require(task["type"] in TASK_TYPES, "unknown task type")
                require(task["outcome"] in OUTCOMES, "unknown task outcome")
                question = task["question"].strip()
                require(question and question.casefold() not in questions, "questions must be distinct")
                questions.add(question.casefold())
                allowed = task.get("allowed", ["."])
                denied = task.get("denied", [])
                require(isinstance(allowed, list) and bool(allowed) and isinstance(denied, list), "invalid scope")
                for value in allowed + denied:
                    relative_path(value, allow_root=True)
                repo = repositories[task["repository"]]
                visible = [path for path in repo["files"] if permitted(path, allowed, denied)]
                require(bool(visible), "task has no visible repository files")
                refs = task["evidence"]
                require(isinstance(refs, list) and len(refs) == len(set(refs)), "invalid evidence references")
                resolved = []
                for ref in refs:
                    require(ref in evidence and evidence[ref][0] == task["repository"], "unknown cross-repository evidence")
                    resolved.append(evidence[ref][1])

                outcome = task["outcome"]
                if outcome == "answerable":
                    require(task["type"] in ANSWERABLE_TYPES and isinstance(task["answer"], str) and
                            bool(task["answer"]) and bool(resolved), "answerable task is incomplete")
                    require(all(permitted(item["path"], allowed, denied) for item in resolved),
                            "answerable evidence is outside task scope")
                    repo_counts[task["repository"]] += 1
                else:
                    require(task["type"] == "edge_case" and task["answer"] is None,
                            "edge outcome must be an edge case without an answer")
                    if outcome == "contradictory_evidence":
                        require(len(resolved) >= 2 and all(permitted(item["path"], allowed, denied) for item in resolved),
                                "contradictory case needs multiple visible sources")
                    elif outcome == "no_answer":
                        require(not resolved and not denied, "no-answer case must cover the complete visible corpus")
                    else:
                        require(bool(denied) and bool(resolved) and
                                all(not permitted(item["path"], allowed, denied) for item in resolved),
                                "access-denied evidence must be excluded by scope")

                check_id = task.get("check")
                if task["type"] == "change":
                    require(check_id in checks and checks[check_id]["repository"] == task["repository"],
                            "change task lacks its repository check")
                    require(check_id not in used_checks, "change checks cannot be shared")
                    used_checks.add(check_id)
                else:
                    require(check_id is None, "only change tasks may name checks")
                counts[task["type"]] += 1
                records[task_id] = {"task": task, "repository": repo, "evidence": resolved,
                                    "visible": visible, "check": checks.get(check_id)}
            except (KeyError, TypeError, ValueError) as exc:
                errors.append(f"{task_id}: {exc}")

        require(len(used_checks) == len(checks), "every declared check must belong to one change task")
        require(sum(repo_counts.values()) >= 30 and all(value >= 10 for value in repo_counts.values()),
                f"answerable pilot floor not met: {repo_counts}")
        require(counts["documentation_lookup"] >= 1 and counts["code_localization"] >= 1 and
                counts["cross_file_analysis"] >= 1 and counts["change"] >= 1,
                f"required task types missing: {counts}")
        outcomes = {record["task"]["outcome"] for record in records.values()}
        require({"no_answer", "contradictory_evidence", "access_denied"} <= outcomes,
                "required edge cases are missing")
    except (KeyError, TypeError, ValueError, OSError, json.JSONDecodeError) as exc:
        errors.append(f"corpus: {exc}")
    return {"ok": not errors, "tasks": len(records), "types": counts,
            "answerable_by_repository": repo_counts, "errors": errors}, records


def export(records: dict[str, dict[str, Any]], task_id: str, output: Path) -> None:
    require(task_id in records, "unknown task ID")
    record = records[task_id]
    files = [(relative, (record["repository"]["root"] / relative).read_bytes())
             for relative in record["visible"]]
    output.mkdir(parents=True, exist_ok=False)
    repository = output / "repository"
    for relative, content in files:
        target = repository / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content)
    task = record["task"]
    agent = {"version": "repoctx.m4.agent-input/v1alpha1", "id": task_id,
             "question": task["question"], "repository": task["repository"],
             "files": record["visible"]}
    if record["check"]:
        agent["check_id"] = record["check"]["id"]
    (output / "input.json").write_text(json.dumps(agent, indent=2) + "\n", encoding="utf-8")


def run_change_check(record: dict[str, Any], repository: Path) -> tuple[bool, str]:
    check = record["check"]
    require(check is not None, "task is not a change task")
    require(repository.is_dir(), "verification repository is missing")
    top_level = check["module"].split(".")[0]
    previous = list(sys.path)
    try:
        for cache in repository.rglob("__pycache__"):
            shutil.rmtree(cache)
        importlib.invalidate_caches()
        sys.path.insert(0, str(repository.resolve()))
        for name in list(sys.modules):
            if name == top_level or name.startswith(top_level + "."):
                del sys.modules[name]
        function = getattr(importlib.import_module(check["module"]), check["function"])
        actual = function(*check["args"])
        if "equals" in check:
            passed = actual == check["equals"]
        else:
            passed = isinstance(actual, str) and check["contains"] in actual
        return passed, f"check {check['id']} {'passed' if passed else 'failed'}"
    except Exception as exc:  # The synthetic repository result is check output.
        return False, f"check {check['id']} failed: {type(exc).__name__}"
    finally:
        sys.path[:] = previous
        for name in list(sys.modules):
            if name == top_level or name.startswith(top_level + "."):
                del sys.modules[name]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--fixture")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--verify-change")
    parser.add_argument("--repository", type=Path)
    args = parser.parse_args()
    if bool(args.fixture) != bool(args.output):
        parser.error("--fixture and --output must be supplied together")
    if bool(args.verify_change) != bool(args.repository):
        parser.error("--verify-change and --repository must be supplied together")
    result, records = validate(args.root)
    if result["ok"] and args.fixture:
        try:
            export(records, args.fixture, args.output)
            result["exported"] = args.fixture
        except (OSError, ValueError) as exc:
            result["ok"] = False
            result["errors"].append(f"export: {exc}")
    if result["ok"] and args.verify_change:
        try:
            passed, message = run_change_check(records[args.verify_change], args.repository)
            result["check"] = {"passed": passed, "message": message}
            result["ok"] = passed
        except (KeyError, ValueError) as exc:
            result["ok"] = False
            result["errors"].append(f"check: {exc}")
    print(json.dumps(result, sort_keys=True))
    return 0 if result["ok"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
