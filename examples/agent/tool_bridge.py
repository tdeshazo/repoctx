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
import secrets
import stat
import subprocess
import time
from pathlib import Path
from typing import Any, Callable, Sequence


_HANDLE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}\Z")
_CONTEXT_VERSION = "repoctx.context/v1alpha3"
_CHECKPOINT_VERSION = "repoctx.adapter.checkpoint/v1alpha1"


class EvidenceLifecycleError(RuntimeError):
    """Base error for persisted evidence lifecycle failures."""


class MissingReferenceError(EvidenceLifecycleError):
    """The requested handle is not registered or its bundle is missing."""


class ExpiredReferenceError(EvidenceLifecycleError):
    """The requested bundle or checkpoint exceeded its retention period."""


class StaleEvidenceError(EvidenceLifecycleError):
    """The checkout no longer matches the checkpoint snapshot."""


class StorageLimitError(EvidenceLifecycleError):
    """The caller-owned run would exceed its configured storage bound."""


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
        run_id: str | None = None,
        resume_run: bool = False,
        max_reference_bytes: int = 8_192,
        max_read_bytes: int | None = None,
        max_read_response_bytes: int = 32_768,
        max_checkpoint_bytes: int = 16_384,
        max_store_bytes: int = 64 * 1024 * 1024,
        max_bundles: int = 64,
        retention_seconds: int = 24 * 60 * 60,
        clock: Callable[[], float] = time.time,
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
        if not 1_024 <= max_checkpoint_bytes <= 1024 * 1024:
            raise ValueError("Invalid checkpoint limit")
        if not 1 <= max_store_bytes <= 1024 * 1024 * 1024:
            raise ValueError("Invalid bundle store limit")
        if not 1 <= max_bundles <= 1024:
            raise ValueError("Invalid bundle count limit")
        if not 1 <= retention_seconds <= 7 * 24 * 60 * 60:
            raise ValueError("Invalid bundle retention")
        if not callable(clock):
            raise ValueError("Invalid lifecycle clock")
        self.max_bytes = max_bytes
        self.max_reference_bytes = max_reference_bytes
        self.max_read_bytes = read_limit
        self.max_read_response_bytes = max_read_response_bytes
        self.max_checkpoint_bytes = max_checkpoint_bytes
        self.max_store_bytes = max_store_bytes
        self.max_bundles = max_bundles
        self.retention_seconds = retention_seconds
        self._clock = clock
        self.denied_prefixes = tuple(denied_prefixes)
        self.run_id: str | None = None
        self.bundle_directory: Path | None = None
        if bundle_directory is not None:
            self._validate_handle(run_id, label="run ID")
            store = Path(bundle_directory).resolve(strict=True)
            if not store.is_dir():
                raise ValueError("Bundle store must be a directory")
            if _inside(store, root_path):
                raise ValueError("Bundle store must be outside the indexed repository")
            run_directory = store / run_id
            if resume_run:
                try:
                    info = run_directory.lstat()
                except FileNotFoundError:
                    raise MissingReferenceError("Persisted run is missing") from None
                if stat.S_ISLNK(info.st_mode) or not stat.S_ISDIR(info.st_mode):
                    raise EvidenceLifecycleError("Persisted run is not an isolated directory")
                if info.st_mode & 0o077:
                    raise EvidenceLifecycleError("Persisted run permissions are not owner-only")
                if hasattr(os, "geteuid") and info.st_uid != os.geteuid():
                    raise EvidenceLifecycleError("Persisted run has another owner")
                resolved_run = run_directory.resolve(strict=True)
                if resolved_run.parent != store:
                    raise EvidenceLifecycleError("Persisted run escaped its caller scope")
            else:
                try:
                    run_directory.mkdir(mode=0o700)
                except FileExistsError:
                    raise EvidenceLifecycleError("Bundle run already exists") from None
                resolved_run = run_directory.resolve(strict=True)
            self.run_id = run_id
            self.bundle_directory = resolved_run
        elif run_id is not None or resume_run:
            raise ValueError("Run lifecycle requires a bundle directory")
        self._persisted: dict[str, dict[str, Any]] = {}
        self._expired: set[str] = set()
        self._snapshot_id: str | None = None

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
        if bundle.get("version") != _CONTEXT_VERSION:
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
        if handle in self._expired:
            raise ExpiredReferenceError("Bundle handle expired")
        if handle not in self._persisted:
            raise MissingReferenceError("Bundle handle is not registered for this run")
        if type(offset) is not int or offset < 0:
            raise ValueError("Invalid bundle offset")
        limit = self.max_read_bytes if max_bytes is None else max_bytes
        if type(limit) is not int or not 1 <= limit <= self.max_read_bytes:
            raise ValueError("Invalid bundle read size")
        metadata = self._persisted[handle]
        if self._now() >= metadata["expires_unix"]:
            self._expired.add(handle)
            raise ExpiredReferenceError("Bundle handle expired")
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

    def create_checkpoint(
        self,
        task: str,
        *,
        unresolved_questions: Sequence[str],
        next_retrieval: dict[str, Any],
    ) -> str:
        """Return bounded harness-owned state; it is not repository evidence."""
        if not isinstance(task, str) or not task or len(task.encode("utf-8")) > 8192:
            raise ValueError("Invalid checkpoint task")
        if isinstance(unresolved_questions, (str, bytes)) or len(unresolved_questions) > 16:
            raise ValueError("Invalid unresolved questions")
        if any(not isinstance(item, str) or len(item.encode("utf-8")) > 2048
               for item in unresolved_questions):
            raise ValueError("Invalid unresolved questions")
        retrieval = self._normalize_next_retrieval(next_retrieval)
        if not self._persisted or self._snapshot_id is None:
            raise EvidenceLifecycleError("A checkpoint requires persisted evidence")
        now = self._now()
        records = []
        for handle in sorted(self._persisted):
            metadata = self._persisted[handle]
            if now >= metadata["expires_unix"]:
                self._expired.add(handle)
                raise ExpiredReferenceError("Cannot checkpoint expired evidence")
            self._verified_bundle_bytes(metadata)
            records.append({
                "handle": handle,
                "bundle_bytes": metadata["bytes"],
                "bundle_sha256": metadata["sha256"],
                "created_unix": metadata["created_unix"],
                "expires_unix": metadata["expires_unix"],
                "relevant": metadata["relevant"],
                "incomplete": metadata["bundle_incomplete"],
            })
        first = self._persisted[records[0]["handle"]]
        checkpoint = {
            "version": _CHECKPOINT_VERSION,
            "run_id": self.run_id,
            "task": task,
            "snapshot": first["snapshot"],
            "bundles": records,
            "unresolved_questions": list(unresolved_questions),
            "next_retrieval": retrieval,
            "created_unix": now,
            "expires_unix": min(item["expires_unix"] for item in records),
            "trust": first["trust"],
        }
        encoded = _compact(checkpoint)
        if len(encoded) > self.max_checkpoint_bytes:
            raise EvidenceLifecycleError("Checkpoint exceeds its response budget")
        return encoded.decode("utf-8")

    def resume_checkpoint(self, encoded: str) -> dict[str, Any]:
        """Verify stored bytes and current source identity before enabling reads."""
        if self.bundle_directory is None or self.run_id is None:
            raise EvidenceLifecycleError("Checkpoint resume requires a persisted run")
        if self._persisted:
            raise EvidenceLifecycleError("Checkpoint resume requires an empty adapter")
        if not isinstance(encoded, str) or len(encoded.encode("utf-8")) > self.max_checkpoint_bytes:
            raise ValueError("Invalid checkpoint encoding")
        try:
            checkpoint = json.loads(encoded)
        except json.JSONDecodeError:
            raise ValueError("Invalid checkpoint encoding") from None
        if not isinstance(checkpoint, dict) or checkpoint.get("version") != _CHECKPOINT_VERSION:
            raise ValueError("Unexpected checkpoint protocol")
        if checkpoint.get("run_id") != self.run_id:
            raise EvidenceLifecycleError("Checkpoint belongs to another run")
        task = checkpoint.get("task")
        questions = checkpoint.get("unresolved_questions")
        self._validate_checkpoint_text(task, questions)
        self._normalize_next_retrieval(checkpoint.get("next_retrieval"))
        snapshot = checkpoint.get("snapshot")
        if not isinstance(snapshot, dict) or not isinstance(snapshot.get("id"), str):
            raise ValueError("Checkpoint lacks snapshot identity")
        trust = checkpoint.get("trust")
        if not isinstance(trust, dict) or trust.get("role") != "untrusted_repository_data":
            raise ValueError("Checkpoint lacks evidence trust classification")
        records = checkpoint.get("bundles")
        if not isinstance(records, list) or not records or len(records) > self.max_bundles:
            raise ValueError("Invalid checkpoint bundle inventory")

        now = self._now()
        pending: dict[str, dict[str, Any]] = {}
        total_bytes = 0
        for record in records:
            metadata = self._checkpoint_metadata(record, snapshot, trust)
            handle = record["handle"]
            if handle in pending:
                raise ValueError("Duplicate checkpoint bundle handle")
            if now >= metadata["expires_unix"]:
                self._expired.add(handle)
                raise ExpiredReferenceError("Checkpoint contains expired evidence")
            total_bytes += metadata["bytes"]
            if total_bytes > self.max_store_bytes:
                raise StorageLimitError("Checkpoint exceeds the run storage limit")
            self._verified_bundle_bytes(metadata)
            pending[handle] = metadata
        expected_names = {item["path"].name for item in pending.values()}
        actual_names = {item.name for item in self.bundle_directory.iterdir()}
        if actual_names != expected_names:
            raise EvidenceLifecycleError("Persisted run does not match checkpoint inventory")

        try:
            validation = json.loads(self.get_context(snapshot=snapshot["id"], depth=0))
        except (RuntimeError, json.JSONDecodeError) as error:
            raise StaleEvidenceError("Checkpoint snapshot revalidation failed") from error
        if validation.get("snapshot", {}).get("id") != snapshot["id"]:
            raise StaleEvidenceError("Checkpoint snapshot identity changed")

        self._persisted = pending
        self._snapshot_id = snapshot["id"]
        return checkpoint

    def cleanup_expired(self) -> list[str]:
        """Remove only expired bundles owned by this run; active files remain."""
        now = self._now()
        removed = []
        for handle, metadata in list(self._persisted.items()):
            if now < metadata["expires_unix"]:
                continue
            try:
                self._verified_bundle_bytes(metadata)
            except MissingReferenceError:
                pass
            else:
                metadata["path"].unlink()
            del self._persisted[handle]
            self._expired.add(handle)
            removed.append(handle)
        return removed

    def _checkpoint_metadata(
        self,
        record: object,
        snapshot: dict[str, Any],
        trust: dict[str, Any],
    ) -> dict[str, Any]:
        if not isinstance(record, dict):
            raise ValueError("Invalid checkpoint bundle record")
        handle = record.get("handle")
        self._validate_handle(handle)
        size = record.get("bundle_bytes")
        digest = record.get("bundle_sha256")
        created = record.get("created_unix")
        expires = record.get("expires_unix")
        if type(size) is not int or not 0 <= size <= self.max_bytes:
            raise ValueError("Invalid checkpoint bundle size")
        if not isinstance(digest, str) or re.fullmatch(r"[0-9a-f]{64}", digest) is None:
            raise ValueError("Invalid checkpoint bundle digest")
        if type(created) is not int or type(expires) is not int or expires <= created:
            raise ValueError("Invalid checkpoint retention")
        relevant = record.get("relevant")
        if not isinstance(relevant, dict):
            raise ValueError("Invalid checkpoint relevant IDs")
        symbols = self._validate_checkpoint_ids(relevant.get("symbol_ids"), 12)
        units = self._validate_checkpoint_ids(relevant.get("unit_ids"), 8)
        if type(record.get("incomplete")) is not bool:
            raise ValueError("Invalid checkpoint incompleteness signal")
        return {
            "path": self.bundle_directory / f"{handle}.context.json",
            "sha256": digest,
            "bytes": size,
            "snapshot_id": snapshot["id"],
            "snapshot": snapshot,
            "trust": trust,
            "bundle_incomplete": record["incomplete"],
            "created_unix": created,
            "expires_unix": expires,
            "relevant": {"symbol_ids": symbols, "unit_ids": units},
        }

    @staticmethod
    def _validate_checkpoint_ids(value: object, maximum: int) -> list[str]:
        if not isinstance(value, list) or len(value) > maximum:
            raise ValueError("Invalid checkpoint relevant IDs")
        if any(not isinstance(item, str) or len(item.encode("utf-8")) > 8192
               for item in value):
            raise ValueError("Invalid checkpoint relevant IDs")
        return list(value)

    @staticmethod
    def _validate_checkpoint_text(task: object, questions: object) -> None:
        if not isinstance(task, str) or not task or len(task.encode("utf-8")) > 8192:
            raise ValueError("Invalid checkpoint task")
        if not isinstance(questions, list) or len(questions) > 16:
            raise ValueError("Invalid unresolved questions")
        if any(not isinstance(item, str) or len(item.encode("utf-8")) > 2048
               for item in questions):
            raise ValueError("Invalid unresolved questions")

    @staticmethod
    def _normalize_next_retrieval(value: object) -> dict[str, Any]:
        if not isinstance(value, dict) or set(value) - {
            "query", "symbol_ids", "unit_ids", "depth"
        }:
            raise ValueError("Invalid next retrieval")
        query = value.get("query", "")
        symbols = value.get("symbol_ids", [])
        units = value.get("unit_ids", [])
        depth = value.get("depth", 1)
        if not isinstance(query, str) or len(query.encode("utf-8")) > 8192:
            raise ValueError("Invalid next retrieval")
        if not isinstance(symbols, list) or not isinstance(units, list):
            raise ValueError("Invalid next retrieval")
        if (len(symbols) > 12 or len(units) > 8 or
                type(depth) is not int or depth not in range(0, 5)):
            raise ValueError("Invalid next retrieval")
        if any(not isinstance(item, str) or len(item.encode("utf-8")) > 8192
               for item in symbols + units):
            raise ValueError("Invalid next retrieval")
        return {"query": query, "symbol_ids": symbols, "unit_ids": units, "depth": depth}

    def _now(self) -> int:
        return int(self._clock())

    @staticmethod
    def _validate_handle(handle: object, *, label: str = "bundle handle") -> None:
        if not isinstance(handle, str) or _HANDLE.fullmatch(handle) is None:
            raise ValueError(f"Invalid caller-controlled {label}")

    def _persist_reference(self, handle: str, payload: bytes, bundle: dict[str, Any]) -> str:
        if handle in self._persisted:
            raise ValueError("Bundle handle already exists")
        snapshot = bundle.get("snapshot")
        if not isinstance(snapshot, dict) or not isinstance(snapshot.get("id"), str):
            raise RuntimeError("File-reference delivery requires snapshot identity")
        if self._snapshot_id is not None and snapshot["id"] != self._snapshot_id:
            raise StaleEvidenceError("A persisted run cannot mix snapshot identities")
        if len(self._persisted) >= self.max_bundles:
            raise StorageLimitError("Persisted run reached its bundle count limit")
        stored_bytes = sum(item["bytes"] for item in self._persisted.values())
        if stored_bytes + len(payload) > self.max_store_bytes:
            raise StorageLimitError("Persisted run reached its byte limit")
        symbol_ids = [item["id"] for item in bundle.get("symbols", [])
                      if isinstance(item, dict) and isinstance(item.get("id"), str)]
        unit_ids = [item["id"] for item in bundle.get("units", [])
                    if isinstance(item, dict) and isinstance(item.get("id"), str)]
        omissions = bundle.get("omissions", {})
        created = self._now()
        metadata = {
            "path": self.bundle_directory / f"{handle}.context.json",
            "sha256": hashlib.sha256(payload).hexdigest(),
            "bytes": len(payload),
            "snapshot_id": snapshot["id"],
            "snapshot": {key: snapshot[key] for key in
                         ("id", "source_id", "profile_id", "verification")
                         if key in snapshot},
            "trust": bundle["trust"],
            "bundle_incomplete": _has_omissions(omissions),
            "created_unix": created,
            "expires_unix": created + self.retention_seconds,
        }
        reference = {
            "version": "repoctx.adapter.file-reference/v1alpha1",
            "delivery": "file_reference",
            "handle": handle,
            "run_id": self.run_id,
            "snapshot": metadata["snapshot"],
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
            "expires_unix": metadata["expires_unix"],
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
        metadata["relevant"] = {
            "symbol_ids": list(reference["relevant"]["symbol_ids"]),
            "unit_ids": list(reference["relevant"]["unit_ids"]),
        }
        self._persisted[handle] = metadata
        self._snapshot_id = snapshot["id"]
        return _compact(reference).decode("utf-8")

    @staticmethod
    def _publish(path: Path, payload: bytes) -> None:
        temporary = path.parent / f".{path.name}.{secrets.token_hex(8)}.tmp"
        flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_CLOEXEC", 0)
        flags |= getattr(os, "O_NOFOLLOW", 0)
        descriptor = None
        created = False
        published = False
        try:
            descriptor = os.open(temporary, flags, 0o600)
            created = True
            view = memoryview(payload)
            while view:
                written = os.write(descriptor, view)
                if written <= 0:
                    raise OSError("short bundle write")
                view = view[written:]
            os.fsync(descriptor)
            os.close(descriptor)
            descriptor = None
            os.link(temporary, path, follow_symlinks=False)
            published = True
            temporary.unlink()
            created = False
            directory = os.open(path.parent, os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
            try:
                os.fsync(directory)
            finally:
                os.close(directory)
        except Exception:
            if published:
                try:
                    path.unlink()
                except FileNotFoundError:
                    pass
            if created:
                try:
                    temporary.unlink()
                except FileNotFoundError:
                    pass
            raise
        finally:
            if descriptor is not None:
                os.close(descriptor)

    @staticmethod
    def _verified_bundle_bytes(metadata: dict[str, Any]) -> bytes:
        flags = os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) | getattr(os, "O_NOFOLLOW", 0)
        try:
            descriptor = os.open(metadata["path"], flags)
        except FileNotFoundError:
            raise MissingReferenceError("Persisted bundle is missing") from None
        try:
            info = os.fstat(descriptor)
            if (not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077 or
                    info.st_size != metadata["bytes"]):
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
