#!/usr/bin/env python3
"""Validate and lock the controlled M4 experiment schedule without running trials."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import random
import subprocess
from typing import Any


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "evals" / "m4"
PROTOCOL = ROOT / "protocol.json"
CONDITIONS = ROOT / "conditions.json"
SCORING = ROOT / "scoring.json"
ACCOUNTING = ROOT / "accounting.json"
PLACEHOLDERS = {"", "unknown", "latest", "main", "head", "devel"}
TRACK_CONDITIONS = {
    "end_to_end": {"ordinary_tools", "bounded_lexical", "repoctx_no_graph",
                   "repoctx_graph", "artifact_aware"},
    "evidence_only": {"bounded_lexical", "repoctx_no_graph", "repoctx_graph",
                      "artifact_aware", "human_oracle"},
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def load_module(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    require(spec is not None and spec.loader is not None, f"cannot load {path.name}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"{path.name} must contain an object")
    return value


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def safe_relative(value: Any) -> str:
    require(isinstance(value, str) and bool(value), "path must be nonempty")
    path = PurePosixPath(value)
    require(not path.is_absolute() and path.as_posix() == value and
            not any(part in {".", ".."} for part in path.parts), "path must be safe and relative")
    return value


def validate_scoring(value: dict[str, Any]) -> None:
    require(value.get("version") == "repoctx.m4.scoring/v1alpha1", "unsupported scoring version")
    profiles = value.get("profiles")
    require(isinstance(profiles, dict) and set(profiles) == set(TRACK_CONDITIONS),
            "scoring profiles must match tracks")
    expected = {
        "end_to_end": {"outcome", "answer", "citations", "change", "scope",
                       "condition_adherence"},
        "evidence_only": {"outcome", "answer", "citations", "evidence", "scope",
                          "condition_adherence"},
    }
    for identifier, rules in profiles.items():
        require(isinstance(rules, dict) and set(rules) == expected[identifier],
                f"invalid {identifier} scoring rules")
        require(all(isinstance(rule, str) and rule.strip() for rule in rules.values()),
                f"empty {identifier} scoring rule")


def validate_protocol(protocol_path: Path = PROTOCOL) -> tuple[dict[str, Any], dict[str, Any]]:
    root = protocol_path.parent
    protocol = load_json(protocol_path)
    require(protocol.get("version") == "repoctx.m4.protocol/v1alpha1", "unsupported protocol version")
    required = {"version", "seed", "repetitions", "cache_states", "compiler_profile",
                "context_profile", "model_profile", "budgets", "permissions", "tracks",
                "ordering", "workspace_policy", "cache_observation"}
    require(set(protocol) == required, "invalid protocol fields")
    require(type(protocol["seed"]) is int and protocol["seed"] >= 0, "invalid schedule seed")
    require(type(protocol["repetitions"]) is int and protocol["repetitions"] > 0,
            "invalid repetition count")
    require(protocol["cache_states"] == ["cold", "warm"], "cache strata must be cold and warm")
    require(protocol["ordering"] == "seeded_global_shuffle", "schedule must use a global shuffle")
    require(protocol["workspace_policy"] == "fresh_export_per_trial", "workspaces must be fresh")

    compiler = protocol["compiler_profile"]
    require(compiler == {"id": "repoctx-source-v1", "ir_contract": "repoctx.ir/v1alpha4",
                         "provider": "source", "allow": ["."], "deny": [],
                         "max_source_bytes": 2097152, "max_total_source_bytes": 536870912},
            "compiler profile drifted")
    context = protocol["context_profile"]
    require(context == {"contract": "repoctx.context/v1alpha3", "max_symbols": 64,
                         "direction": "both"}, "context profile drifted")
    model = protocol["model_profile"]
    require(set(model) == {"reasoning_effort", "max_output_tokens", "timeout_seconds"} and
            model["reasoning_effort"] in {"low", "medium", "high"} and
            all(type(model[key]) is int and model[key] > 0
                for key in ("max_output_tokens", "timeout_seconds")), "invalid model profile")

    conditions_module = load_module("m4_conditions_for_protocol", PROJECT / "scripts/m4_conditions.py")
    condition_config, conditions = conditions_module.load_conditions(root / "conditions.json")
    budgets = protocol["budgets"]
    require(set(budgets) == {"condition_output_bytes", "tool_output_bytes_per_call",
                             "max_tool_calls", "tool_timeout_seconds", "trial_timeout_seconds"} and
            all(type(value) is int and value > 0 for value in budgets.values()), "invalid budgets")
    require(budgets["condition_output_bytes"] == condition_config["max_output_bytes"],
            "condition output budgets disagree")

    permissions = protocol["permissions"]
    require(set(permissions) == {"matched_repository", "evidence_only"},
            "invalid permission profiles")
    common_permission_fields = {"tools", "repository", "scratch", "network", "host_files",
                                "evaluator_files"}
    require(set(permissions["matched_repository"]) == common_permission_fields and
            permissions["matched_repository"]["tools"] == ["repository_shell"] and
            permissions["matched_repository"]["repository"] == "fresh_export_read_write" and
            permissions["matched_repository"]["scratch"] == "trial_private",
            "invalid matched permission profile")
    require(set(permissions["evidence_only"]) == common_permission_fields and
            permissions["evidence_only"]["tools"] == [] and
            permissions["evidence_only"]["repository"] == "no_runtime_access" and
            permissions["evidence_only"]["scratch"] == "none",
            "evidence-only track must have no tools or repository access")
    for profile in permissions.values():
        require(profile["network"] is False and profile["host_files"] is False and
                profile["evaluator_files"] is False, "permission profile leaks external access")

    tracks = protocol["tracks"]
    require(isinstance(tracks, list) and len(tracks) == 2, "expected two experiment tracks")
    indexed = {}
    for track in tracks:
        require(set(track) == {"id", "permission_profile", "prompt", "scoring_profile", "conditions"},
                "invalid track fields")
        identifier = track["id"]
        require(identifier in TRACK_CONDITIONS and identifier not in indexed, "unknown or duplicate track")
        require(set(track["conditions"]) == TRACK_CONDITIONS[identifier] and
                len(track["conditions"]) == len(TRACK_CONDITIONS[identifier]),
                f"{identifier} condition matrix drifted")
        require(all(condition in conditions for condition in track["conditions"]), "unknown condition")
        expected_permission = "matched_repository" if identifier == "end_to_end" else "evidence_only"
        require(track["permission_profile"] == expected_permission and
                track["scoring_profile"] == identifier, "track profile mismatch")
        prompt = root / safe_relative(track["prompt"])
        require(prompt.is_file() and prompt.read_text(encoding="utf-8").strip(), "missing track prompt")
        indexed[identifier] = track
    require(set(indexed) == set(TRACK_CONDITIONS), "track matrix is incomplete")

    cache = protocol["cache_observation"]
    require(set(cache) == {"cold", "warm", "os_cache", "required_trace_fields"} and
            all(isinstance(cache[key], str) and cache[key].strip()
                for key in ("cold", "warm", "os_cache")) and
            cache["required_trace_fields"] ==
            ["requested", "supplier_cache_before", "supplier_cache_after", "os_cache"],
            "cache observation policy is incomplete")
    scoring = load_json(root / "scoring.json")
    validate_scoring(scoring)
    accounting_module = load_module("m4_accounting_for_protocol",
                                    PROJECT / "scripts/m4_accounting.py")
    accounting_module.validate_config(root / "accounting.json")
    return protocol, scoring


def eligible(condition: str, record: dict[str, Any], conditions: dict[str, dict[str, Any]]) -> bool:
    if condition != "artifact_aware":
        return True
    return record["task"]["repository"] in conditions[condition]["catalogs"]


def build_schedule(protocol: dict[str, Any], records: dict[str, dict[str, Any]],
                   conditions: dict[str, dict[str, Any]]) -> list[dict[str, Any]]:
    schedule = []
    for track in protocol["tracks"]:
        for task_id in sorted(records):
            record = records[task_id]
            for condition in track["conditions"]:
                if not eligible(condition, record, conditions):
                    continue
                for cache in protocol["cache_states"]:
                    for repeat in range(1, protocol["repetitions"] + 1):
                        schedule.append({
                            "track": track["id"], "task": task_id, "condition": condition,
                            "cache_state": cache, "repeat": repeat,
                            "permission_profile": track["permission_profile"],
                            "scoring_profile": track["scoring_profile"],
                        })
    random.Random(protocol["seed"]).shuffle(schedule)
    for index, trial in enumerate(schedule, 1):
        trial["id"] = f"trial-{index:04d}"
        trial["workspace_id"] = f"workspace-{index:04d}"
        trial["fresh_workspace"] = True
        trial["cache_preparation"] = ("not_applicable" if trial["condition"] == "ordinary_tools"
                                      else protocol["cache_observation"][trial["cache_state"]])
        trial["cache_trace_required"] = protocol["cache_observation"]["required_trace_fields"]
    return schedule


def input_hashes() -> dict[str, str]:
    paths = [PROTOCOL, CONDITIONS, SCORING, ACCOUNTING,
             PROJECT / "scripts/check_m4_tasks.py", PROJECT / "scripts/m4_conditions.py",
             PROJECT / "scripts/m4_accounting.py", Path(__file__).resolve()]
    paths.extend(sorted((ROOT / "prompts").glob("*.txt")))
    paths.extend(sorted((ROOT / "artifacts").rglob("*")))
    paths.extend(sorted((ROOT / "repositories").rglob("*")))
    paths.extend([ROOT / "manifest.json"])
    return {path.relative_to(PROJECT).as_posix(): digest(path) for path in paths if path.is_file()}


def git_state() -> tuple[str, bool]:
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=PROJECT, text=True).strip()
    dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=PROJECT, text=True))
    return revision, dirty


def binary_identity(binary: Path) -> tuple[str, dict[str, Any]]:
    require(binary.is_file(), "repoctx binary is missing")
    result = subprocess.run([str(binary), "version", "-format", "json"], capture_output=True,
                            text=True, timeout=30)
    require(result.returncode == 0, "repoctx binary does not report build identity")
    build = json.loads(result.stdout)
    require(build.get("program") == "repoctx" and build.get("contracts", {}).get("ir") ==
            "repoctx.ir/v1alpha4" and build["contracts"].get("context") ==
            "repoctx.context/v1alpha3", "repoctx binary contracts do not match the protocol")
    return digest(binary), build


def create_lock(model_id: str, model_revision: str, binary: Path) -> dict[str, Any]:
    protocol, _ = validate_protocol()
    fixtures = load_module("check_m4_tasks_for_protocol", PROJECT / "scripts/check_m4_tasks.py")
    validation, records = fixtures.validate(fixtures.ROOT)
    require(validation["ok"], "M4 corpus is invalid")
    conditions_module = load_module("m4_conditions_for_lock", PROJECT / "scripts/m4_conditions.py")
    _, conditions = conditions_module.load_conditions(CONDITIONS)
    revision, dirty = git_state()
    require(len(revision) in {40, 64} and
            all(character in "0123456789abcdef" for character in revision),
            "source revision is not a full Git commit")
    require(not dirty, "refusing to lock a dirty worktree")
    require(model_id.casefold() not in PLACEHOLDERS and model_revision.casefold() not in PLACEHOLDERS,
            "model ID and immutable revision must be explicit")
    binary_sha256, build = binary_identity(binary.resolve())
    reported_revision = build.get("revision", "unknown").casefold()
    require(reported_revision in PLACEHOLDERS or reported_revision == revision,
            "repoctx binary revision does not match the source commit")
    require(build.get("modified", "unknown").casefold() != "true",
            "repoctx binary reports modified source")
    schedule = build_schedule(protocol, records, conditions)
    return {
        "version": "repoctx.m4.run-lock/v1alpha1",
        "source": {"revision": revision, "clean": True, "input_sha256": input_hashes()},
        "repoctx": {"binary_sha256": binary_sha256, "build": build,
                    "compiler_profile": protocol["compiler_profile"],
                    "context_profile": protocol["context_profile"]},
        "model": {"id": model_id, "revision": model_revision,
                  "profile": protocol["model_profile"]},
        "harness": {"protocol_sha256": digest(PROTOCOL), "conditions_sha256": digest(CONDITIONS),
                    "scoring_sha256": digest(SCORING), "accounting_sha256": digest(ACCOUNTING),
                    "seed": protocol["seed"],
                    "ordering": protocol["ordering"], "budgets": protocol["budgets"],
                    "cache_observation": protocol["cache_observation"]},
        "schedule": schedule,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["validate", "lock"])
    parser.add_argument("--model-id")
    parser.add_argument("--model-revision")
    parser.add_argument("--repoctx", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    protocol, _ = validate_protocol()
    if args.action == "validate":
        print(json.dumps({"ok": True, "tracks": [track["id"] for track in protocol["tracks"]],
                          "seed": protocol["seed"]}, sort_keys=True))
        return 0
    if not args.model_id or not args.model_revision or not args.repoctx or not args.output:
        parser.error("lock requires --model-id, --model-revision, --repoctx, and --output")
    require(not args.output.exists(), "output already exists")
    try:
        args.output.resolve().relative_to(PROJECT)
    except ValueError:
        pass
    else:
        raise ValueError("run locks must be written outside the indexed repository")
    value = create_lock(args.model_id, args.model_revision, args.repoctx)
    args.output.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
