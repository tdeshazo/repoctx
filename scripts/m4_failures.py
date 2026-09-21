#!/usr/bin/env python3
"""Validate trace-backed context failures and rank their contributing behavior."""

from __future__ import annotations

import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
from typing import Any


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "evals" / "m4"
CONFIG = ROOT / "failures.json"
CASES = ROOT / "failure-cases.json"
TAGS = ("missing", "under_retrieved", "over_retrieved", "buried")
LAYERS = ("inventory", "discovery", "selection", "budget")
OMISSIONS = {"overview_limit", "result_limit", "output_limit", "source_limit",
             "read_limit"}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(value, dict), f"{path.name} must contain an object")
    return value


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def sha256(value: Any, field: str, *, length: int = 64) -> str:
    require(isinstance(value, str) and len(value) == length and
            all(character in "0123456789abcdef" for character in value),
            f"invalid {field}")
    return value


def validate_config(path: Path = CONFIG) -> dict[str, Any]:
    value = load_json(path)
    require(value == {
        "version": "repoctx.m4.failures/v1alpha1",
        "tags": {
            "missing": "needed evidence is absent from available inputs",
            "under_retrieved": (
                "available evidence was found but omitted, clipped, or insufficiently expanded"
            ),
            "over_retrieved": (
                "irrelevant selected evidence consumed a binding output budget"
            ),
            "buried": "available evidence was not locatable in the retained result",
        },
        "contributors": {
            "inventory": ["unavailable_input", "excluded_scope", "stale_inventory",
                          "noisy_generated_input"],
            "discovery": ["query_tokenization", "candidate_generation", "lexical_ranking"],
            "selection": ["window_size", "window_boundary", "expansion", "diversity",
                          "coalescing"],
            "budget": ["output_limit", "result_limit", "source_limit", "read_limit"],
        },
        "impact_weights": {"incorrect": 5, "failed": 4, "abstained": 3,
                           "recovered": 2, "diagnostic_only": 1},
        "evidence_loss_weights": {"complete": 3, "partial": 2, "none": 0},
        "tag_weights": {"missing": 4, "under_retrieved": 3,
                        "over_retrieved": 2, "buried": 1},
        "priority_formula": (
            "impact*100+evidence_loss*10+maximum_tag_weight; descending score then record ID"
        ),
        "multi_label": True,
    }, "failure diagnosis contract drifted")
    return value


def derive_tags(record: dict[str, Any]) -> list[str]:
    required = record["required_evidence"]
    trace = record["trace"]
    tags = []
    if not required["available"]:
        tags.append("missing")
    if required["available"] and trace["candidate_found"] and not trace["retained"]:
        tags.append("under_retrieved")
    if (trace["irrelevant_selected_bytes"] > 0 and
            "output_limit" in trace["omissions"]):
        tags.append("over_retrieved")
    if required["available"] and not trace["retained"]:
        tags.append("buried")
    return tags


def validate_record(record: Any, config: dict[str, Any]) -> None:
    fields = {"id", "source_revision", "operation", "arguments", "required_evidence",
              "trace", "tags", "contributors", "impact"}
    require(isinstance(record, dict) and set(record) == fields,
            "invalid failure record fields")
    require(isinstance(record["id"], str) and record["id"], "invalid failure record ID")
    sha256(record["source_revision"], "source revision", length=40)
    require(record["operation"] == "discover", "unsupported diagnosed operation")
    arguments = record["arguments"]
    require(isinstance(arguments, dict) and set(arguments) == {"query", "max_bytes", "deny"},
            "invalid replay arguments")
    require(isinstance(arguments["query"], str) and arguments["query"] and
            len(arguments["query"].encode("utf-8")) <= 8192, "invalid replay query")
    require(type(arguments["max_bytes"]) is int and 1024 <= arguments["max_bytes"] <= 1024 ** 2,
            "invalid replay output budget")
    require(isinstance(arguments["deny"], list) and
            all(isinstance(item, str) and item for item in arguments["deny"]),
            "invalid replay deny policy")

    required = record["required_evidence"]
    require(isinstance(required, dict) and set(required) == {
        "path", "marker_sha256", "available"
    }, "invalid required evidence")
    require(isinstance(required["path"], str) and required["path"] and
            not required["path"].startswith("/"), "invalid required evidence path")
    sha256(required["marker_sha256"], "required evidence marker")
    require(type(required["available"]) is bool, "invalid evidence availability")

    trace = record["trace"]
    trace_fields = {"candidate_found", "selected", "materialized", "retained",
                    "irrelevant_selected_bytes", "result_count", "incomplete", "omissions",
                    "observed"}
    require(isinstance(trace, dict) and set(trace) == trace_fields,
            "invalid supporting trace")
    for field in ("candidate_found", "selected", "materialized", "retained", "incomplete"):
        require(type(trace[field]) is bool, f"invalid trace {field}")
    require(not trace["selected"] or trace["candidate_found"],
            "selection requires a discovered candidate")
    require(not trace["materialized"] or trace["selected"],
            "materialization requires selection")
    require(not trace["retained"] or trace["materialized"],
            "retention requires materialization")
    for field in ("irrelevant_selected_bytes", "result_count"):
        require(type(trace[field]) is int and trace[field] >= 0, f"invalid trace {field}")
    require(isinstance(trace["omissions"], list) and len(trace["omissions"]) ==
            len(set(trace["omissions"])) and set(trace["omissions"]) <= OMISSIONS,
            "invalid trace omissions")
    require(trace["incomplete"] == bool(trace["omissions"]),
            "trace incompleteness disagrees with omissions")
    require(isinstance(trace["observed"], str) and trace["observed"] and
            len(trace["observed"].encode("utf-8")) <= 512, "invalid trace observation")

    require(isinstance(record["tags"], list) and len(record["tags"]) == len(set(record["tags"])) and
            set(record["tags"]) <= set(TAGS), "invalid failure tags")
    require(record["tags"] == derive_tags(record), "failure tags do not match trace facts")
    require(len(record["tags"]) > 0, "record does not describe a context failure")

    contributors = record["contributors"]
    require(isinstance(contributors, dict) and set(contributors) == set(LAYERS),
            "invalid failure contributors")
    contributor_count = 0
    for layer in LAYERS:
        values = contributors[layer]
        require(isinstance(values, list) and len(values) == len(set(values)) and
                set(values) <= set(config["contributors"][layer]),
                f"invalid {layer} contributors")
        contributor_count += len(values)
    require(contributor_count > 0, "failure lacks a contributing behavior")
    if "missing" in record["tags"]:
        require(bool(contributors["inventory"]), "missing evidence needs an inventory contributor")

    impact = record["impact"]
    require(isinstance(impact, dict) and set(impact) == {"outcome", "evidence_loss"},
            "invalid failure impact")
    require(impact["outcome"] in config["impact_weights"], "invalid failure outcome")
    require(impact["evidence_loss"] in config["evidence_loss_weights"],
            "invalid evidence loss")


def priority(record: dict[str, Any], config: dict[str, Any]) -> int:
    impact = config["impact_weights"][record["impact"]["outcome"]]
    loss = config["evidence_loss_weights"][record["impact"]["evidence_loss"]]
    tag = max(config["tag_weights"][item] for item in record["tags"])
    return impact * 100 + loss * 10 + tag


def load_cases(path: Path = CASES) -> list[dict[str, Any]]:
    value = load_json(path)
    require(set(value) == {"version", "records"} and
            value["version"] == "repoctx.m4.failure-cases/v1alpha1" and
            isinstance(value["records"], list), "invalid failure case envelope")
    return value["records"]


def build_report(records: list[dict[str, Any]], *, config_path: Path = CONFIG,
                 cases_path: Path | None = None) -> dict[str, Any]:
    config = validate_config(config_path)
    require(records, "failure record set is empty")
    identifiers = set()
    ranked = []
    contributor_groups: dict[str, dict[str, Any]] = {}
    for record in records:
        validate_record(record, config)
        require(record["id"] not in identifiers, "duplicate failure record ID")
        identifiers.add(record["id"])
        score = priority(record, config)
        ranked.append({"id": record["id"], "priority": score, "tags": record["tags"],
                       "impact": record["impact"], "contributors": record["contributors"]})
        for layer in LAYERS:
            for contributor in record["contributors"][layer]:
                key = f"{layer}:{contributor}"
                group = contributor_groups.setdefault(
                    key, {"contributor": key, "records": [], "max_priority": 0}
                )
                group["records"].append(record["id"])
                group["max_priority"] = max(group["max_priority"], score)
    ranked.sort(key=lambda item: (-item["priority"], item["id"]))
    contributors = sorted(contributor_groups.values(),
                          key=lambda item: (-item["max_priority"], -len(item["records"]),
                                            item["contributor"]))
    return {
        "version": "repoctx.m4.failure-report/v1alpha1",
        "failure_contract_sha256": digest(config_path),
        "failure_cases_sha256": digest(cases_path) if cases_path is not None else None,
        "records": len(records),
        "multi_label_records": sum(len(record["tags"]) > 1 for record in records),
        "tag_counts": dict(sorted(Counter(tag for record in records
                                           for tag in record["tags"]).items())),
        "ranked_failures": ranked,
        "ranked_contributors": contributors,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["validate", "report"])
    parser.add_argument("--records", type=Path, default=CASES)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    records = load_cases(args.records)
    report = build_report(records, cases_path=args.records)
    if args.action == "validate":
        print(json.dumps({"ok": True, "records": report["records"],
                          "tag_counts": report["tag_counts"],
                          "top_priority": report["ranked_failures"][0]["id"]},
                         sort_keys=True))
        return 0
    if not args.output:
        parser.error("report requires --output")
    require(not args.output.exists(), "output already exists")
    try:
        args.output.resolve().relative_to(PROJECT)
    except ValueError:
        pass
    else:
        raise ValueError("failure reports must be written outside the indexed repository")
    args.output.write_text(json.dumps(report, ensure_ascii=False, separators=(",", ":")) + "\n",
                           encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
