#!/usr/bin/env python3
"""Verify paired trial artifacts and aggregate outcomes after masked grading."""

import argparse
from collections import defaultdict
import hashlib
import json
from pathlib import Path
import statistics


ROOT = Path(__file__).resolve().parents[1]


def read(path):
    return json.loads(path.read_text(encoding="utf-8"))


def answer_json(text):
    text = text.strip()
    if text.startswith("```json") and text.endswith("```"):
        text = text[7:-3].strip()
    return json.loads(text)


def coverage(bundle, task, sources):
    ranges = defaultdict(list)
    for evidence in bundle.get("evidence", []):
        file = evidence["file"]
        start, end = evidence["start_byte"], evidence["end_byte"]
        assert file in sources and 0 <= start <= end <= len(sources[file])
        assert sources[file][start:end].decode("utf-8") == evidence["text"]
        assert hashlib.sha256(sources[file]).hexdigest() == evidence["sha256"]
        ranges[file].append((start, end))
    matched = 0
    for span in task["answer_spans"]:
        position = span["start_byte"]
        for start, end in sorted(ranges[task["source_map"][span["file"]]]):
            if start > position:
                break
            position = max(position, end)
        matched += position >= span["end_byte"]
    return matched


def aggregate(rows):
    durations = [row["duration_seconds"] for row in rows]
    inputs = [row["usage"]["input_tokens"] for row in rows if row["usage"] is not None]
    return {"trials": len(rows), "distinct_tasks": len({row["fixture"] for row in rows}),
            "passed": sum(row["passed"] for row in rows),
            "exact_citations": sum(row["citations_exact"] for row in rows),
            "condition_adherent": sum(row["condition_adherent"] for row in rows),
            "median_seconds": statistics.median(durations),
            "mean_seconds": statistics.mean(durations),
            "mean_input_tokens": statistics.mean(inputs) if inputs else None,
            "usage_available": len(inputs),
            "sum_usage": {field: sum((row["usage"] or {}).get(field, 0) for row in rows)
                          for field in ("input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens")},
            "mean_shell_calls": statistics.mean(row["shell_calls"] for row in rows),
            "mean_tool_output_bytes": statistics.mean(row["tool_output_bytes"] for row in rows),
            "tool_errors": sum(row["tool_errors"] for row in rows),
            "truncated_tool_outputs": sum(row["truncated_tool_outputs"] for row in rows),
            "source_unchanged": sum(row["source_unchanged"] for row in rows),
            "foreign_tool_trials": sum(row["foreign_tool_count"] > 0 for row in rows)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    args = parser.parse_args()
    directory = args.directory
    protocol = read(directory / "protocol.json")
    for path, expected in protocol["corpus_file_sha256"].items():
        assert hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == expected, path
    trials = [json.loads(line) for line in (directory / "trials.jsonl").read_text().splitlines()]
    assert len(trials) == len({row["id"] for row in trials}) == len(protocol["schedule"])
    key = read(directory / "blind-key.json")
    blind = {value: name for name, value in key.items()}
    judgments = read(directory / "judgments.json")
    assert set(judgments) == set(key)
    assert all(isinstance(value.get("pass"), bool) and isinstance(value.get("reason"), str)
               and value["reason"] for value in judgments.values())
    scheduled = {row["id"]: row for row in protocol["schedule"]}
    for trial in trials:
        assert all(trial[field] == value for field, value in scheduled[trial["id"]].items())
    rows = []
    for trial in sorted(trials, key=lambda row: row["id"]):
        task = read(ROOT / "evals/m0/tasks" / (trial["fixture"] + ".json"))
        expected = read(ROOT / "evals/m0/expected" / (trial["fixture"] + ".json"))
        sources = {path: (ROOT / "evals/m0" / source).read_bytes()
                   for source, path in task["source_map"].items() if path in protocol["permitted_source_sha256"][trial["fixture"]]}
        try:
            answer = answer_json(trial["answer"])
            citations = answer["citations"]
            exact = bool(citations) and all(cite["file"] in sources and isinstance(cite["quote"], str)
                    and bool(cite["quote"]) and cite["quote"].encode("utf-8") in sources[cite["file"]]
                    for cite in citations)
            outcome_match = answer["outcome"] == expected["outcome"]
        except (ValueError, TypeError, KeyError):
            exact, outcome_match = False, False
        commands = trial["shell"]
        for command in commands:
            body = {key: command[key] for key in ("exit_code", "stdout", "stderr", "truncated")}
            size = len(json.dumps(body, ensure_ascii=False, separators=(",", ":")).encode("utf-8"))
            assert size == command["delivered_bytes"] <= task["budgets"]["max_bytes"]
        assisted = trial["condition"] == "assisted"
        adhered = bool(trial["retrieval"]) and bool(commands) and commands[0]["command"].strip() == "repoctx-evidence" if assisted else not trial["retrieval"]
        initial = trial["retrieval"][0] if trial["retrieval"] else None
        initial_metrics = None
        if assisted:
            initial_metrics = {"available": False, "context_exit": initial["context_exit"] if initial else None,
                               "required_spans": len(task["answer_spans"]), "covered_spans": 0,
                               "full_evidence": False if task["answer_spans"] else None,
                               "compile_seconds": initial["compile_seconds"] if initial else None,
                               "compiled": initial["compiled"] if initial else None,
                               "relationship_recall": None}
            if initial and initial["context_exit"] == 0:
                bundle = json.loads(initial["payload"])
                assert len(initial["payload"].encode()) <= task["budgets"]["max_bytes"]
                assert len(bundle["symbols"]) <= task["budgets"]["max_symbols"]
                delivered = any(not command["truncated"] and command["stdout"] == initial["payload"] for command in commands)
                matched = coverage(bundle, task, sources)
                initial_metrics.update(available=delivered, covered_spans=matched if delivered else 0,
                                       full_evidence=matched == len(task["answer_spans"]) and delivered if task["answer_spans"] else None,
                                       verified_files=bundle["snapshot"]["verified_files"])
                # Only the context obligation is observable here. Unit-owned
                # imports require an index graph evaluator and stay unavailable.
                obligations = task["expected_relationships"]
                if obligations and all(rel["surface"] == "context" for rel in obligations) and delivered:
                    names = {symbol["id"]: symbol["name"] for symbol in bundle["symbols"]}
                    matched_rels = 0
                    for rel in obligations:
                        for observed in bundle.get("relationships", []):
                            from_name = names.get(observed["from"], observed["from"].split("#")[-1])
                            to_name = names.get(observed["to"], observed["to"].split("#")[-1])
                            if (from_name, to_name, observed["kind"], observed["resolution"]) == (rel["from"], rel["to"], rel["kind"], rel["resolution"]):
                                matched_rels += bool(observed.get("sites"))
                                break
                    initial_metrics["relationship_recall"] = matched_rels / len(obligations)
        judgment = judgments[blind[trial["id"]]]
        rows.append({key: trial[key] for key in ("id", "fixture", "cache", "repeat", "condition", "duration_seconds", "usage", "source_unchanged")} |
                    {"blind_id": blind[trial["id"]], "passed": bool(judgment["pass"] and exact and outcome_match and trial["exit_code"] == 0),
                     "judgment": judgment, "citations_exact": exact, "outcome_match": outcome_match,
                     "condition_adherent": adhered, "shell_calls": len(commands),
                     "tool_errors": sum(command["exit_code"] != 0 for command in commands),
                     "tool_output_bytes": sum(command["delivered_bytes"] for command in commands),
                     "truncated_tool_outputs": sum(command["truncated"] for command in commands),
                     "foreign_tool_count": len(trial["foreign_tools"]), "source_read_syscalls": None,
                     "source_paths_mentioned_in_commands": sorted(path for path in sources if any(path in call["command"] for call in commands)),
                     "post_initial_shell_calls": max(0, len(commands) - 1) if assisted else None,
                     "initial_retrieval": initial_metrics})
    groups = {cache + "/" + condition: aggregate([row for row in rows if row["cache"] == cache and row["condition"] == condition])
              for cache in ("cold", "warm") for condition in ("baseline", "assisted")}
    by_task = {task: {condition: aggregate([row for row in rows if row["fixture"] == task and row["condition"] == condition])
                     for condition in ("baseline", "assisted")} for task in sorted({row["fixture"] for row in rows})}
    pairs = defaultdict(dict)
    for row in rows:
        pairs[row["fixture"], row["cache"], row["repeat"]][row["condition"]] = row
    paired = {}
    for cache in ("cold", "warm"):
        selected = [pair for (_, stratum, _), pair in pairs.items() if stratum == cache]
        assert all(set(pair) == {"baseline", "assisted"} for pair in selected)
        paired[cache] = {
            "pairs": len(selected),
            "baseline_only_pass": sum(pair["baseline"]["passed"] and not pair["assisted"]["passed"] for pair in selected),
            "assisted_only_pass": sum(pair["assisted"]["passed"] and not pair["baseline"]["passed"] for pair in selected),
            "mean_assisted_minus_baseline_seconds": statistics.mean(pair["assisted"]["duration_seconds"] - pair["baseline"]["duration_seconds"] for pair in selected),
            "mean_assisted_minus_baseline_input_tokens": statistics.mean(pair["assisted"]["usage"]["input_tokens"] - pair["baseline"]["usage"]["input_tokens"] for pair in selected if pair["assisted"]["usage"] and pair["baseline"]["usage"]),
        }
    result = {"groups": groups, "by_task": by_task, "rows": rows, "paired": paired,
              "overall": {condition: aggregate([row for row in rows if row["condition"] == condition])
                          for condition in ("baseline", "assisted")}}
    (directory / "results.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps({"groups": groups, "overall": result["overall"]}, indent=2))


if __name__ == "__main__":
    main()
