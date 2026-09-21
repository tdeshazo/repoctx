#!/usr/bin/env python3
"""Validate paired persisted-evidence trials and report complete lifecycle costs."""

from __future__ import annotations

import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
from typing import Any


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "evals" / "m4"
CONFIG = ROOT / "persistence.json"
MANIFEST = ROOT / "manifest.json"
PROTOCOL = ROOT / "protocol.json"
CONDITIONS = ("inline", "file_reference")
SCENARIOS = {"recovery", "source_mutation", "missing_handle", "expired_handle"}
ERRORS = {"none", "missing", "expired", "stale", "not_applicable"}
READ_STATUSES = {"success", "missing", "expired", "stale", "failed", "timeout"}
USAGE_FIELDS = ("input_tokens", "output_tokens", "cached_input_tokens", "reasoning_tokens")
CONTROL_FIELDS = ("task_sha256", "permission_profile", "budgets_sha256",
                  "model_profile_sha256", "compaction_policy_sha256")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"{path.name} must contain an object")
    return value


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def value_digest(value: object) -> str:
    encoded = json.dumps(value, ensure_ascii=False, sort_keys=True,
                         separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()


def validate_config(path: Path = CONFIG) -> dict[str, Any]:
    value = load_json(path)
    require(value == {
        "version": "repoctx.m4.persistence/v1alpha1",
        "conditions": list(CONDITIONS),
        "minimum_quality_pairs": 30,
        "required_scenarios": ["recovery", "source_mutation", "missing_handle",
                               "expired_handle"],
        "quality_task_outcome": "answerable",
        "resilience_tasks": {
            "source_mutation": "edge-contradictory-timeout",
            "missing_handle": "edge-no-answer-blockchain",
            "expired_handle": "edge-denied-admin-token",
        },
        "compaction_policy": {
            "remove_prior_evidence": True,
            "retain_checkpoint_only": True,
            "decisive_detail_in_checkpoint": False,
        },
        "matched_controls": list(CONTROL_FIELDS),
        "measurements": ["verified_task_success", "detail_recovered",
                         "stale_evidence_rejected", "reference_error", "reads",
                         "repeated_evidence_bytes", "reference_bytes", "preview_bytes",
                         "checkpoint_bytes", "provider_usage", "wall_ms",
                         "storage_byte_milliseconds"],
        "quality_metric": "verified_task_success_absolute_delta",
        "efficiency_metrics": ["input_plus_output_tokens_relative_reduction",
                               "tool_calls_relative_reduction",
                               "trial_wall_ms_relative_reduction"],
        "claim_policy": (
            "unassessed until a locked held-out run meets the M4-05 sample, quality, "
            "uncertainty, and practical-threshold rules"
        ),
    }, "persistence evaluation contract drifted")
    return value


def task_records(path: Path = MANIFEST) -> dict[str, dict[str, Any]]:
    manifest = load_json(path)
    require(manifest.get("version") == "repoctx.m4.tasks/v1alpha1",
            "unsupported task manifest")
    tasks = manifest.get("tasks")
    require(isinstance(tasks, list), "task manifest lacks tasks")
    indexed = {item.get("id"): item for item in tasks if isinstance(item, dict)}
    require(len(indexed) == len(tasks) and all(isinstance(item, str) for item in indexed),
            "task manifest has invalid IDs")
    return indexed


def task_identities(path: Path = MANIFEST) -> dict[str, str]:
    return {identifier: value_digest(record)
            for identifier, record in task_records(path).items()}


def scenario_schedule(config: dict[str, Any], path: Path = MANIFEST) -> dict[str, str]:
    tasks = task_records(path)
    resilience = {task: scenario for scenario, task in config["resilience_tasks"].items()}
    require(len(resilience) == len(config["resilience_tasks"]) and
            all(task in tasks for task in resilience), "invalid resilience task schedule")
    schedule = {}
    for identifier, task in tasks.items():
        if task.get("outcome") == config["quality_task_outcome"]:
            schedule[identifier] = "recovery"
        else:
            require(identifier in resilience, "edge task lacks a resilience scenario")
            schedule[identifier] = resilience[identifier]
    return schedule


def control_identities(config: dict[str, Any], protocol_path: Path = PROTOCOL) -> dict[str, str]:
    protocol = load_json(protocol_path)
    require(protocol.get("version") == "repoctx.m4.protocol/v1alpha1",
            "unsupported M4 protocol")
    return {
        "budgets_sha256": value_digest(protocol.get("budgets")),
        "model_profile_sha256": value_digest(protocol.get("model_profile")),
        "compaction_policy_sha256": value_digest(config["compaction_policy"]),
    }


def nonnegative(value: Any, field: str) -> int:
    require(type(value) is int and value >= 0, f"{field} must be a nonnegative integer")
    return value


def sha256(value: Any, field: str) -> str:
    require(isinstance(value, str) and len(value) == 64 and
            all(character in "0123456789abcdef" for character in value),
            f"invalid {field}")
    return value


def validate_controls(value: Any) -> None:
    require(isinstance(value, dict) and set(value) == set(CONTROL_FIELDS),
            "invalid matched controls")
    sha256(value["task_sha256"], "task identity")
    sha256(value["budgets_sha256"], "budget identity")
    sha256(value["model_profile_sha256"], "model profile identity")
    sha256(value["compaction_policy_sha256"], "compaction policy identity")
    require(isinstance(value["permission_profile"], str) and value["permission_profile"],
            "invalid permission profile")


def validate_usage(value: Any) -> None:
    require(isinstance(value, dict) and set(value) == {"raw", "normalized"} and
            isinstance(value["raw"], dict), "invalid provider usage")
    normalized = value["normalized"]
    require(isinstance(normalized, dict) and set(normalized) == set(USAGE_FIELDS),
            "invalid normalized provider usage")
    for field in USAGE_FIELDS:
        item = normalized[field]
        require(item is None or (type(item) is int and item >= 0),
                f"invalid provider usage {field}")
    if normalized["cached_input_tokens"] is not None and normalized["input_tokens"] is not None:
        require(normalized["cached_input_tokens"] <= normalized["input_tokens"],
                "cached input tokens exceed input tokens")
    if normalized["reasoning_tokens"] is not None and normalized["output_tokens"] is not None:
        require(normalized["reasoning_tokens"] <= normalized["output_tokens"],
                "reasoning tokens exceed output tokens")


def validate_reads(value: Any, condition: str) -> None:
    require(isinstance(value, list) and value, "reads must contain the follow-up retrieval")
    for sequence, read in enumerate(value, 1):
        fields = {"sequence", "operation", "status", "requested_bytes", "source_bytes",
                  "response_bytes", "repeated_evidence_bytes", "wall_ms"}
        require(isinstance(read, dict) and set(read) == fields, "invalid read record")
        require(read["sequence"] == sequence, "read sequence is incomplete")
        expected_operation = "context" if condition == "inline" else "read_context"
        require(read["operation"] == expected_operation, "read operation violates condition")
        require(read["status"] in READ_STATUSES, "invalid read status")
        for field in fields - {"sequence", "operation", "status"}:
            nonnegative(read[field], f"read.{field}")
        require(read["source_bytes"] <= read["requested_bytes"],
                "read returned more source bytes than requested")
        require(read["repeated_evidence_bytes"] <= read["source_bytes"],
                "repeated evidence exceeds read bytes")


def validate_record(record: Any, known_tasks: dict[str, str],
                    identities: dict[str, str]) -> None:
    fields = {
        "version", "run_lock_sha256", "pair_id", "task_id", "condition", "scenario",
        "controls", "compaction", "pre_resume", "initial_delivery", "read_trace_complete",
        "reads", "outcome", "provider_usage", "tool_calls", "wall_ms", "storage",
    }
    require(isinstance(record, dict) and set(record) == fields,
            "invalid persistence trial fields")
    require(record["version"] == "repoctx.m4.persistence-trial/v1alpha1",
            "unsupported persistence trial")
    sha256(record["run_lock_sha256"], "run lock identity")
    require(isinstance(record["pair_id"], str) and record["pair_id"], "invalid pair ID")
    require(record["task_id"] in known_tasks, "unknown held-out task")
    require(record["condition"] in CONDITIONS, "invalid persistence condition")
    require(record["scenario"] in SCENARIOS, "invalid persistence scenario")
    validate_controls(record["controls"])
    require(record["controls"]["task_sha256"] == known_tasks[record["task_id"]],
            "task identity does not match the manifest")
    require(record["controls"]["permission_profile"] == "matched_repository",
            "persistence trials require the matched permission profile")
    for field, expected in identities.items():
        require(record["controls"][field] == expected, f"{field} drifted")

    compaction = record["compaction"]
    require(isinstance(compaction, dict) and set(compaction) == {
        "performed", "prior_evidence_removed", "decisive_detail_present", "checkpoint_bytes"
    } and compaction["performed"] is True and compaction["prior_evidence_removed"] is True and
            compaction["decisive_detail_present"] is False,
            "trial did not apply the matched compaction policy")
    nonnegative(compaction["checkpoint_bytes"], "checkpoint bytes")
    pre_resume = record["pre_resume"]
    require(isinstance(pre_resume, dict) and set(pre_resume) == {
        "source_mutated", "handle_state"
    }, "invalid pre-resume state")
    require(type(pre_resume["source_mutated"]) is bool, "invalid source mutation state")
    expected_mutation = record["scenario"] == "source_mutation"
    require(pre_resume["source_mutated"] == expected_mutation,
            "source mutation scenario was not reproduced")
    if record["condition"] == "inline":
        expected_handle = "not_applicable"
    else:
        expected_handle = {"missing_handle": "missing", "expired_handle": "expired"}.get(
            record["scenario"], "available"
        )
    require(pre_resume["handle_state"] == expected_handle,
            "reference state scenario was not reproduced")

    initial = record["initial_delivery"]
    require(isinstance(initial, dict) and set(initial) == {
        "delivery_bytes", "bundle_bytes", "reference_bytes", "preview_bytes"
    }, "invalid initial delivery accounting")
    for field, item in initial.items():
        nonnegative(item, f"initial_delivery.{field}")
    require(initial["delivery_bytes"] >= initial["reference_bytes"] + initial["preview_bytes"],
            "initial delivery components exceed delivery bytes")
    if record["condition"] == "inline":
        require(initial["reference_bytes"] == 0 and initial["preview_bytes"] == 0 and
                initial["delivery_bytes"] == initial["bundle_bytes"],
                "inline delivery accounting is inconsistent")
    else:
        require(initial["reference_bytes"] > 0 and initial["bundle_bytes"] > 0,
                "file-reference delivery accounting is incomplete")

    require(record["read_trace_complete"] is True, "subsequent read trace is incomplete")
    validate_reads(record["reads"], record["condition"])
    outcome = record["outcome"]
    require(isinstance(outcome, dict) and set(outcome) == {
        "verified_task_success", "detail_recovered", "stale_evidence_rejected",
        "reference_error", "evidence_used_after_rejection"
    }, "invalid persistence outcome")
    for field in ("verified_task_success", "detail_recovered", "stale_evidence_rejected",
                  "evidence_used_after_rejection"):
        require(type(outcome[field]) is bool, f"invalid outcome {field}")
    require(outcome["reference_error"] in ERRORS, "invalid reference error")
    if record["condition"] == "inline":
        require(outcome["reference_error"] == "not_applicable",
                "inline trial cannot report a reference error")
    validate_usage(record["provider_usage"])
    nonnegative(record["tool_calls"], "tool calls")
    require(record["tool_calls"] >= len(record["reads"]),
            "tool-call count omits a subsequent read")
    nonnegative(record["wall_ms"], "trial wall time")
    storage = record["storage"]
    require(isinstance(storage, dict) and set(storage) == {
        "peak_bytes", "retained_ms", "byte_milliseconds"
    }, "invalid storage accounting")
    for field, item in storage.items():
        nonnegative(item, f"storage.{field}")
    if record["condition"] == "inline":
        require(all(item == 0 for item in storage.values()),
                "inline trial cannot report persisted storage")
    else:
        require(storage["peak_bytes"] >= initial["bundle_bytes"],
                "peak storage omits the persisted bundle")


def read_records(path: Path) -> list[dict[str, Any]]:
    records = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as error:
            raise ValueError(f"invalid JSONL at line {line_number}: {error.msg}") from None
        require(isinstance(value, dict), f"line {line_number} must contain an object")
        records.append(value)
    return records


def expected_rejection(record: dict[str, Any]) -> bool:
    if record["condition"] != "file_reference":
        return False
    outcome = record["outcome"]
    expected = {"source_mutation": "stale", "missing_handle": "missing",
                "expired_handle": "expired"}.get(record["scenario"])
    return (expected is not None and outcome["reference_error"] == expected and
            not outcome["evidence_used_after_rejection"] and
            (record["scenario"] != "source_mutation" or outcome["stale_evidence_rejected"]))


def summarize(records: list[dict[str, Any]]) -> dict[str, Any]:
    usage = {field: {"value": 0, "observed": 0, "missing": 0} for field in USAGE_FIELDS}
    usage["input_plus_output_tokens"] = {"value": 0, "observed": 0, "missing": 0}
    summary = {
        "trials": len(records),
        "verified_task_successes": sum(item["outcome"]["verified_task_success"]
                                       for item in records),
        "detail_recoveries": sum(item["outcome"]["detail_recovered"] for item in records),
        "reference_errors": dict(sorted(Counter(
            item["outcome"]["reference_error"] for item in records
        ).items())),
        "delivery_bytes": sum(item["initial_delivery"]["delivery_bytes"] for item in records),
        "bundle_bytes": sum(item["initial_delivery"]["bundle_bytes"] for item in records),
        "reference_bytes": sum(item["initial_delivery"]["reference_bytes"] for item in records),
        "preview_bytes": sum(item["initial_delivery"]["preview_bytes"] for item in records),
        "checkpoint_bytes": sum(item["compaction"]["checkpoint_bytes"] for item in records),
        "reads": {
            "calls": sum(len(item["reads"]) for item in records),
            "source_bytes": sum(read["source_bytes"] for item in records for read in item["reads"]),
            "response_bytes": sum(read["response_bytes"] for item in records
                                  for read in item["reads"]),
            "repeated_evidence_bytes": sum(read["repeated_evidence_bytes"]
                                           for item in records for read in item["reads"]),
        },
        "tool_calls": sum(item["tool_calls"] for item in records),
        "wall_ms": sum(item["wall_ms"] for item in records),
        "storage": {
            "peak_bytes_sum": sum(item["storage"]["peak_bytes"] for item in records),
            "retained_ms_sum": sum(item["storage"]["retained_ms"] for item in records),
            "byte_milliseconds": sum(item["storage"]["byte_milliseconds"] for item in records),
        },
        "provider_usage": usage,
    }
    for record in records:
        normalized = record["provider_usage"]["normalized"]
        for field in USAGE_FIELDS:
            value = normalized[field]
            if value is None:
                usage[field]["missing"] += 1
            else:
                usage[field]["value"] += value
                usage[field]["observed"] += 1
        if normalized["input_tokens"] is None or normalized["output_tokens"] is None:
            usage["input_plus_output_tokens"]["missing"] += 1
        else:
            usage["input_plus_output_tokens"]["value"] += (
                normalized["input_tokens"] + normalized["output_tokens"]
            )
            usage["input_plus_output_tokens"]["observed"] += 1
    for field in (*USAGE_FIELDS, "input_plus_output_tokens"):
        if usage[field]["observed"] == 0:
            usage[field]["value"] = None
    return summary


def build_report(records: list[dict[str, Any]], *, config_path: Path = CONFIG,
                 manifest_path: Path = MANIFEST, protocol_path: Path = PROTOCOL) -> dict[str, Any]:
    config = validate_config(config_path)
    known_tasks = task_identities(manifest_path)
    expected_scenarios = scenario_schedule(config, manifest_path)
    identities = control_identities(config, protocol_path)
    require(records, "persistence trial set is empty")
    pairs: dict[str, list[dict[str, Any]]] = {}
    for record in records:
        validate_record(record, known_tasks, identities)
        pairs.setdefault(record["pair_id"], []).append(record)
    run_locks = {record["run_lock_sha256"] for record in records}
    require(len(run_locks) == 1, "persistence records mix run locks")
    seen_tasks = set()
    scenarios = set()
    for pair_id, pair in pairs.items():
        require(len(pair) == 2 and {item["condition"] for item in pair} == set(CONDITIONS),
                f"pair {pair_id} is incomplete")
        require(len({item["task_id"] for item in pair}) == 1 and
                len({item["scenario"] for item in pair}) == 1,
                f"pair {pair_id} does not describe one matched task")
        require(pair[0]["controls"] == pair[1]["controls"],
                f"pair {pair_id} controls do not match")
        task = pair[0]["task_id"]
        require(task not in seen_tasks, "held-out task appears in multiple pairs")
        require(pair[0]["scenario"] == expected_scenarios[task],
                f"pair {pair_id} changed the frozen scenario schedule")
        seen_tasks.add(task)
        scenarios.add(pair[0]["scenario"])
    quality_pairs = sum(expected_scenarios[task] == "recovery" for task in seen_tasks)
    require(quality_pairs >= config["minimum_quality_pairs"],
            "persistence evaluation is underpowered")
    require(seen_tasks == set(known_tasks), "persistence task schedule is incomplete")
    require(scenarios == set(config["required_scenarios"]),
            "persistence scenario coverage is incomplete")

    ordered = sorted(records, key=lambda item: (item["pair_id"], item["condition"]))
    records_sha256 = value_digest(ordered)
    by_condition = {condition: summarize([item for item in ordered
                                          if item["condition"] == condition])
                    for condition in CONDITIONS}
    resilience = {}
    for scenario in sorted(SCENARIOS):
        selected = [item for item in ordered if item["scenario"] == scenario and
                    item["condition"] == "file_reference"]
        resilience[scenario] = {
            "trials": len(selected),
            "detail_recovered": sum(item["outcome"]["detail_recovered"] for item in selected),
            "expected_rejections": sum(expected_rejection(item) for item in selected),
        }
    return {
        "version": "repoctx.m4.persistence-report/v1alpha1",
        "run_lock_sha256": next(iter(run_locks)),
        "persistence_contract_sha256": digest(config_path),
        "task_manifest_sha256": digest(manifest_path),
        "protocol_sha256": digest(protocol_path),
        "records_sha256": records_sha256,
        "paired_tasks": len(seen_tasks),
        "quality_paired_tasks": quality_pairs,
        "claim_status": "experimental_unassessed",
        "conditions": by_condition,
        "resilience": resilience,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["validate", "report"])
    parser.add_argument("--records", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    config = validate_config()
    if args.action == "validate":
        print(json.dumps({"ok": True, "conditions": config["conditions"],
                          "minimum_quality_pairs": config["minimum_quality_pairs"],
                          "claim_policy": config["claim_policy"]}, sort_keys=True))
        return 0
    if not args.records or not args.output:
        parser.error("report requires --records and --output")
    require(not args.output.exists(), "output already exists")
    try:
        args.output.resolve().relative_to(PROJECT)
    except ValueError:
        pass
    else:
        raise ValueError("persistence reports must be written outside the indexed repository")
    report = build_report(read_records(args.records))
    args.output.write_text(json.dumps(report, ensure_ascii=False, separators=(",", ":")) + "\n",
                           encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
