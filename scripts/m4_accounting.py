#!/usr/bin/env python3
"""Validate complete M4 trial accounting and emit a compact deterministic report."""

from __future__ import annotations

import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
from typing import Any


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "evals" / "m4"
CONFIG = ROOT / "accounting.json"
STAGES = ("compilation", "update", "retrieval", "agent", "verification")
STAGE_STATUSES = {"success", "failed", "timeout", "not_applicable"}
STAGE_METRICS = ("wall_ms", "cpu_ms", "peak_rss_bytes", "input_bytes", "output_bytes")
OUTCOMES = {"completed", "failed", "timeout", "abstained"}
TOOL_STATUSES = {"success", "failed", "timeout"}
USAGE_FIELDS = ("input_tokens", "output_tokens", "cached_input_tokens", "reasoning_tokens")
CACHE_FIELDS = ("requested", "supplier_cache_before", "supplier_cache_after", "os_cache")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"{path.name} must contain an object")
    return value


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def validate_config(path: Path = CONFIG) -> dict[str, Any]:
    value = load_json(path)
    require(value == {
        "version": "repoctx.m4.accounting/v1alpha1",
        "stages": list(STAGES),
        "stage_statuses": ["success", "failed", "timeout", "not_applicable"],
        "stage_metrics": list(STAGE_METRICS),
        "stage_boundaries": {
            "compilation": "initial full repository compilation only",
            "update": "incremental refresh or warm preparation after initial compilation",
            "retrieval": "condition generation excluding compilation and update",
            "agent": "model and harness execution after condition delivery",
            "verification": "evaluator scoring and executable checks after agent execution",
        },
        "trial_metrics": ["wall_ms"],
        "trial_outcomes": ["completed", "failed", "timeout", "abstained"],
        "tool_statuses": ["success", "failed", "timeout"],
        "normalized_usage": list(USAGE_FIELDS),
        "additive_usage": ["input_tokens", "output_tokens"],
        "subset_usage": {"cached_input_tokens": "input_tokens",
                         "reasoning_tokens": "output_tokens"},
        "group_by": ["track", "condition", "cache_state"],
        "aggregation_note": "trial wall time is end-to-end; stage and tool-call times may be nested and are never added to it",
        "null_measurement": "not reported by the harness or provider; never coerce to zero",
    }, "accounting contract drifted")
    return value


def nullable_count(value: Any, field: str) -> None:
    require(value is None or (type(value) is int and value >= 0),
            f"{field} must be a nonnegative integer or null")


def validate_stage(name: str, value: Any) -> None:
    require(isinstance(value, dict) and set(value) == {"status", *STAGE_METRICS},
            f"invalid {name} stage fields")
    require(value["status"] in STAGE_STATUSES, f"invalid {name} status")
    for metric in STAGE_METRICS:
        nullable_count(value[metric], f"{name}.{metric}")
    if value["status"] == "not_applicable":
        require(all(value[metric] is None for metric in STAGE_METRICS),
                f"not-applicable {name} metrics must be null")
    else:
        require(value["wall_ms"] is not None, f"{name}.wall_ms is required")


def validate_cache(schedule: dict[str, Any], value: Any) -> None:
    require(isinstance(value, dict) and set(value) == set(CACHE_FIELDS),
            "invalid cache observation fields")
    require(value["requested"] == schedule["cache_state"], "cache stratum does not match schedule")
    require(value["os_cache"] in {"observed_cold", "observed_warm", "unknown"},
            "invalid OS-cache observation")
    if schedule["condition"] == "ordinary_tools":
        require(value["supplier_cache_before"] == "not_applicable" and
                value["supplier_cache_after"] == "not_applicable",
                "ordinary-tools supplier cache must be not_applicable")
    elif schedule["cache_state"] == "cold":
        require(value["supplier_cache_before"] == "empty" and
                value["supplier_cache_after"] in {"populated", "failed"},
                "invalid cold supplier-cache observation")
    else:
        require(value["supplier_cache_before"] == "prepared" and
                value["supplier_cache_after"] in {"reused", "failed"},
                "invalid warm supplier-cache observation")


def validate_usage(value: Any) -> None:
    require(isinstance(value, dict) and set(value) == {"raw", "normalized"},
            "invalid provider usage fields")
    require(isinstance(value["raw"], dict), "raw provider usage must be an object")
    normalized = value["normalized"]
    require(isinstance(normalized, dict) and set(normalized) == set(USAGE_FIELDS),
            "invalid normalized usage fields")
    for field in USAGE_FIELDS:
        nullable_count(normalized[field], f"provider_usage.normalized.{field}")
    pairs = (("cached_input_tokens", "input_tokens"), ("reasoning_tokens", "output_tokens"))
    for subset, parent in pairs:
        if normalized[subset] is not None:
            require(normalized[parent] is not None and normalized[subset] <= normalized[parent],
                    f"{subset} must be a subset of {parent}")


def validate_tools(schedule: dict[str, Any], value: Any) -> None:
    require(isinstance(value, list), "tool_calls must be an array")
    allowed = [] if schedule["permission_profile"] == "evidence_only" else ["repository_shell"]
    require(schedule["permission_profile"] in {"evidence_only", "matched_repository"},
            "unknown permission profile")
    for index, call in enumerate(value, 1):
        require(isinstance(call, dict) and set(call) ==
                {"sequence", "tool", "status", "wall_ms", "input_bytes", "output_bytes"},
                "invalid tool-call fields")
        require(call["sequence"] == index, "tool-call sequence must be contiguous")
        require(call["tool"] in allowed, "tool call violates the permission profile")
        require(call["status"] in TOOL_STATUSES, "invalid tool-call status")
        for field in ("wall_ms", "input_bytes", "output_bytes"):
            require(type(call[field]) is int and call[field] >= 0,
                    f"tool call {field} must be nonnegative")


def validate_record(schedule: dict[str, Any], value: Any) -> None:
    expected = {"version", "trial_id", "wall_ms", "cache", "outcome", "stages",
                "tool_trace_complete", "tool_calls", "provider_usage"}
    require(isinstance(value, dict) and set(value) == expected, "invalid trial accounting fields")
    require(value["version"] == "repoctx.m4.trial-accounting/v1alpha1",
            "unsupported trial accounting version")
    require(value["trial_id"] == schedule["id"], "trial ID does not match schedule")
    require(type(value["wall_ms"]) is int and value["wall_ms"] >= 0,
            "trial wall_ms must be nonnegative")
    require(value["outcome"] in OUTCOMES, "invalid trial outcome")
    validate_cache(schedule, value["cache"])
    require(isinstance(value["stages"], dict) and set(value["stages"]) == set(STAGES),
            "trial stages are missing or unknown")
    for stage in STAGES:
        validate_stage(stage, value["stages"][stage])
    require(value["tool_trace_complete"] is True, "tool trace must be complete")
    validate_tools(schedule, value["tool_calls"])
    validate_usage(value["provider_usage"])
    statuses = [stage["status"] for stage in value["stages"].values()]
    tool_statuses = [call["status"] for call in value["tool_calls"]]
    if value["outcome"] == "failed":
        require("failed" in statuses or "failed" in tool_statuses,
                "failed outcome has no recorded failure")
    if value["outcome"] == "timeout":
        require("timeout" in statuses or "timeout" in tool_statuses,
                "timeout outcome has no recorded timeout")


def read_records(path: Path) -> list[dict[str, Any]]:
    records = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        require(bool(line.strip()), f"blank accounting record at line {line_number}")
        value = json.loads(line)
        require(isinstance(value, dict), f"line {line_number} must contain an object")
        records.append(value)
    return records


def measured(values: list[Any], *, maximum: bool = False) -> dict[str, Any]:
    observed = [value for value in values if value is not None]
    return {"value": (None if not observed else max(observed) if maximum else sum(observed)),
            "observed": len(observed), "missing": len(values) - len(observed)}


def summarize(records: list[dict[str, Any]]) -> dict[str, Any]:
    stages = {}
    for stage in STAGES:
        values = [record["stages"][stage] for record in records]
        stages[stage] = {
            "statuses": dict(sorted(Counter(value["status"] for value in values).items())),
            "wall_ms_sum": measured([value["wall_ms"] for value in values]),
            "cpu_ms_sum": measured([value["cpu_ms"] for value in values]),
            "peak_rss_bytes_max": measured([value["peak_rss_bytes"] for value in values], maximum=True),
            "input_bytes_sum": measured([value["input_bytes"] for value in values]),
            "output_bytes_sum": measured([value["output_bytes"] for value in values]),
        }
    calls = [call for record in records for call in record["tool_calls"]]
    tools = {
        "calls": len(calls),
        "statuses": dict(sorted(Counter(call["status"] for call in calls).items())),
        "wall_ms_sum": sum(call["wall_ms"] for call in calls),
        "input_bytes_sum": sum(call["input_bytes"] for call in calls),
        "output_bytes_sum": sum(call["output_bytes"] for call in calls),
    }
    normalized = [record["provider_usage"]["normalized"] for record in records]
    usage = {field: measured([value[field] for value in normalized]) for field in USAGE_FIELDS}
    complete = [value for value in normalized
                if value["input_tokens"] is not None and value["output_tokens"] is not None]
    usage["input_plus_output_tokens"] = {
        "value": (sum(value["input_tokens"] + value["output_tokens"] for value in complete)
                  if complete else None),
        "observed": len(complete), "missing": len(normalized) - len(complete),
    }
    return {
        "trials": len(records),
        "wall_ms_sum": sum(record["wall_ms"] for record in records),
        "outcomes": dict(sorted(Counter(record["outcome"] for record in records).items())),
        "stages": stages,
        "tools": tools,
        "usage": usage,
    }


def build_report(lock_path: Path, lock: dict[str, Any], records: list[dict[str, Any]]) -> dict[str, Any]:
    validate_config()
    require(lock.get("version") == "repoctx.m4.run-lock/v1alpha1", "unsupported run lock")
    schedule = lock.get("schedule")
    require(isinstance(schedule, list) and schedule, "run lock has no schedule")
    indexed = {}
    for trial in schedule:
        require(isinstance(trial, dict) and isinstance(trial.get("id"), str) and
                trial["id"] not in indexed, "invalid or duplicate scheduled trial")
        for field in ("track", "condition", "cache_state", "permission_profile"):
            require(isinstance(trial.get(field), str), f"scheduled trial lacks {field}")
        indexed[trial["id"]] = trial
    supplied = {}
    for record in records:
        trial_id = record.get("trial_id")
        require(trial_id in indexed and trial_id not in supplied, "unknown or duplicate trial record")
        validate_record(indexed[trial_id], record)
        supplied[trial_id] = record
    require(set(supplied) == set(indexed), "accounting is incomplete for the run schedule")

    ordered = [supplied[trial["id"]] for trial in schedule]
    groups = []
    keys = sorted({(trial["track"], trial["condition"], trial["cache_state"])
                   for trial in schedule})
    for track, condition, cache_state in keys:
        selected = [supplied[trial["id"]] for trial in schedule
                    if (trial["track"], trial["condition"], trial["cache_state"]) ==
                    (track, condition, cache_state)]
        groups.append({"track": track, "condition": condition, "cache_state": cache_state,
                       "summary": summarize(selected)})
    return {
        "version": "repoctx.m4.accounting-report/v1alpha1",
        "run_lock_sha256": digest(lock_path),
        "accounting_contract_sha256": digest(CONFIG),
        "summary": summarize(ordered),
        "groups": groups,
        "trials": ordered,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["validate", "report"])
    parser.add_argument("--run-lock", type=Path)
    parser.add_argument("--records", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.action == "validate":
        value = validate_config()
        print(json.dumps({"ok": True, "stages": value["stages"],
                          "group_by": value["group_by"]}, sort_keys=True))
        return 0
    if not args.run_lock or not args.records or not args.output:
        parser.error("report requires --run-lock, --records, and --output")
    require(not args.output.exists(), "output already exists")
    try:
        args.output.resolve().relative_to(PROJECT)
    except ValueError:
        pass
    else:
        raise ValueError("accounting reports must be written outside the indexed repository")
    report = build_report(args.run_lock, load_json(args.run_lock), read_records(args.records))
    args.output.write_text(json.dumps(report, ensure_ascii=False, separators=(",", ":")) + "\n",
                           encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
