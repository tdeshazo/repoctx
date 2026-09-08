"""Model-neutral agent tool adapter. It does not call an LLM service.

The application, not the model, fixes the binary, root, index, exclusions, and
maximum output. Return the resulting string as tool-result content. Never place
repository-derived text in a system/developer instruction message.
"""
from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Sequence


class RepositoryContextTool:
    def __init__(
        self,
        binary: str,
        root: str,
        index: str,
        *,
        max_bytes: int = 32_768,
        denied_prefixes: Sequence[str] = (),
    ) -> None:
        self.binary = str(Path(binary).resolve(strict=True))
        self.root = str(Path(root).resolve(strict=True))
        self.index = str(Path(index).resolve(strict=True))
        if not 1 <= max_bytes <= 8 * 1024 * 1024:
            raise ValueError("Invalid payload limit")
        self.max_bytes = max_bytes
        self.denied_prefixes = tuple(denied_prefixes)

    def get_context(
        self,
        query: str = "",
        symbol_ids: Sequence[str] = (),
        *,
        snapshot: str | None = None,
        depth: int = 1,
    ) -> str:
        """Retrieve evidence; symbols are semantic IDs, not graph-array indexes.

        To expand a prior bundle, supply its snapshot.id and a symbol ID from
        its symbol or relationship records. Stale/mismatched snapshots fail.
        """
        if not isinstance(query, str) or len(query.encode("utf-8")) > 8192:
            raise ValueError("Invalid query")
        if isinstance(symbol_ids, (str, bytes)) or len(symbol_ids) > 12:
            raise ValueError("Expected at most 12 symbol IDs")
        if any(not isinstance(s, str) or len(s.encode("utf-8")) > 8192 for s in symbol_ids):
            raise ValueError("Invalid symbol IDs")
        if depth not in range(0, 5):
            raise ValueError("Depth must be 0..4")
        args = [
            self.binary, "context", "-root", self.root,
            "-max-bytes", str(self.max_bytes), "-depth", str(depth),
            "-query", query, "-format", "json",
        ]
        for symbol in symbol_ids:
            args.extend(["-symbol", symbol])
        for prefix in self.denied_prefixes:
            args.extend(["-deny", prefix])
        if snapshot is not None:
            args.extend(["-expect-snapshot", snapshot])
        args.append(self.index)
        completed = subprocess.run(args, capture_output=True, timeout=60, check=False)
        if completed.returncode != 0:
            raise RuntimeError(completed.stderr.decode("utf-8", errors="replace"))
        if len(completed.stdout) > self.max_bytes:
            raise RuntimeError("Compiler exceeded the configured payload bound")
        payload = completed.stdout.decode("utf-8")
        bundle = json.loads(payload)
        if bundle.get("version") != "repoctx.context/v1alpha1":
            raise RuntimeError("Unexpected context protocol")
        if bundle.get("trust", {}).get("role") != "untrusted_repository_data":
            raise RuntimeError("Missing evidence trust classification")
        return payload  # Preserve the counted bytes; do not pretty-print/re-encode.
