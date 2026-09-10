#!/usr/bin/env python3
"""Compile retained replay outputs and manual judgments; does not call a model."""

import hashlib
import json
from pathlib import Path
import statistics

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
CORPUS = ROOT / "evals/m0"


def read(path):
    return json.loads(path.read_text(encoding="utf-8"))


def permitted(task, path):
    def within(prefixes):
        return any(path == p or path.startswith(p + "/") for p in prefixes)
    return within(task["allowed_scope"]) and not within(task["denied_scope"])


def main():
    manifest = read(CORPUS / "manifest.json")
    judgments = read(HERE / "judgments.json")
    runs = [read(HERE / directory / "run.json") for directory in ("pilot", "batch")]
    assert runs[0]["corpus_sha256"] == runs[1]["corpus_sha256"]
    for relative, digest in runs[0]["file_sha256"].items():
        assert hashlib.sha256((ROOT / relative).read_bytes()).hexdigest() == digest, relative
    selected = [key for run in runs for key in run["selected"]]
    assert len(selected) == len(set(selected)) == len(manifest["fixtures"])
    rows = []
    for item in manifest["fixtures"]:
        key = item["id"]
        group = "pilot" if key in runs[0]["selected"] else "batch"
        folder = HERE / group / key
        execution = read(folder / "execution.json")
        delivered = read(folder / "delivered.json")
        answer = read(folder / "answer.txt")
        task, expected = read(CORPUS / item["task"]), read(CORPUS / item["expected"])
        source = {entry["file"]: entry for entry in delivered["files"]}
        scope_ok = all(permitted(task, path) for path in source)
        exact = all(entry["start_byte"] == 0 and entry["end_byte"] == len(entry["text"].encode("utf-8"))
                    for entry in source.values())
        for stored, path in task["source_map"].items():
            if permitted(task, path):
                exact = exact and source[path]["text"].encode("utf-8") == (CORPUS / stored).read_bytes()
            else:
                assert path not in source
                assert (CORPUS / stored).read_text().strip() not in (folder / "answer.txt").read_text()
        citations = answer.get("citations", [])
        citation_ok = bool(citations) and all(isinstance(cite.get("quote"), str) and cite["quote"] and
                      cite.get("file") in source and cite["quote"] in source[cite["file"]]["text"]
                      for cite in citations)
        covered = 0
        for span in task["answer_spans"]:
            path = task["source_map"][span["file"]]
            entry = source.get(path)
            if entry and entry["start_byte"] <= span["start_byte"] and entry["end_byte"] >= span["end_byte"]:
                data = entry["text"].encode("utf-8")
                covered += data[span["start_byte"]:span["end_byte"]].decode("utf-8") == span["text"]
        payload = json.dumps(delivered, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        assert len(payload) == execution["payload_bytes"]
        assert execution["requested_model"] == "gpt-5.6-terra"
        assert execution["exit_code"] == 0 and not execution["tool_calls_observed"]
        judgment = judgments["judgments"][key]
        row = {"fixture": key, "category": item["category"], "execution_pass": True,
               "outcome_expected": expected["outcome"], "outcome_observed": answer["outcome"],
               "outcome_match": answer["outcome"] == expected["outcome"],
               "agent_outcome_pass": judgment["pass"] and citation_ok and answer["outcome"] == expected["outcome"],
               "manual_reason": judgment["reason"], "answer": answer["answer"],
               "citations_exact": citation_ok, "scope_pass": scope_ok, "source_bytes_exact": exact,
               "required_spans": len(task["answer_spans"]), "delivered_spans": covered,
               "delivered_span_recall": covered / len(task["answer_spans"]) if task["answer_spans"] else None,
               "relationship_recall": None,
               "relationship_status": "unavailable_surface" if task["expected_relationships"] else "not_applicable",
               "required_relationships": len(task["expected_relationships"]),
               "payload_bytes": len(payload), "byte_budget": task["budgets"]["max_bytes"],
               "byte_budget_pass": len(payload) <= task["budgets"]["max_bytes"],
               "symbol_budget_status": "not_applicable_no_symbol_selection",
               "exact_token_budget_status": "not_claimed",
               "duration_seconds": execution["duration_seconds"], "usage": execution["usage"],
               "artifact_directory": folder.relative_to(HERE).as_posix()}
        assert scope_ok and exact and citation_ok and row["byte_budget_pass"]
        rows.append(row)
    category_results = {}
    for row in rows:
        group = category_results.setdefault(row["category"], {"passed": 0, "total": 0})
        group["passed"] += int(row["agent_outcome_pass"])
        group["total"] += 1
    usage = {key: sum((row["usage"] or {}).get(key, 0) for row in rows)
             for key in ("input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens")}
    result = {"condition": runs[0]["condition"], "requested_model": "gpt-5.6-terra",
              "served_model_snapshot": None, "source_revision": runs[0]["source_revision"],
              "worktree_dirty": runs[0]["worktree_dirty"], "corpus_sha256": runs[0]["corpus_sha256"],
              "cli_version": runs[0]["cli_version"], "reasoning_effort": "medium",
              "trial_count": len(rows), "distinct_task_count": len(rows),
              "agent_outcomes": {"passed": sum(row["agent_outcome_pass"] for row in rows), "total": len(rows)},
              "categories": category_results, "usage": usage,
              "duration_seconds": {"sum": round(sum(row["duration_seconds"] for row in rows), 3),
                                   "median": statistics.median(row["duration_seconds"] for row in rows)},
              "required_spans": sum(row["required_spans"] for row in rows),
              "delivered_spans": sum(row["delivered_spans"] for row in rows),
              "manual_grading_policy": judgments["policy"], "results": rows}
    (HERE / "results.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps({key: result[key] for key in ("agent_outcomes", "categories", "usage", "duration_seconds",
                                                  "required_spans", "delivered_spans", "corpus_sha256")}))


if __name__ == "__main__":
    main()
