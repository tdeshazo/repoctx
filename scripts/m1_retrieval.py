#!/usr/bin/env python3
"""Compare exact source-span retrieval on frozen M0 tasks, without model calls."""

import argparse
import hashlib
import json
import re
from pathlib import Path
import subprocess
import tempfile

import check_fixtures
from score_paired_trial import coverage


def execute(command):
    result = subprocess.run(command, capture_output=True, timeout=60)
    return {"command": command, "exit_code": result.returncode,
            "stdout": result.stdout.decode(), "stderr": result.stderr.decode()}


def markdown_bundle(text, sources):
    opening = re.search(r"^(`{3,})json\n", text, re.MULTILINE)
    if opening is None:
        raise ValueError("missing fenced manifest")
    bundle, _ = json.JSONDecoder().raw_decode(text[opening.end():])
    for evidence in bundle["evidence"]:
        raw = sources[evidence["file"]][evidence["start_byte"]:evidence["end_byte"]].decode()
        marker = "\n## Evidence " + evidence["id"] + "\n\n"
        start = text.index(marker) + len(marker)
        fence_end = text.index("\n", start)
        fence = text[start:fence_end]
        if not re.fullmatch(r"`{3,}", fence):
            raise ValueError("invalid evidence fence")
        end = text.index("\n" + fence + "\n", fence_end)
        body = text[fence_end + 1:end + 1]
        if body != raw + ("" if raw.endswith("\n") else "\n"):
            raise ValueError("Markdown evidence differs from source bytes")
        evidence["text"] = raw
    return bundle


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", required=True, type=Path)
    parser.add_argument("--after", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    validation, records = check_fixtures.validate(check_fixtures.ROOT)
    if not validation["ok"]:
        raise ValueError(validation)
    rows = []
    for label, binary in (("before", args.before.resolve()), ("after", args.after.resolve())):
        for key, (task, expected, _) in sorted(records.items()):
            with tempfile.TemporaryDirectory(prefix="repoctx-m1-") as temporary:
                scratch = Path(temporary)
                check_fixtures.export(check_fixtures.ROOT, records, key, scratch / "export")
                repository = scratch / "export/repository"
                index = scratch / "index.json"
                compile_result = execute([str(binary), "compile", "-root", str(repository), "-o", str(index)])
                if compile_result["exit_code"]:
                    raise RuntimeError(compile_result)
                sources = {path: (repository / path).read_bytes() for path in task["source_map"].values()
                           if (repository / path).is_file()}
                for format in ("json", "markdown"):
                    command = [str(binary), "context", "-root", str(repository), "-query", task["question"],
                               "-max-bytes", str(task["budgets"]["max_bytes"]),
                               "-max-symbols", str(task["budgets"]["max_symbols"]), "-format", format, str(index)]
                    result = execute(command)
                    bundle = None
                    if result["exit_code"] == 0:
                        if format == "json":
                            bundle = json.loads(result["stdout"])
                        else:
                            bundle = markdown_bundle(result["stdout"], sources)
                    recalled = coverage(bundle, task, sources) if bundle is not None else 0
                    rows.append({"condition": label, "fixture": key, "format": format,
                                 "exit_code": result["exit_code"], "stderr": result["stderr"],
                                 "payload": result["stdout"], "bytes": len(result["stdout"].encode()),
                                 "budget": task["budgets"]["max_bytes"],
                                 "answer_spans": len(task["answer_spans"]), "recalled_spans": recalled,
                                 "source_sha256": {p: hashlib.sha256(b).hexdigest() for p, b in sources.items()}})
    report = {"version": "repoctx.m1.retrieval/v1", "distinct_tasks": len(records),
              "source_revision": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
              "working_tree_changes": bool(subprocess.check_output(["git", "status", "--porcelain"])),
              "binaries": {label: {"path": str(binary), "sha256": hashlib.sha256(binary.read_bytes()).hexdigest()}
                           for label, binary in (("before", args.before), ("after", args.after))},
              "summary": {}, "trials": rows,
              "limitations": ["No model answers, latency, or relationship-recall claims.",
                              "Twelve small frozen synthetic tasks; query/schema changes are part of treatment.",
                              "No-match and restricted fixtures measure scope, not an access-denied classifier."]}
    for label in ("before", "after"):
        for format in ("json", "markdown"):
            selected = [row for row in rows if row["condition"] == label and row["format"] == format]
            answerable = [row for row in selected if row["answer_spans"]]
            report["summary"][label + "_" + format] = {
                "answerable_tasks": len(answerable),
                "fully_covered": sum(row["recalled_spans"] == row["answer_spans"] for row in answerable),
                "recalled_spans": sum(row["recalled_spans"] for row in answerable),
                "answer_spans": sum(row["answer_spans"] for row in answerable),
                "within_budget": all(row["bytes"] <= row["budget"] for row in selected)}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report["summary"], indent=2))


if __name__ == "__main__":
    main()
