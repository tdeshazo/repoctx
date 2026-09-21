#!/usr/bin/env python3
"""Validate and summarize the predeclared M5 complete-path profile."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import re
import statistics
from typing import Any


PROJECT = Path(__file__).resolve().parents[1]
CONFIG = PROJECT / "benchmarks" / "m5" / "profile.json"
HARNESSES = (
    PROJECT / "internal" / "benchfixture" / "repository.go",
    PROJECT / "pkg" / "compiler" / "compiler_bench_test.go",
    PROJECT / "pkg" / "agentctx" / "build_bench_test.go",
)
BENCHMARK = re.compile(
    r"^BenchmarkM5(?P<group>Compiler|Context)Path/files=(?P<files>\d+)/"
    r"(?P<stage>[a-z_]+)-\d+\s+\d+\s+(?P<metrics>.+)$"
)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError(message)


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_config(path: Path = CONFIG) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    require(value == {
        "version": "repoctx.m5.profile/v1alpha1",
        "sizes": [10, 100, 1000],
        "samples": 10,
        "benchtime": "3x",
        "workload": {
            "language": "go", "files_per_directory": 1, "symbols_per_file": 3,
            "calls_per_file": 1, "query": "Compute helper Record",
            "context_max_bytes": 32768, "consistency": "verified-local",
        },
        "stages": {
            "compiler": ["source_discovery", "parsing", "linking", "validation",
                         "serialization", "source_verification", "complete_compilation"],
            "context": ["ranking", "context_materialization"],
        },
        "atomic_stages": ["source_discovery", "parsing", "linking", "validation",
                          "serialization", "source_verification", "ranking"],
        "complete_path": ["complete_compilation", "serialization",
                          "context_materialization"],
        "target": {
            "applies_to_files": 1000, "warm_complete_path_p95_ratio_max": 0.5,
            "warm_allocated_bytes_ratio_max": 0.5,
            "incremental_artifact_to_canonical_ir_bytes_ratio_max": 1.25,
            "canonical_semantic_output": "identical", "source_freshness": "verified",
        },
    }, "M5 profile contract drifted")
    return value


def parse_metrics(text: str) -> dict[str, float]:
    fields = text.split()
    require(len(fields) % 2 == 0, "malformed benchmark metrics")
    metrics = {}
    for index in range(0, len(fields), 2):
        metrics[fields[index + 1]] = float(fields[index])
    for unit in ("ns/op", "B/op", "allocs/op", "files", "source-bytes",
                 "artifact-bytes"):
        require(unit in metrics, f"benchmark result lacks {unit}")
    return metrics


def parse_results(paths: list[Path], config: dict[str, Any]) -> tuple[dict[str, str], dict]:
    environment: dict[str, str] = {}
    observed: dict[tuple[int, str], list[dict[str, float]]] = {}
    expected_groups = {stage: group for group, stages in config["stages"].items()
                       for stage in stages}
    for path in paths:
        for line in path.read_text(encoding="utf-8").splitlines():
            for key in ("goos", "goarch", "cpu"):
                if line.startswith(key + ": "):
                    value = line.split(": ", 1)[1]
                    require(key not in environment or environment[key] == value,
                            f"benchmark environment disagrees on {key}")
                    environment[key] = value
            match = BENCHMARK.match(line)
            if not match:
                continue
            files = int(match.group("files"))
            stage = match.group("stage")
            group = match.group("group").lower()
            require(files in config["sizes"], f"unexpected benchmark size {files}")
            require(expected_groups.get(stage) == group, f"unexpected benchmark stage {stage}")
            metrics = parse_metrics(match.group("metrics"))
            require(metrics["files"] == files, "benchmark file metric disagrees with name")
            if group == "context":
                require("context-bytes" in metrics, "context benchmark lacks context-bytes")
            observed.setdefault((files, stage), []).append(metrics)
    require(set(environment) == {"goos", "goarch", "cpu"},
            "benchmark environment is incomplete")
    for files in config["sizes"]:
        for stage in expected_groups:
            count = len(observed.get((files, stage), []))
            require(count == config["samples"],
                    f"{files}/{stage} has {count} samples, want {config['samples']}")
        source_sizes = {item["source-bytes"] for stage in expected_groups
                        for item in observed[(files, stage)]}
        artifact_sizes = {item["artifact-bytes"] for stage in expected_groups
                          for item in observed[(files, stage)]}
        require(len(source_sizes) == 1 and len(artifact_sizes) == 1,
                f"{files}-file artifact metrics drifted between samples")
        context_sizes = {item["context-bytes"] for stage in config["stages"]["context"]
                         for item in observed[(files, stage)]}
        require(len(context_sizes) == 1,
                f"{files}-file context metrics drifted between samples")
    return environment, observed


def nearest_rank(values: list[float], percentile: float) -> float:
    ordered = sorted(values)
    index = max(0, int(len(ordered) * percentile + 0.999999) - 1)
    return ordered[index]


def summary(samples: list[dict[str, float]]) -> dict[str, int]:
    return {
        "median_ns_per_op": round(statistics.median(item["ns/op"] for item in samples)),
        "p95_ns_per_op": round(nearest_rank([item["ns/op"] for item in samples], 0.95)),
        "median_allocated_bytes_per_op": round(statistics.median(
            item["B/op"] for item in samples)),
        "median_allocations_per_op": round(statistics.median(
            item["allocs/op"] for item in samples)),
    }


def build_report(paths: list[Path], config_path: Path = CONFIG) -> dict[str, Any]:
    config = load_config(config_path)
    environment, observed = parse_results(paths, config)
    sizes = []
    for files in config["sizes"]:
        stages = {stage: summary(observed[(files, stage)])
                  for stage in (*config["stages"]["compiler"],
                                *config["stages"]["context"])}
        first = observed[(files, "complete_compilation")][0]
        static_units = {unit: round(first[unit]) for unit in
                        ("source-bytes", "artifact-bytes", "files")}
        context_bytes = round(observed[(files, "context_materialization")][0][
            "context-bytes"])
        complete = config["complete_path"]
        sizes.append({
            **static_units,
            "context-bytes": context_bytes,
            "artifact_to_source_ratio": round(static_units["artifact-bytes"] /
                                                static_units["source-bytes"], 4),
            "complete_path_median_ns": sum(stages[stage]["median_ns_per_op"]
                                           for stage in complete),
            "complete_path_p95_component_sum_ns": sum(
                stages[stage]["p95_ns_per_op"] for stage in complete),
            "complete_path_median_allocated_bytes": sum(
                stages[stage]["median_allocated_bytes_per_op"] for stage in complete),
            "stages": stages,
        })
    largest = sizes[-1]
    bottleneck = max(config["atomic_stages"],
                     key=lambda stage: largest["stages"][stage]["median_ns_per_op"])
    return {
        "version": "repoctx.m5.profile-report/v1alpha1",
        "profile_contract_sha256": digest(config_path),
        "benchmark_harness_sha256": {
            path.relative_to(PROJECT).as_posix(): digest(path) for path in HARNESSES
        },
        "benchmark_input_sha256": [digest(path) for path in paths],
        "environment": environment,
        "samples_per_stage": config["samples"],
        "benchtime": config["benchtime"],
        "sizes": sizes,
        "largest_atomic_bottleneck": {
            "files": largest["files"], "stage": bottleneck,
            **largest["stages"][bottleneck],
        },
        "target": config["target"],
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["validate", "report"])
    parser.add_argument("--input", type=Path, action="append", default=[])
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    config = load_config()
    if args.action == "validate":
        print(json.dumps({"ok": True, "sizes": config["sizes"],
                          "samples": config["samples"]}, sort_keys=True))
        return 0
    require(args.input, "report requires at least one --input")
    require(args.output is not None, "report requires --output")
    require(not args.output.exists(), "output already exists")
    try:
        args.output.resolve().relative_to(PROJECT)
    except ValueError:
        pass
    else:
        raise ValueError("profile reports must be written outside the repository")
    report = build_report(args.input)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n",
                           encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
