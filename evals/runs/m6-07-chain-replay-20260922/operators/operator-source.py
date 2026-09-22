#!/usr/bin/env python3
"""Run one tuned M6-07 artifacts-chain treatment replay with a skill-only overlay."""

import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess


REPOSITORY = Path("/home/travis/Workspace/repository_context_program/repoctx")
RUNNER_PATH = REPOSITORY / "scripts/run_first_use.py"
HELPER_PATH = REPOSITORY / "scripts/run_workflow_pilot.py"
SOURCE_REVISION = "20c335b3f0e6d82bdd6b0f52c605b1a7138e585e"
SOURCE_NAME = "repoctx-source@" + SOURCE_REVISION
TASK_ID = "f-artifacts-check-chain"
CONDITION = "ordinary_tools_plus_repoctx"
CORPUS = REPOSITORY / "evals/first-use/completion"


def load_runner():
    spec = importlib.util.spec_from_file_location("m607_first_use_runner", RUNNER_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load first-use runner")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


runner = load_runner()
pilot = runner.pilot


def capture(command, **kwargs):
    completed = subprocess.run(command, text=True, capture_output=True, check=False, **kwargs)
    return {"command": command, "returncode": completed.returncode,
            "stdout": completed.stdout, "stderr": completed.stderr}


def verify_completion_freeze(path):
    for line in path.read_text(encoding="utf-8").splitlines():
        expected, relative = line.split("  ", 1)
        target = (CORPUS / relative).resolve()
        target.relative_to(CORPUS.resolve())
        if not target.is_file() or pilot.sha256(target) != expected:
            raise ValueError("frozen completion corpus does not match: " + relative)
    return pilot.sha256(path)


def load_task(path):
    value = json.loads(path.read_text(encoding="utf-8"))
    if set(value) != {"id", "type", "repository", "prompt"} or value["id"] != TASK_ID:
        raise ValueError("unexpected replay task input")
    if value["repository"] != SOURCE_NAME or not isinstance(value["prompt"], str) or not value["prompt"].strip():
        raise ValueError("task does not identify the pinned source export")
    return value


def write_checksum(path):
    path.with_suffix(".sha256").write_text(pilot.sha256(path) + "  " + path.name + "\n", encoding="utf-8")


def snapshot_file(source, destination):
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)
    return {"source_path": str(source), "snapshot_path": str(destination), "sha256": pilot.sha256(destination)}


def setup_probe(args, run_dir, source_export):
    setup = run_dir / "setup"
    workspace, sibling = setup / "sandbox-a", setup / "sandbox-b"
    repository = workspace / "repository"
    shutil.copytree(source_export, repository)
    sibling.mkdir(parents=True)
    sentinel = sibling / "other-workspace.txt"
    sentinel.write_text("not trial evidence\n", encoding="utf-8")
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
    filesystem = capture(common + ["/bin/sh", "-c",
        "printf probe > " + shlex.quote(str(marker)) + " && test ! -r " + shlex.quote(str(args.evaluator_key)) +
        " && test ! -r " + shlex.quote(str(sentinel)) + " && " + shlex.quote(str(binary)) + " version -format json >/dev/null"], cwd=workspace)
    treatment = capture(common + ["/bin/sh", "-c", "PATH=" + shlex.quote(str(tools_dir) + os.pathsep + pilot.ORDINARY_PATH) +
                         "; repoctx discover -root repository -query repoctx -max-bytes 12000"], cwd=workspace)
    socket = capture(common + ["python3", "-c", "import socket,sys\ntry: socket.socket()\nexcept PermissionError: sys.exit(0)\nelse: sys.exit(1)"], cwd=workspace)
    record = {"purpose": "non-model M6-07 tuned-chain-replay named-profile probe", "workspace": str(workspace),
              "evaluator_denied": str(args.evaluator_key), "sibling_denied": str(sentinel), "pinned_repoctx": str(binary),
              "workspace_write_succeeded": filesystem["returncode"] == 0 and marker.is_file(),
              "evaluator_and_sibling_read_denied": filesystem["returncode"] == 0,
              "filesystem": filesystem, "treatment_tool_operation": treatment, "socket": socket,
              "versions": {"codex": capture([str(args.codex), "--version"]), "repoctx": capture([str(binary), "version", "-format", "json"])}}
    pilot.write_json(setup / "probe.json", record)
    if filesystem["returncode"] or treatment["returncode"] or socket["returncode"]:
        raise RuntimeError("permission-profile probe failed; see " + str(setup / "probe.json"))


def archive(run_dir, archive_dir):
    if archive_dir.exists():
        raise ValueError("--archive-dir must not exist")
    archive_dir.mkdir(parents=True)
    files = ["protocol.initial.json", "protocol.initial.sha256", "protocol.json", "original-source-hashes.json", "effective-source-hashes.json", "setup/probe.json", "trials.jsonl", "task-inputs/f-artifacts-check-chain.json", "overlay/skills/repoctx/SKILL.md", "operators/operator-source.py", "operators/runner-source.py", "operators/runner-helper-source.py", "trials/01-f-artifacts-check-chain-ordinary_tools_plus_repoctx/record.json"]
    files += [path.relative_to(run_dir).as_posix() for path in sorted((run_dir / "trials/01-f-artifacts-check-chain-ordinary_tools_plus_repoctx/raw").iterdir())]
    pilot.write_json(run_dir / "archive-manifest.json", {"classification": "tuned same-task M6-07 replay with a skill-only overlay; not independent fresh, comparative, efficiency, or generalization evidence", "whitelist": files})
    files.append("archive-manifest.json")
    for relative in files:
        target = archive_dir / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(run_dir / relative, target)
    sums = [pilot.sha256(path) + "  ./" + path.relative_to(archive_dir).as_posix() for path in sorted(archive_dir.rglob("*")) if path.is_file()]
    (archive_dir / "SHA256SUMS").write_text("\n".join(sums) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-dir", type=Path, required=True)
    parser.add_argument("--archive-dir", type=Path, required=True)
    parser.add_argument("--repoctx", type=Path, required=True)
    parser.add_argument("--overlay-skill", type=Path, required=True)
    parser.add_argument("--codex", type=Path, default=Path(shutil.which("codex") or "codex"))
    parser.add_argument("--auth-source", type=Path, default=Path.home() / ".codex/auth.json")
    parser.add_argument("--timeout-seconds", type=int, default=300)
    args = parser.parse_args()
    args.run_dir, args.archive_dir = args.run_dir.resolve(), args.archive_dir.resolve()
    args.repoctx, args.overlay_skill = args.repoctx.resolve(), args.overlay_skill.resolve()
    args.codex, args.auth_source = args.codex.resolve(), args.auth_source.resolve()
    args.evaluator_key = (CORPUS / "evaluator/expected.json").resolve()
    input_path = (CORPUS / "agent_inputs" / (TASK_ID + ".json")).resolve()
    runner.outside_repository(args.run_dir)
    if args.run_dir.exists() or args.archive_dir.exists() or args.timeout_seconds < 1:
        parser.error("run/archive directories must not exist and timeout must be positive")
    for path in (args.repoctx, args.overlay_skill, args.codex, args.auth_source, args.evaluator_key, input_path):
        if not path.is_file():
            parser.error("missing required file: " + str(path))
    freeze_sha256, task = verify_completion_freeze(CORPUS / "evaluator/freeze.sha256"), load_task(input_path)
    resolved = capture(["git", "rev-parse", "--verify", SOURCE_REVISION + "^{commit}"], cwd=REPOSITORY)
    if resolved["returncode"] or resolved["stdout"].strip() != SOURCE_REVISION:
        parser.error("pinned source revision unavailable")
    version = runner.capture([str(args.repoctx), "version", "-format", "json"])
    metadata = runner.verify_binary(version, SOURCE_REVISION)
    args.run_dir.mkdir(parents=True)
    operators = args.run_dir / "operators"
    snapshots = {"operator": snapshot_file(Path(__file__).resolve(), operators / "operator-source.py"),
                 "runner": snapshot_file(RUNNER_PATH, operators / "runner-source.py"),
                 "runner_helper": snapshot_file(HELPER_PATH, operators / "runner-helper-source.py")}
    snapshot_file(input_path, args.run_dir / "task-inputs" / input_path.name)
    overlay = snapshot_file(args.overlay_skill, args.run_dir / "overlay/skills/repoctx/SKILL.md")
    source_export = args.run_dir / "source-export"
    runner.SOURCE_REVISION = SOURCE_REVISION
    runner.export_source(source_export)
    original = runner.source_hashes(source_export)
    pilot.write_json(args.run_dir / "original-source-hashes.json", original)
    shutil.copy2(args.run_dir / "overlay/skills/repoctx/SKILL.md", source_export / "skills/repoctx/SKILL.md")
    effective = runner.source_hashes(source_export)
    pilot.write_json(args.run_dir / "effective-source-hashes.json", effective)
    protocol = {"verification": "M6-07 tuned artifacts-chain replay", "classification": "tuned replay of the same prior task with only the current repoctx skill overlaid; not independent fresh, comparative, efficiency, or generalization evidence", "started_at": datetime.now(timezone.utc).isoformat(), "model": runner.MODEL, "model_revision": "unresolved_alias", "reasoning_effort": runner.REASONING_EFFORT, "timeout_seconds": args.timeout_seconds, "tool_call_cap": runner.TOOL_CALL_CAP, "tool_call_cap_enforcement": "advisory_prompt_only", "task_id": TASK_ID, "condition": CONDITION, "freeze_sha256": freeze_sha256, "task_input_sha256": pilot.sha256(input_path), "evaluator_key": {"path": str(args.evaluator_key), "sha256": pilot.sha256(args.evaluator_key)}, "source": {"requested_revision": SOURCE_REVISION, "revision": SOURCE_REVISION, "name": SOURCE_NAME, "original_source_sha256": original, "original_export_sha256": pilot.sha256(args.run_dir / "original-source-hashes.json"), "effective_source_sha256": effective, "effective_export_sha256": pilot.sha256(args.run_dir / "effective-source-hashes.json")}, "overlay": {**overlay, "destination": "skills/repoctx/SKILL.md", "bytes": (args.run_dir / "overlay/skills/repoctx/SKILL.md").stat().st_size, "only_overlay": True}, "repoctx": {"path": str(args.repoctx), "sha256": pilot.sha256(args.repoctx), "version": version, "metadata": metadata, "operator_setup_cost_excluded": True}, "codex": {"path": str(args.codex), "version": runner.capture([str(args.codex), "--version"])}, "operator_sources": snapshots, "limits": "Named pilot profile: minimal read, workspace-root write, root/tmp deny, network disabled. The sole treatment workspace receives a source export with only skills/repoctx/SKILL.md overlaid; scratch is writable only inside that workspace."}
    pilot.write_json(args.run_dir / "protocol.json", protocol)
    shutil.copy2(args.run_dir / "protocol.json", args.run_dir / "protocol.initial.json")
    write_checksum(args.run_dir / "protocol.initial.json")
    setup_probe(args, args.run_dir, source_export)
    protocol["setup_probe_sha256"] = pilot.sha256(args.run_dir / "setup/probe.json")
    args.input_dir = input_path.parent
    runner.run_trial({"task": task, "condition": CONDITION, "position": 1}, args, args.run_dir, source_export, effective)
    protocol["finished_at"] = datetime.now(timezone.utc).isoformat()
    pilot.write_json(args.run_dir / "protocol.json", protocol)
    archive(args.run_dir, args.archive_dir)


if __name__ == "__main__":
    main()
