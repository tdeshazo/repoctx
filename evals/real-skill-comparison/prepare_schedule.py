#!/usr/bin/env python3
"""Freeze a balanced, repeated schedule for the real skill comparison."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import random
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parent
PROJECT = ROOT.parents[1]
PROTOCOL_PATH = ROOT / "protocol.json"
TASK_VERSION = "repoctx.real-skill-comparison.tasks/v1"
LOCK_VERSION = "repoctx.real-skill-comparison.lock/v1"
HEX_256 = re.compile(r"^[0-9a-f]{64}$")
HEX_REVISION = re.compile(r"^(?:[0-9a-f]{40}|[0-9a-f]{64})$")
PLACEHOLDERS = {"", "current", "default", "dev", "head", "latest", "main",
                "unknown", "unresolved"}
BUDGET_FIELDS = {
    "max_context_tokens",
    "max_output_tokens",
    "max_tool_calls",
    "max_trial_seconds",
    "max_tool_output_bytes",
    "tool_timeout_seconds",
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def digest_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def digest_file(path: Path) -> str:
    return digest_bytes(path.read_bytes())


def load_object(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read {label}: {exc}") from exc
    require(isinstance(value, dict), f"{label} must be a JSON object")
    return value


def sha256(value: Any, label: str) -> None:
    require(isinstance(value, str) and HEX_256.fullmatch(value) is not None,
            f"{label} must be a lowercase SHA-256 digest")


def revision(value: Any, label: str) -> None:
    require(isinstance(value, str) and HEX_REVISION.fullmatch(value) is not None,
            f"{label} must be a full 40- or 64-character Git revision")


def validate_protocol(protocol: dict[str, Any]) -> None:
    require(protocol.get("version") == "repoctx.real-skill-comparison/v1",
            "unsupported comparison protocol version")
    sample = protocol.get("sample")
    require(isinstance(sample, dict) and type(sample.get("minimum_distinct_tasks")) is int and
            sample["minimum_distinct_tasks"] >= 30 and
            type(sample.get("repeats_per_task_condition")) is int and
            sample["repeats_per_task_condition"] == 3,
            "protocol must retain at least 30 tasks and three repeats per condition")
    family_floors = sample.get("minimum_tasks_per_family")
    require(isinstance(family_floors, dict) and set(family_floors) ==
            {"known_file", "navigation_or_relationship", "change"} and
            all(type(value) is int and value > 0 for value in family_floors.values()),
            "protocol task-family floors are invalid")
    randomization = protocol.get("randomization")
    require(isinstance(randomization, dict) and
            type(randomization.get("seed")) is int and randomization["seed"] >= 0 and
            randomization.get("unit") == "task_id_and_repeat" and
            randomization.get("within_pair_order") ==
            "randomized_and_balanced_across_pairs" and
            randomization.get("pairing") ==
            "run both conditions for the same task and repeat in adjacent slots",
            "protocol pairing or randomization rules drifted")
    primary = protocol.get("primary_comparison")
    require(isinstance(primary, dict) and primary.get("initial_index") ==
            "empty_private_scratch_for_each_trial" and
            primary.get("prefilled_evidence") is False,
            "primary comparison must start cold and contain no prefilled evidence")


def validate_task_manifest(tasks: dict[str, Any], protocol: dict[str, Any]) -> list[dict[str, Any]]:
    require(set(tasks) == {"version", "tasks"},
            "task manifest may contain only version and task identity records")
    require(tasks.get("version") == TASK_VERSION, "unsupported task manifest version")
    records = tasks.get("tasks")
    require(isinstance(records, list), "tasks must be an array")
    sample = protocol["sample"]
    minimum = sample["minimum_distinct_tasks"]
    require(len(records) >= minimum, f"at least {minimum} distinct tasks are required")

    ids: set[str] = set()
    family_floors = sample["minimum_tasks_per_family"]
    family_counts = {family: 0 for family in family_floors}
    required_fields = {"id", "family", "repository_revision", "repository_tree_sha256",
                       "task_sha256", "grader_sha256"}
    for index, record in enumerate(records, 1):
        require(isinstance(record, dict) and set(record) == required_fields,
                f"task {index} must contain exactly {sorted(required_fields)}")
        task_id = record["id"]
        require(isinstance(task_id, str) and re.fullmatch(r"[a-z0-9][a-z0-9._-]{0,63}", task_id),
                f"task {index} has an invalid id")
        require(task_id not in ids, f"duplicate task id: {task_id}")
        ids.add(task_id)
        family = record["family"]
        require(isinstance(family, str) and family in family_floors,
                f"task {task_id} has an unknown family")
        family_counts[family] += 1
        revision(record["repository_revision"], f"{task_id}.repository_revision")
        for field in ("repository_tree_sha256", "task_sha256", "grader_sha256"):
            sha256(record[field], f"{task_id}.{field}")

    for family, floor in family_floors.items():
        require(family_counts[family] >= floor,
                f"task family {family} has {family_counts[family]} tasks; {floor} required")
    return sorted(records, key=lambda record: record["id"])


def git_state() -> tuple[str, bool]:
    revision_value = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=PROJECT, text=True).strip()
    dirty = bool(subprocess.check_output(
        ["git", "status", "--porcelain"], cwd=PROJECT, text=True))
    return revision_value, dirty


def validate_run_lock(lock: dict[str, Any], project_revision: str, dirty: bool) -> None:
    require(lock.get("version") == LOCK_VERSION, "unsupported run-lock version")
    required = {"version", "model", "repoctx", "skill", "harness", "execution"}
    require(set(lock) == required, "invalid run-lock fields")

    model = lock["model"]
    require(isinstance(model, dict) and set(model) ==
            {"provider", "id", "revision", "reasoning_effort"}, "invalid model identity")
    for field in ("provider", "id", "revision", "reasoning_effort"):
        require(isinstance(model[field], str) and model[field].strip() and
                model[field].strip().lower() not in PLACEHOLDERS,
                f"model.{field} must be pinned before scheduling")

    repoctx = lock["repoctx"]
    require(isinstance(repoctx, dict) and set(repoctx) ==
            {"source_revision", "binary_sha256", "version_output_sha256"},
            "invalid Repoctx identity")
    revision(repoctx["source_revision"], "repoctx.source_revision")
    sha256(repoctx["binary_sha256"], "repoctx.binary_sha256")
    sha256(repoctx["version_output_sha256"], "repoctx.version_output_sha256")

    skill = lock["skill"]
    require(isinstance(skill, dict) and set(skill) ==
            {"source_revision", "snapshot_sha256"}, "invalid skill identity")
    revision(skill["source_revision"], "skill.source_revision")
    sha256(skill["snapshot_sha256"], "skill.snapshot_sha256")

    harness = lock["harness"]
    require(isinstance(harness, dict) and set(harness) ==
            {"project_revision", "working_tree_clean", "generator_sha256", "client", "client_version"},
            "invalid harness identity")
    revision(harness["project_revision"], "harness.project_revision")
    require(harness["project_revision"] == project_revision,
            "run lock does not pin the current project revision")
    require(harness["working_tree_clean"] is True and not dirty,
            "the project working tree must be clean")
    require(harness["generator_sha256"] == digest_file(Path(__file__).resolve()),
            "run lock does not pin this schedule generator")
    for field in ("client", "client_version"):
        require(isinstance(harness[field], str) and harness[field].strip() and
                harness[field].strip().lower() not in PLACEHOLDERS,
                f"harness.{field} must be pinned before scheduling")

    execution = lock["execution"]
    require(isinstance(execution, dict) and set(execution) ==
            {"permission_profile_sha256", "tool_profile_sha256", "budgets"},
            "invalid matched execution profile")
    sha256(execution["permission_profile_sha256"], "execution.permission_profile_sha256")
    sha256(execution["tool_profile_sha256"], "execution.tool_profile_sha256")
    budgets = execution["budgets"]
    require(isinstance(budgets, dict) and set(budgets) == BUDGET_FIELDS and
            all(type(value) is int and value > 0 for value in budgets.values()),
            "execution budgets must be positive integers and shared by both conditions")


def build_schedule(tasks: list[dict[str, Any]], protocol: dict[str, Any]) -> list[dict[str, Any]]:
    randomizer = random.Random(protocol["randomization"]["seed"])
    repeats = protocol["sample"]["repeats_per_task_condition"]
    pairs = [(task, repeat) for task in tasks for repeat in range(1, repeats + 1)]
    randomizer.shuffle(pairs)
    orders = ["control_first"] * (len(pairs) // 2)
    orders += ["treatment_first"] * (len(pairs) - len(orders))
    randomizer.shuffle(orders)

    schedule = []
    for pair_number, ((task, repeat), order) in enumerate(zip(pairs, orders), 1):
        conditions = (["control", "treatment"] if order == "control_first"
                      else ["treatment", "control"])
        for position, condition in enumerate(conditions, 1):
            schedule.append({
                "trial_id": f"trial-{len(schedule) + 1:04d}",
                "workspace_id": f"workspace-{len(schedule) + 1:04d}",
                "pair_id": f"pair-{pair_number:04d}",
                "pair_position": position,
                "task_id": task["id"],
                "task_family": task["family"],
                "repeat": repeat,
                "condition": condition,
                "skill_visible": condition == "treatment",
                "repoctx_binary": "same_pinned_binary",
                "index_state_at_start": "empty",
                "prefilled_evidence": False,
                "fresh_workspace": True,
                "repository_revision": task["repository_revision"],
                "repository_tree_sha256": task["repository_tree_sha256"],
                "task_sha256": task["task_sha256"],
                "grader_sha256": task["grader_sha256"],
            })
    return schedule


def freeze(tasks_path: Path, lock_path: Path, output_path: Path) -> dict[str, Any]:
    protocol = load_object(PROTOCOL_PATH, "protocol")
    validate_protocol(protocol)
    tasks_document = load_object(tasks_path, "task manifest")
    task_records = validate_task_manifest(tasks_document, protocol)
    run_lock = load_object(lock_path, "run lock")
    project_revision, dirty = git_state()
    validate_run_lock(run_lock, project_revision, dirty)

    if output_path.exists():
        raise ValueError(f"refusing to overwrite existing schedule: {output_path}")
    schedule = build_schedule(task_records, protocol)
    frozen = {
        "version": "repoctx.real-skill-comparison.schedule/v1",
        "protocol_sha256": digest_file(PROTOCOL_PATH),
        "task_manifest_sha256": digest_file(tasks_path),
        "run_lock_sha256": digest_file(lock_path),
        "schedule_generator_sha256": digest_file(Path(__file__).resolve()),
        "project_revision": project_revision,
        "seed": protocol["randomization"]["seed"],
        "distinct_tasks": len(task_records),
        "repeats_per_task_condition": protocol["sample"]["repeats_per_task_condition"],
        "pairs": len(schedule) // 2,
        "trials": len(schedule),
        "schedule": schedule,
    }
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(frozen, indent=2) + "\n", encoding="utf-8")
    return {
        "schedule": str(output_path),
        "schedule_sha256": digest_file(output_path),
        "distinct_tasks": len(task_records),
        "pairs": len(schedule) // 2,
        "trials": len(schedule),
        "control_first_pairs": sum(
            1 for index in range(0, len(schedule), 2)
            if schedule[index]["condition"] == "control"),
        "treatment_first_pairs": sum(
            1 for index in range(0, len(schedule), 2)
            if schedule[index]["condition"] == "treatment"),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tasks", required=True, type=Path,
                        help="operator-only task identity manifest JSON")
    parser.add_argument("--lock", required=True, type=Path,
                        help="run-lock JSON with pinned model, binary, skill, harness, and shared profile")
    parser.add_argument("--output", required=True, type=Path,
                        help="new path for the frozen trial schedule")
    args = parser.parse_args()
    try:
        result = freeze(args.tasks, args.lock, args.output)
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"schedule freeze failed: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
