#!/usr/bin/env python3
"""Run one post-pilot M6-07 relationship treatment replay with frozen overlays."""

import argparse
from datetime import datetime, timezone
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess


REPOSITORY = Path("/home/travis/Workspace/repository_context_program/repoctx")
RUNNER_PATH = REPOSITORY / "scripts/run_first_use.py"
TASK_ID = "f-context-go-relationships"
CONDITION = "ordinary_tools_plus_repoctx"


def load_runner():
    spec = importlib.util.spec_from_file_location("first_use_runner", RUNNER_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load frozen first-use runner")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


runner = load_runner()
pilot = runner.pilot


def copy_overlay(run_dir, source_export, source, destination):
    snapshot = run_dir / "overlay" / destination
    snapshot.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, snapshot)
    target = source_export / destination
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(snapshot, target)
    return {
        "source_path": str(source.resolve()), "snapshot_path": str(snapshot),
        "destination": destination, "sha256": pilot.sha256(snapshot),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-dir", type=Path, required=True)
    parser.add_argument("--repoctx", type=Path, required=True)
    parser.add_argument("--codex", type=Path, default=Path(shutil.which("codex") or "codex"))
    parser.add_argument("--auth-source", type=Path, default=Path.home() / ".codex/auth.json")
    parser.add_argument("--overlay-readme", type=Path, required=True)
    parser.add_argument("--overlay-skill", type=Path, required=True)
    parser.add_argument("--timeout-seconds", type=int, default=300)
    parser.add_argument("--probe-only", action="store_true")
    args = parser.parse_args()
    args.run_dir = args.run_dir.resolve()
    runner.outside_repository(args.run_dir)
    if args.run_dir.exists() or args.timeout_seconds < 1:
        parser.error("--run-dir must not exist and --timeout-seconds must be positive")
    for path in (args.repoctx, args.codex, args.auth_source, args.overlay_readme, args.overlay_skill):
        if not path.is_file():
            parser.error("missing required file: " + str(path))
    resolved = subprocess.run(["git", "rev-parse", "--verify", runner.SOURCE_REVISION + "^{commit}"],
                              cwd=REPOSITORY, text=True, capture_output=True, check=False)
    if resolved.returncode:
        parser.error("missing pinned source revision: " + runner.SOURCE_REVISION)
    freeze_sha256 = runner.verify_freeze(runner.CORPUS / "evaluator/freeze.sha256")
    args.input_dir = runner.CORPUS / "agent_inputs"
    task = next(task for task in runner.load_tasks(args.input_dir) if task["id"] == TASK_ID)
    binary_version = runner.capture([str(args.repoctx), "version", "-format", "json"])
    binary_metadata = runner.verify_binary(binary_version, resolved.stdout.strip())

    args.run_dir.mkdir(parents=True)
    source_export = args.run_dir / "source-export"
    runner.export_source(source_export)
    original_hashes = runner.source_hashes(source_export)
    pilot.write_json(args.run_dir / "original-source-hashes.json", original_hashes)
    overlays = [
        copy_overlay(args.run_dir, source_export, args.overlay_readme, "README.md"),
        copy_overlay(args.run_dir, source_export, args.overlay_skill, "skills/repoctx/SKILL.md"),
    ]
    effective_hashes = runner.source_hashes(source_export)
    pilot.write_json(args.run_dir / "effective-source-hashes.json", effective_hashes)
    guidance_version = "sha256:" + pilot.sha256(args.run_dir / "overlay/README.md")[:16] + "+" + pilot.sha256(args.run_dir / "overlay/skills/repoctx/SKILL.md")[:16]
    protocol = {
        "replay": "M6-07-postpilot-targeted-relationship-treatment",
        "classification": "targeted development replay; not paired, held-out, or aggregate evaluation evidence",
        "started_at": datetime.now(timezone.utc).isoformat(),
        "model": runner.MODEL, "model_revision": "unresolved_alias", "reasoning_effort": runner.REASONING_EFFORT,
        "timeout_seconds": args.timeout_seconds, "tool_call_cap": runner.TOOL_CALL_CAP,
        "tool_call_cap_enforcement": "advisory_prompt_only", "task_id": task["id"],
        "condition": CONDITION, "input_sha256": pilot.sha256(runner.CORPUS / "agent_inputs" / (TASK_ID + ".json")),
        "freeze_sha256": freeze_sha256,
        "source": {"requested_revision": runner.SOURCE_REVISION, "revision": resolved.stdout.strip(),
                   "original_source_sha256": original_hashes,
                   "original_export_sha256": pilot.sha256(args.run_dir / "original-source-hashes.json"),
                   "effective_source_sha256": effective_hashes,
                   "effective_export_sha256": pilot.sha256(args.run_dir / "effective-source-hashes.json")},
        "overlays": overlays, "guidance_version": guidance_version,
        "repoctx": {"path": str(args.repoctx.resolve()), "sha256": pilot.sha256(args.repoctx),
                    "version": binary_version, "metadata": binary_metadata, "operator_setup_cost_excluded": True},
        "runner": {"path": str(RUNNER_PATH), "sha256": pilot.sha256(RUNNER_PATH)},
        "runner_helper": {"path": str(Path(pilot.__file__).resolve()), "sha256": pilot.sha256(Path(pilot.__file__).resolve())},
        "limits": "Named pilot profile: minimal read, workspace-root write, root/tmp deny, network disabled. The sole treatment workspace contains a frozen source export plus frozen README and repoctx-skill overlays. Scratch is writable only inside that workspace.",
    }
    shutil.copy2(Path(__file__).resolve(), args.run_dir / "operator-source.py")
    shutil.copy2(RUNNER_PATH, args.run_dir / "runner-source.py")
    shutil.copy2(Path(pilot.__file__).resolve(), args.run_dir / "runner-helper-source.py")
    pilot.write_json(args.run_dir / "protocol.json", protocol)
    shutil.copy2(args.run_dir / "protocol.json", args.run_dir / "protocol.initial.json")
    (args.run_dir / "protocol.initial.sha256").write_text(
        pilot.sha256(args.run_dir / "protocol.initial.json") + "  protocol.initial.json\n", encoding="utf-8")

    runner.setup_probe(args, args.run_dir, source_export)
    protocol["setup_probe_sha256"] = pilot.sha256(args.run_dir / "setup/probe.json")
    if args.probe_only:
        protocol["result"] = "probe_only"
        protocol["finished_at"] = datetime.now(timezone.utc).isoformat()
        pilot.write_json(args.run_dir / "protocol.json", protocol)
        return
    runner.run_trial({"task": task, "condition": CONDITION, "position": 1}, args, args.run_dir,
                     source_export, effective_hashes)
    protocol["finished_at"] = datetime.now(timezone.utc).isoformat()
    pilot.write_json(args.run_dir / "protocol.json", protocol)


if __name__ == "__main__":
    main()
