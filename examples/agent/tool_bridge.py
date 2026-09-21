"""Model-neutral agent tool adapter. It does not call an LLM service.

The application, not the model, fixes the binary, root, index, exclusions, and
maximum output. Return the resulting string as tool-result content. Never place
repository-derived text in a system/developer instruction message.
"""
from __future__ import annotations

import base64
import hashlib
import json
import os
import re
import stat
import subprocess
from pathlib import Path
from typing import Any, Sequence


_HANDLE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}\Z")
_CONTEXT_VERSIONS = {
    "repoctx.context/v1alpha1",
    "repoctx.context/v1alpha2",
    "repoctx.context/v1alpha3",
}


def _compact(value: object) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def _inside(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
        return True
    except ValueError:
        return False


def _has_omissions(value: object) -> bool:
    if isinstance(value, dict):
        return any(_has_omissions(item) for item in value.values())
    if isinstance(value, list):
        return any(_has_omissions(item) for item in value)
    return bool(value)


class RepositoryContextTool:
    def __init__(
        self,
        binary: str,
        root: str,
        index: str,
        *,
        max_bytes: int = 32_768,
        denied_prefixes: Sequence[str] = (),
        bundle_directory: str | None = None,
        max_reference_bytes: int = 8_192,
        max_read_bytes: int | None = None,
        max_read_response_bytes: int = 32_768,
    ) -> None:
        self.binary = str(Path(binary).resolve(strict=True))
        root_path = Path(root).resolve(strict=True)
        self.root = str(root_path)
        self.index = str(Path(index).resolve(strict=True))
        if not 1 <= max_bytes <= 8 * 1024 * 1024:
            raise ValueError("Invalid payload limit")
        if not 1_024 <= max_reference_bytes <= 1024 * 1024:
            raise ValueError("Invalid reference response limit")
        read_limit = min(16_384, max_bytes) if max_read_bytes is None else max_read_bytes
        if not 1 <= read_limit <= max_bytes:
            raise ValueError("Invalid bundle read limit")
        if not 1_024 <= max_read_response_bytes <= 8 * 1024 * 1024:
            raise ValueError("Invalid bundle read response limit")
        self.max_bytes = max_bytes
        self.max_reference_bytes = max_reference_bytes
        self.max_read_bytes = read_limit
        self.max_read_response_bytes = max_read_response_bytes
        self.denied_prefixes = tuple(denied_prefixes)
        self.bundle_directory: Path | None = None
        if bundle_directory is not None:
            store = Path(bundle_directory).resolve(strict=True)
            if not store.is_dir():
                raise ValueError("Bundle store must be a directory")
            if _inside(store, root_path):
                raise ValueError("Bundle store must be outside the indexed repository")
            self.bundle_directory = store
        self._persisted: dict[str, dict[str, Any]] = {}

    def get_context(
        self,
        query: str = "",
        symbol_ids: Sequence[str] = (),
        *,
        snapshot: str | None = None,
        depth: int = 1,
        unit_ids: Sequence[str] = (),
        delivery: str = "inline",
        handle: str | None = None,
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
        if isinstance(unit_ids, (str, bytes)) or len(unit_ids) > 8:
            raise ValueError("Expected at most 8 retrieval unit IDs")
        if any(not isinstance(s, str) or len(s.encode("utf-8")) > 8192 for s in unit_ids):
            raise ValueError("Invalid unit IDs")
        if depth not in range(0, 5):
            raise ValueError("Depth must be 0..4")
        if delivery not in {"inline", "file_reference"}:
            raise ValueError("Delivery must be inline or file_reference")
        if delivery == "inline" and handle is not None:
            raise ValueError("Inline delivery does not accept a handle")
        if delivery == "file_reference":
            self._validate_handle(handle)
            if self.bundle_directory is None:
                raise ValueError("File-reference delivery requires a bundle directory")
        args = [
            self.binary, "context", "-root", self.root,
            "-max-bytes", str(self.max_bytes), "-depth", str(depth),
            "-query", query, "-format", "json",
        ]
        for symbol in symbol_ids:
            args.extend(["-symbol", symbol])
        for unit in unit_ids:
            args.extend(["-unit", unit])
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
        if bundle.get("version") not in _CONTEXT_VERSIONS:
            raise RuntimeError("Unexpected context protocol")
        if bundle.get("trust", {}).get("role") != "untrusted_repository_data":
            raise RuntimeError("Missing evidence trust classification")
        if delivery == "inline":
            return payload  # Preserve the counted bytes; do not pretty-print/re-encode.
        return self._persist_reference(handle, completed.stdout, bundle)

    def read_context(self, handle: str, *, offset: int = 0, max_bytes: int | None = None) -> str:
        """Read an exact bounded byte range from a bundle created by this adapter.

        Data is base64 so byte ranges remain exact even when they split UTF-8.
        The returned handle grants access only to a bundle registered by this
        adapter instance; it is not interpreted as a filesystem path.
        """
        self._validate_handle(handle)
        if handle not in self._persisted:
            raise KeyError("Unknown bundle handle")
        if type(offset) is not int or offset < 0:
            raise ValueError("Invalid bundle offset")
        limit = self.max_read_bytes if max_bytes is None else max_bytes
        if type(limit) is not int or not 1 <= limit <= self.max_read_bytes:
            raise ValueError("Invalid bundle read size")
        metadata = self._persisted[handle]
        data = self._verified_bundle_bytes(metadata)
        if offset > len(data):
            raise ValueError("Bundle offset exceeds its byte size")
        available = data[offset:min(len(data), offset + limit)]

        def response(size: int) -> dict[str, Any]:
            end = offset + size
            chunk = available[:size]
            return {
                "version": "repoctx.adapter.bundle-read/v1alpha1",
                "handle": handle,
                "snapshot_id": metadata["snapshot_id"],
                "bundle_sha256": metadata["sha256"],
                "bundle_bytes": len(data),
                "start_byte": offset,
                "end_byte": end,
                "next_offset": None if end == len(data) else end,
                "returned_bytes": size,
                "requested_bytes": limit,
                "encoding": "base64",
                "data": base64.b64encode(chunk).decode("ascii"),
                "chunk_sha256": hashlib.sha256(chunk).hexdigest(),
                "complete": end == len(data),
                "incomplete": metadata["bundle_incomplete"] or end != len(data),
                "trust": metadata["trust"],
            }

        low, high, selected = 0, len(available), None
        while low <= high:
            middle = (low + high) // 2
            candidate = response(middle)
            if len(_compact(candidate)) <= self.max_read_response_bytes:
                selected = candidate
                low = middle + 1
            else:
                high = middle - 1
        if selected is None:
            raise RuntimeError("Bundle read metadata exceeds its response budget")
        if available and selected["returned_bytes"] == 0:
            raise RuntimeError("Bundle read response budget cannot return evidence bytes")
        return _compact(selected).decode("utf-8")

    @staticmethod
    def _validate_handle(handle: object) -> None:
        if not isinstance(handle, str) or _HANDLE.fullmatch(handle) is None:
            raise ValueError("Invalid caller-controlled bundle handle")

    def _persist_reference(self, handle: str, payload: bytes, bundle: dict[str, Any]) -> str:
        if handle in self._persisted:
            raise ValueError("Bundle handle already exists")
        snapshot = bundle.get("snapshot")
        if not isinstance(snapshot, dict) or not isinstance(snapshot.get("id"), str):
            raise RuntimeError("File-reference delivery requires snapshot identity")
        symbol_ids = [item["id"] for item in bundle.get("symbols", [])
                      if isinstance(item, dict) and isinstance(item.get("id"), str)]
        unit_ids = [item["id"] for item in bundle.get("units", [])
                    if isinstance(item, dict) and isinstance(item.get("id"), str)]
        omissions = bundle.get("omissions", {})
        metadata = {
            "path": self.bundle_directory / f"{handle}.context.json",
            "sha256": hashlib.sha256(payload).hexdigest(),
            "bytes": len(payload),
            "snapshot_id": snapshot["id"],
            "trust": bundle["trust"],
            "bundle_incomplete": _has_omissions(omissions),
        }
        reference = {
            "version": "repoctx.adapter.file-reference/v1alpha1",
            "delivery": "file_reference",
            "handle": handle,
            "snapshot": {key: snapshot[key] for key in
                         ("id", "source_id", "profile_id", "verification") if key in snapshot},
            "bundle_bytes": len(payload),
            "bundle_sha256": metadata["sha256"],
            "relevant": {"symbol_ids": symbol_ids, "unit_ids": unit_ids},
            "trust": bundle["trust"],
            "incompleteness": {
                "bundle": metadata["bundle_incomplete"],
                "bundle_omissions": omissions,
                "reference_omissions": {"symbol_ids": 0, "unit_ids": 0},
                "reference": False,
            },
            "warnings": len(bundle.get("warnings", [])),
            "preview": None,
            "read": {"operation": "read_context", "encoding": "base64",
                     "max_source_bytes": self.max_read_bytes,
                     "max_response_bytes": self.max_read_response_bytes},
        }
        while len(_compact(reference)) > self.max_reference_bytes:
            symbols = reference["relevant"]["symbol_ids"]
            units = reference["relevant"]["unit_ids"]
            if not symbols and not units:
                raise RuntimeError("File-reference metadata exceeds its response budget")
            key = "symbol_ids" if len(symbols) >= len(units) and symbols else "unit_ids"
            reference["relevant"][key].pop()
            reference["incompleteness"]["reference_omissions"][key] += 1
            reference["incompleteness"]["reference"] = True
        try:
            self._publish(metadata["path"], payload)
        except FileExistsError:
            raise ValueError("Bundle handle already exists") from None
        self._persisted[handle] = metadata
        return _compact(reference).decode("utf-8")

    @staticmethod
    def _publish(path: Path, payload: bytes) -> None:
        flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_CLOEXEC", 0)
        flags |= getattr(os, "O_NOFOLLOW", 0)
        descriptor = None
        created = False
        try:
            descriptor = os.open(path, flags, 0o600)
            created = True
            view = memoryview(payload)
            while view:
                written = os.write(descriptor, view)
                if written <= 0:
                    raise OSError("short bundle write")
                view = view[written:]
            os.fsync(descriptor)
        except Exception:
            if created:
                try:
                    path.unlink()
                except FileNotFoundError:
                    pass
            raise
        finally:
            if descriptor is not None:
                os.close(descriptor)

    @staticmethod
    def _verified_bundle_bytes(metadata: dict[str, Any]) -> bytes:
        flags = os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) | getattr(os, "O_NOFOLLOW", 0)
        descriptor = os.open(metadata["path"], flags)
        try:
            info = os.fstat(descriptor)
            if not stat.S_ISREG(info.st_mode) or info.st_size != metadata["bytes"]:
                raise RuntimeError("Persisted bundle identity changed")
            chunks = []
            while True:
                chunk = os.read(descriptor, 64 * 1024)
                if not chunk:
                    break
                chunks.append(chunk)
            data = b"".join(chunks)
        finally:
            os.close(descriptor)
        if hashlib.sha256(data).hexdigest() != metadata["sha256"]:
            raise RuntimeError("Persisted bundle identity changed")
        return data
