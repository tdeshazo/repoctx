#!/usr/bin/python3
"""Agent-facing helper: compile if needed, then query repoctx with the task text."""

import json
from pathlib import Path
import subprocess
import sys
import time


def main():
    task = json.loads(Path("/control/task.json").read_text())
    index = Path("/scratch/index.json.gz")
    metrics = {"compile_seconds": 0.0, "compiled": False}
    if not index.exists():
        start = time.monotonic()
        compiled = subprocess.run(["/tools/repoctx", "compile", "-root", "/workspace", "-o", str(index)],
                                  capture_output=True, text=True)
        metrics.update(compile_seconds=time.monotonic() - start, compiled=True,
                       compile_exit=compiled.returncode)
        if compiled.returncode:
            print(compiled.stderr, file=sys.stderr)
            Path("/scratch/retrieval.json").write_text(json.dumps(metrics))
            return compiled.returncode
    query = " ".join(sys.argv[1:]) if len(sys.argv) > 1 else task["question"]
    command = ["/tools/repoctx", "context", "-root", "/workspace", "-query", query,
               "-max-bytes", str(task["max_bytes"]), "-max-symbols", str(task["max_symbols"]), str(index)]
    start = time.monotonic()
    result = subprocess.run(command, capture_output=True, text=True)
    metrics.update(query=query, context_seconds=time.monotonic() - start,
                   context_exit=result.returncode, payload=result.stdout, stderr=result.stderr)
    with Path("/scratch/retrieval.jsonl").open("a") as stream:
        stream.write(json.dumps(metrics) + "\n")
    sys.stdout.write(result.stdout)
    sys.stderr.write(result.stderr)
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
