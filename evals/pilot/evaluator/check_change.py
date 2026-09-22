#!/usr/bin/env python3
"""Evaluator-only runtime check for p-change-batch-cap."""

import argparse
import importlib
import sys
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", required=True, type=Path)
    args = parser.parse_args()
    repository = args.repository.resolve()

    if not (repository / "relay" / "batches.py").is_file():
        parser.error("--repository must contain relay/batches.py")

    sys.path.insert(0, str(repository))
    batches = importlib.import_module("relay.batches")
    cases = ((1, True), (12, True), (13, False), (0, False), (-1, False))
    failures = [
        f"accepts_batch_size({count}) returned {actual!r}, expected {expected!r}"
        for count, expected in cases
        if (actual := batches.accepts_batch_size(count)) != expected
    ]
    if failures:
        print("p-change-batch-cap failed:", *failures, sep="\n  ", file=sys.stderr)
        return 1
    print("p-change-batch-cap passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
