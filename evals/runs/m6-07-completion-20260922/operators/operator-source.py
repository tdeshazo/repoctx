#!/usr/bin/env python3
"""Run approved M6-07 actual-source treatment verification tasks once each."""

import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


REPOSITORY = Path("/home/travis/Workspace/repository_context_program/repoctx")
RUNNER_PATH = REPOSITORY / "scripts/run_first_use.py"
HELPER_PATH = REPOSITORY / "scripts/run_workflow_pilot.py"
SOURCE_REVISION = "20c335b3f0e6d82bdd6b0f52c605b1a7138e585e"
SOURCE_NAME = "repoctx-source@" + SOURCE_REVISION
CONDITION = "ordinary_tools_plus_repoctx"


def load_runner():
    spec = importlib.util.spec_from_file_location("m607_first_use_runner", RUNNER_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load first-use runner")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


runner = load_runner()
pilot = runner.pilot


def sha256_bytes(value):
    return hashlib.sha256(value).hexdigest()


def write_checksum(path):
    checksum = path.with_suffix(".sha256")
    checksum.write_text(pilot.sha256(path) + "  " + path.name + "\n", encoding="utf-8")


def capture(command, **kwargs):
    completed = subprocess.run(command, text=True, capture_output=True, check=False, **kwargs)
    return {"command": command, "returncode": completed.returncode,
            "stdout": completed.stdout, "stderr": completed.stderr}


def load_tasks(input_dir):
    tasks, seen = [], set()
    for path in sorted(input_dir.glob("*.json")):
        value = json.loads(path.read_text(encoding="utf-8"))
        if set(value) != {"id", "type", "repository", "prompt"}:
            raise ValueError("unexpected task schema: " + str(path))
        task_id = value["id"]
        if not isinstance(task_id, str) or not task_id or "/" in task_id or task_id in seen:
            raise ValueError("invalid or duplicate task id: " + str(path))
        if value["repository"] != SOURCE_NAME:
            raise ValueError("task does not identify the actual source export: " + str(path))
        if not isinstance(value["prompt"], str) or not value["prompt"].strip():
            raise ValueError("task prompt is empty: " + str(path))
        seen.add(task_id)
        tasks.append(value)
    if not tasks:
        raise ValueError("no task inputs found")
    return tasks


def filesystem_probe(args, run_dir, source_export):
    """Exercise the exact named pilot profile before an agent trial starts."""
    setup = run_dir / "setup"
    workspace, sibling = setup / "sandbox-a", setup / "sandbox-b"
    repository = workspace / "repository"
    shutil.copytree(source_export, repository)
    sibling.mkdir(parents=True)
    (sibling / "other-workspace.txt").write_text("not trial evidence\n", encoding="utf-8")
    tools_dir = workspace / ".tools"
    tools_dir.mkdir()
    binary = tools_dir / "repoctx"
    shutil.copy2(args.repoctx, binary)
    os.chmod(binary, 0o755)
    scratch = workspace / ".scratch"
    scratch.mkdir()
    marker = scratch / "profile-marker.txt"
    profile = pilot.filesystem_config(args.codex, binary)
    common = [str(args.codex), "sandbox", "-c", 'default_permissions="pilot"', "-c", profile,
              "-c", "permissions.pilot.network.enabled=false", "--permission-profile", "pilot",
              "-C", str(workspace)]
    shell = ["/bin/sh", "-c"]
    quoted = __import__("shlex").quote
    filesystem = capture(common + shell + [
        "printf probe > " + quoted(str(marker)) + " && test ! -r " + quoted(str(args.evaluator_key)) +
        " && test ! -r " + quoted(str(sibling / "other-workspace.txt")) + " && " +
        quoted(str(binary)) + " version -format json >/dev/null"], cwd=workspace)
    treatment = capture(common + shell + [
        "PATH=" + quoted(str(tools_dir) + os.pathsep + pilot.ORDINARY_PATH) +
        "; repoctx discover -root repository -query repoctx -max-bytes 12000"], cwd=workspace)
    socket = capture(common + ["python3", "-c", "import socket,sys\ntry: socket.socket()\nexcept PermissionError: sys.exit(0)\nelse: sys.exit(1)"], cwd=workspace)
    record = {
        "purpose": "non-model M6-07 actual-source named-profile probe",
        "workspace": str(workspace), "evaluator_denied": str(args.evaluator_key),
        "sibling_denied": str(sibling / "other-workspace.txt"), "pinned_repoctx": str(binary),
        "workspace_write_succeeded": filesystem["returncode"] == 0 and marker.is_file(),
        "evaluator_and_sibling_read_denied": filesystem["returncode"] == 0,
        "filesystem": filesystem, "treatment_tool_operation": treatment, "socket": socket,
        "versions": {"codex": capture([str(args.codex), "--version"]),
                     "repoctx": capture([str(binary), "version", "-format", "json"])}
    }
    pilot.write_json(setup / "probe.json", record)
    if filesystem["returncode"] or treatment["returncode"] or socket["returncode"]:
        raise RuntimeError("permission-profile probe failed; see " + str(setup / "probe.json"))
    return record


def snapshot_inputs(args, run_dir, tasks):
    operators = run_dir / "operators"
    operators.mkdir(parents=True)
    snapshots = {
        "operator": (Path(__file__).resolve(), operators / "operator-source.py"),
        "runner": (RUNNER_PATH, operators / "runner-source.py"),
        "runner_helper": (HELPER_PATH, operators / "runner-helper-source.py"),
    }
    result = {}
    for name, (source, destination) in snapshots.items():
        shutil.copy2(source, destination)
        result[name] = {"source_path": str(source), "snapshot_path": str(destination),
                        "sha256": pilot.sha256(destination)}
    inputs = run_dir / "task-inputs"
    inputs.mkdir()
    for task in tasks:
        shutil.copy2(args.input_dir / (task["id"] + ".json"), inputs / (task["id"] + ".json"))
    return result


def copy_archive(run_dir, archive_dir):
    if archive_dir.exists():
        raise ValueError("--archive-dir must not exist")
    archive_dir.mkdir(parents=True)
    files = [
        "protocol.initial.json", "protocol.initial.sha256", "protocol.json", "source-export-hashes.json",
        "trials.jsonl", "setup/probe.json",
        "operators/operator-source.py", "operators/runner-source.py", "operators/runner-helper-source.py",
    ]
    for task_input in sorted((run_dir / "task-inputs").glob("*.json")):
        files.append(task_input.relative_to(run_dir).as_posix())
    for trial in sorted((run_dir / "trials").iterdir()):
        files.append((trial / "record.json").relative_to(run_dir).as_posix())
        for raw in sorted((trial / "raw").iterdir()):
            files.append(raw.relative_to(run_dir).as_posix())
    manifest = {"classification": "M6-07 actual-source treatment verification; no comparative, efficiency, or generalization claim",
                "whitelist": files}
    pilot.write_json(run_dir / "archive-manifest.json", manifest)
    files.append("archive-manifest.json")
    for relative in files:
        source, destination = run_dir / relative, archive_dir / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, destination)
    records = []
    for path in sorted(archive_dir.rglob("*")):
        if path.is_file():
            records.append(pilot.sha256(path) + "  ./" + path.relative_to(archive_dir).as_posix())
    (archive_dir / "SHA256SUMS").write_text("\n".join(records) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-dir", type=Path, required=True)
    parser.add_argument("--archive-dir", type=Path)
    parser.add_argument("--input-dir", type=Path, required=True)
    parser.add_argument("--evaluator-key", type=Path, required=True)
    parser.add_argument("--repoctx", type=Path, required=True)
    parser.add_argument("--codex", type=Path, default=Path(shutil.which("codex") or "codex"))
    parser.add_argument("--auth-source", type=Path, default=Path.home() / ".codex/auth.json")
    parser.add_argument("--timeout-seconds", type=int, default=300)
    parser.add_argument("--probe-only", action="store_true")
    args = parser.parse_args()
    args.run_dir = args.run_dir.resolve()
    if args.archive_dir:
        args.archive_dir = args.archive_dir.resolve()
    args.input_dir = args.input_dir.resolve()
    args.evaluator_key = args.evaluator_key.resolve()
    args.repoctx = args.repoctx.resolve()
    args.codex = args.codex.resolve()
    args.auth_source = args.auth_source.resolve()
    runner.outside_repository(args.run_dir)
    if args.run_dir.exists() or args.timeout_seconds < 1:
        parser.error("--run-dir must not exist and --timeout-seconds must be positive")
    if args.probe_only and args.archive_dir:
        parser.error("--archive-dir is for a completed run, not --probe-only")
    for path in (args.input_dir, args.evaluator_key, args.repoctx, args.codex, args.auth_source):
        if not path.exists():
            parser.error("missing required path: " + str(path))
    if not args.input_dir.is_dir() or not args.evaluator_key.is_file():
        parser.error("task inputs must be a directory and evaluator key must be a file")
    tasks = load_tasks(args.input_dir)
    resolved = capture(["git", "rev-parse", "--verify", SOURCE_REVISION + "^{commit}"], cwd=REPOSITORY)
    if resolved["returncode"] or resolved["stdout"].strip() != SOURCE_REVISION:
        parser.error("requested source revision is unavailable or ambiguous")
    version = runner.capture([str(args.repoctx), "version", "-format", "json"])
    metadata = runner.verify_binary(version, SOURCE_REVISION)
    args.run_dir.mkdir(parents=True)
    snapshots = snapshot_inputs(args, args.run_dir, tasks)
    source_export = args.run_dir / "source-export"
    runner.SOURCE_REVISION = SOURCE_REVISION
    runner.export_source(source_export)
    source_hashes = runner.source_hashes(source_export)
    pilot.write_json(args.run_dir / "source-export-hashes.json", source_hashes)
    protocol = {
        "verification": "M6-07 actual-source treatment completion", "classification": "one treatment run per approved task; no comparative, efficiency, or generalization claim",
        "started_at": datetime.now(timezone.utc).isoformat(), "model": runner.MODEL,
        "model_revision": "unresolved_alias", "reasoning_effort": runner.REASONING_EFFORT,
        "timeout_seconds": args.timeout_seconds, "tool_call_cap": runner.TOOL_CALL_CAP,
        "tool_call_cap_enforcement": "advisory_prompt_only", "condition": CONDITION,
        "source": {"requested_revision": SOURCE_REVISION, "revision": SOURCE_REVISION, "name": SOURCE_NAME,
                   "export_sha256": pilot.sha256(args.run_dir / "source-export-hashes.json"), "source_sha256": source_hashes,
                   "excluded": ["evals/", "scripts/", "docs/reports/", "docs/history/", ".agents/", ".codex/", ".git/", "build/", "dist/"]},
        "tasks": [{"id": task["id"], "type": task["type"], "input_sha256": pilot.sha256(args.input_dir / (task["id"] + ".json")),
                   "prompt_sha256": sha256_bytes(task["prompt"].encode("utf-8"))} for task in tasks],
        "evaluator_key": {"path": str(args.evaluator_key.resolve()), "sha256": pilot.sha256(args.evaluator_key)},
        "repoctx": {"path": str(args.repoctx.resolve()), "sha256": pilot.sha256(args.repoctx), "version": version,
                    "metadata": metadata, "operator_setup_cost_excluded": True},
        "codex": {"path": str(args.codex.resolve()), "version": runner.capture([str(args.codex), "--version"])},
        "operator_sources": snapshots,
        "limits": "Named pilot profile: minimal read, workspace-root write, root/tmp deny, network disabled. Each treatment workspace contains only the actual source export and actual repoctx skill; scratch is writable only inside that workspace.",
    }
    pilot.write_json(args.run_dir / "protocol.json", protocol)
    shutil.copy2(args.run_dir / "protocol.json", args.run_dir / "protocol.initial.json")
    write_checksum(args.run_dir / "protocol.initial.json")
    filesystem_probe(args, args.run_dir, source_export)
    protocol["setup_probe_sha256"] = pilot.sha256(args.run_dir / "setup/probe.json")
    if args.probe_only:
        protocol["result"] = "probe_only"
        protocol["finished_at"] = datetime.now(timezone.utc).isoformat()
        pilot.write_json(args.run_dir / "protocol.json", protocol)
        return
    for position, task in enumerate(tasks, 1):
        runner.run_trial({"task": task, "condition": CONDITION, "position": position}, args, args.run_dir, source_export, source_hashes)
    protocol["finished_at"] = datetime.now(timezone.utc).isoformat()
    pilot.write_json(args.run_dir / "protocol.json", protocol)
    if args.archive_dir:
        copy_archive(args.run_dir, args.archive_dir)


if __name__ == "__main__":
    main()
