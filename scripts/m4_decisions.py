#!/usr/bin/env python3
"""Validate frozen M4 decision rules and classify precomputed paired intervals."""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path
from typing import Any, Optional


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "evals" / "m4"
CONFIG = ROOT / "decisions.json"
QUALITY_TOLERANCES = {
    "verified_task_success_absolute_delta": -0.05,
    "required_span_recall_absolute_delta": -0.05,
}
PRACTICAL_THRESHOLDS = {
    "input_plus_output_tokens_relative_reduction": 0.15,
    "tool_calls_relative_reduction": 0.20,
    "trial_wall_ms_relative_reduction": 0.15,
    "selected_evidence_bytes_relative_reduction": 0.15,
    "required_span_recall_absolute_improvement": 0.03,
}
EXPECTED_COMPARISONS = {
    "end_to_end_repoctx_graph_vs_ordinary": {
        "track": "end_to_end", "candidate": "repoctx_graph", "reference": "ordinary_tools",
        "eligibility": "all", "decision": "efficiency_with_quality_guard",
        "quality_metric": "verified_task_success_absolute_delta",
        "efficiency_metrics": ["input_plus_output_tokens_relative_reduction",
                               "tool_calls_relative_reduction",
                               "trial_wall_ms_relative_reduction"],
        "attribution": "repoctx_with_graph",
    },
    "evidence_repoctx_no_graph_vs_lexical": {
        "track": "evidence_only", "candidate": "repoctx_no_graph",
        "reference": "bounded_lexical", "eligibility": "all",
        "decision": "efficiency_with_quality_guard",
        "quality_metric": "required_span_recall_absolute_delta",
        "efficiency_metrics": ["selected_evidence_bytes_relative_reduction"],
        "attribution": "repoctx_selection_without_graph",
    },
    "evidence_graph_vs_no_graph": {
        "track": "evidence_only", "candidate": "repoctx_graph",
        "reference": "repoctx_no_graph", "eligibility": "all",
        "decision": "quality_superiority",
        "quality_metric": "required_span_recall_absolute_delta", "efficiency_metrics": [],
        "attribution": "repoctx_graph_expansion",
    },
    "artifact_aware_vs_repoctx_graph": {
        "track": "evidence_only", "candidate": "artifact_aware",
        "reference": "repoctx_graph", "eligibility": "declared_catalog",
        "decision": "descriptive_only",
        "quality_metric": "required_span_recall_absolute_delta", "efficiency_metrics": [],
        "attribution": "repository_declarations_and_harness_selection",
    },
    "human_oracle_upper_bound": {
        "track": "evidence_only", "candidate": "human_oracle",
        "reference": "repoctx_graph", "eligibility": "all",
        "decision": "descriptive_only",
        "quality_metric": "required_span_recall_absolute_delta", "efficiency_metrics": [],
        "attribution": "human_gold",
    },
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"{path.name} must contain an object")
    return value


def validate_config(path: Path = CONFIG) -> dict[str, Any]:
    value = load_json(path)
    require(value.get("version") == "repoctx.m4.decisions/v1alpha1",
            "unsupported decision-rule version")
    require(set(value) == {"version", "freeze", "uncertainty", "sample_rules",
                           "primary_metrics", "quality_tolerances", "practical_thresholds",
                           "claim_rule", "comparisons", "decision_states"},
            "invalid decision-rule fields")
    require(value["freeze"] == "before held-out trial execution or inspection",
            "decision rules are not frozen before evaluation")
    require(value["uncertainty"] == {
        "method": "paired_task_cluster_bootstrap", "confidence": 0.95,
        "resamples": 10000, "seed": 20260921, "cluster": "task_id",
        "stratify": ["track", "cache_state"], "interval": "percentile",
    }, "uncertainty procedure drifted")
    require(value["sample_rules"] == {
        "minimum_paired_tasks": 30, "maximum_missing_fraction": 0.05,
        "failures_and_timeouts": "score as task failure and retain measured costs",
        "abstentions": "score against the declared outcome; do not drop",
        "underpowered": "inconclusive; never equivalence or non-regression",
    }, "sample and missingness rules drifted")
    require(value["primary_metrics"] == {"end_to_end": "verified_task_success",
                                          "evidence_only": "required_span_recall"},
            "primary metrics drifted")
    require(value["quality_tolerances"] == QUALITY_TOLERANCES,
            "quality tolerances drifted")
    require(value["practical_thresholds"] == PRACTICAL_THRESHOLDS,
            "practical thresholds drifted")
    require(value["claim_rule"] ==
            "quality guard must pass; report each efficiency metric separately; overall support requires at least one supported efficiency metric and no inconclusive efficiency metric",
            "claim aggregation rule drifted")
    comparisons = value["comparisons"]
    require(isinstance(comparisons, list) and len(comparisons) == len(EXPECTED_COMPARISONS),
            "comparison set is incomplete")
    indexed = {}
    for comparison in comparisons:
        identifier = comparison.get("id")
        require(identifier in EXPECTED_COMPARISONS and identifier not in indexed,
                "unknown or duplicate comparison")
        expected = {"id": identifier, **EXPECTED_COMPARISONS[identifier]}
        require(comparison == expected, f"comparison {identifier} drifted")
        indexed[identifier] = comparison
    require(set(indexed) == set(EXPECTED_COMPARISONS), "comparison set is incomplete")
    require(value["decision_states"] == ["supported", "threshold_not_met", "quality_harm",
                                          "inconclusive", "inconclusive_underpowered",
                                          "inconclusive_missing", "descriptive_only"],
            "decision states drifted")
    return value


def validate_interval(name: str, value: Any) -> None:
    require(isinstance(value, dict) and set(value) == {"estimate", "lower", "upper"},
            f"invalid interval for {name}")
    numbers = [value[key] for key in ("lower", "estimate", "upper")]
    require(all(type(number) in {int, float} and math.isfinite(number) for number in numbers),
            f"non-finite interval for {name}")
    require(numbers[0] <= numbers[1] <= numbers[2], f"unordered interval for {name}")
    if "absolute_delta" in name:
        require(numbers[0] >= -1 and numbers[2] <= 1, f"absolute delta outside [-1, 1] for {name}")
    if "relative_reduction" in name:
        require(numbers[2] <= 1, f"relative reduction exceeds 1 for {name}")


def interval_state(value: dict[str, Any], threshold: float) -> str:
    if value["lower"] >= threshold:
        return "supported"
    if value["upper"] < threshold:
        return "threshold_not_met"
    return "inconclusive"


def assess(value: dict[str, Any], rules: Optional[dict[str, Any]] = None) -> dict[str, Any]:
    rules = rules or validate_config()
    require(isinstance(value, dict) and set(value) == {"version", "run_lock_sha256",
                                                       "accounting_report_sha256",
                                                       "scored_results_sha256", "comparison",
                                                       "eligible_tasks", "paired_tasks",
                                                       "missing_fraction", "uncertainty", "intervals"},
            "invalid decision input fields")
    require(value["version"] == "repoctx.m4.decision-input/v1alpha1",
            "unsupported decision input")
    indexed = {item["id"]: item for item in rules["comparisons"]}
    require(value["comparison"] in indexed, "unknown comparison")
    comparison = indexed[value["comparison"]]
    for field in ("run_lock_sha256", "accounting_report_sha256", "scored_results_sha256"):
        require(isinstance(value[field], str) and len(value[field]) == 64 and
                all(character in "0123456789abcdef" for character in value[field]),
                f"invalid {field}")
    for field in ("eligible_tasks", "paired_tasks"):
        require(type(value[field]) is int and value[field] >= 0, f"invalid {field}")
    require(value["paired_tasks"] <= value["eligible_tasks"], "paired tasks exceed eligibility")
    require(type(value["missing_fraction"]) in {int, float} and
            0 <= value["missing_fraction"] <= 1, "invalid missing fraction")
    expected_missing = (0 if value["eligible_tasks"] == 0 else
                        1 - value["paired_tasks"] / value["eligible_tasks"])
    require(math.isclose(value["missing_fraction"], expected_missing, abs_tol=1e-12),
            "missing fraction disagrees with task counts")
    expected_uncertainty = {key: rules["uncertainty"][key]
                            for key in ("method", "confidence", "resamples", "seed",
                                        "cluster", "stratify", "interval")}
    require(value["uncertainty"] == expected_uncertainty, "uncertainty procedure mismatch")
    required_metrics = {comparison["quality_metric"], *comparison["efficiency_metrics"]}
    require(isinstance(value["intervals"], dict) and set(value["intervals"]) == required_metrics,
            "decision input metrics do not match the comparison")
    for name, interval in value["intervals"].items():
        validate_interval(name, interval)

    base = {"version": "repoctx.m4.decision/v1alpha1", "comparison": comparison["id"],
            "attribution": comparison["attribution"], "paired_tasks": value["paired_tasks"],
            "sources": {field: value[field] for field in
                        ("run_lock_sha256", "accounting_report_sha256",
                         "scored_results_sha256")}}
    if comparison["decision"] == "descriptive_only":
        return {**base, "overall": "descriptive_only", "claims": {}}
    minimum = rules["sample_rules"]["minimum_paired_tasks"]
    if value["paired_tasks"] < minimum:
        return {**base, "overall": "inconclusive_underpowered", "claims": {}}
    if value["missing_fraction"] > rules["sample_rules"]["maximum_missing_fraction"]:
        return {**base, "overall": "inconclusive_missing", "claims": {}}

    quality = value["intervals"][comparison["quality_metric"]]
    if comparison["decision"] == "quality_superiority":
        state = interval_state(quality,
                               rules["practical_thresholds"]["required_span_recall_absolute_improvement"])
        return {**base, "overall": state,
                "claims": {comparison["quality_metric"]: state}}

    tolerance = rules["quality_tolerances"][comparison["quality_metric"]]
    if quality["upper"] < tolerance:
        quality_state = "quality_harm"
    elif quality["lower"] >= tolerance:
        quality_state = "supported"
    else:
        quality_state = "inconclusive"
    claims = {comparison["quality_metric"]: quality_state}
    for metric in comparison["efficiency_metrics"]:
        if quality_state == "quality_harm":
            claims[metric] = "quality_harm"
        elif quality_state != "supported":
            claims[metric] = "inconclusive"
        else:
            claims[metric] = interval_state(value["intervals"][metric],
                                             rules["practical_thresholds"][metric])
    efficiency_states = [claims[metric] for metric in comparison["efficiency_metrics"]]
    if quality_state == "quality_harm":
        overall = "quality_harm"
    elif quality_state != "supported" or "inconclusive" in efficiency_states:
        overall = "inconclusive"
    elif "supported" in efficiency_states:
        overall = "supported"
    else:
        overall = "threshold_not_met"
    return {**base, "overall": overall, "claims": claims}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["validate", "assess"])
    parser.add_argument("--input", type=Path)
    args = parser.parse_args()
    rules = validate_config()
    if args.action == "validate":
        print(json.dumps({"ok": True, "comparisons": [item["id"] for item in rules["comparisons"]],
                          "minimum_paired_tasks": rules["sample_rules"]["minimum_paired_tasks"]},
                         sort_keys=True))
        return 0
    if not args.input:
        parser.error("assess requires --input")
    print(json.dumps(assess(load_json(args.input), rules), sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
