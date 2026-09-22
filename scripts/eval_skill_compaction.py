#!/usr/bin/env python3
"""Compare two repoctx skill variants in isolated Codex first-use trials."""

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

import run_workflow_pilot as pilot


ROOT = Path(__file__).resolve().parents[1]
FIXTURE = ROOT / "evals/pilot/repositories/relay-board"
CASES = (
    {
        "id": "provided-evidence",
        "prompt": "The complete evidence is: 'The dispatch service waits two minutes before retrying a queued report.' How long does it wait? Answer from the supplied evidence without inspecting the repository.",
        "required": ("two minutes",),
        "needs_retrieval": False,
    },
    {
        "id": "locate-queue",
        "prompt": "Locate the function that selects a dispatch queue from a report's region and expedited flag. Give its repository path and function name.",
        "required": ("relay/classify.py", "choose_queue"),
        "needs_retrieval": True,
    },
    {
        "id": "trace-dispatch",
        "prompt": "Trace prepare_dispatch across the repository. Explain how it validates the recipient batch and selects the returned queue. Name the participating functions and configuration values.",
        "required": ("accepts_batch_size", "MAX_BATCH_SIZE", "choose_queue", "DEFAULT_QUEUE"),
        "needs_retrieval": True,
    },
    {
        "id": "indexed-trace",
        "prompt": "Use repoctx's indexed context workflow to trace prepare_dispatch's outgoing calls across files. Cite the callsite for each caller-to-callee link, then explain the batch and queue configuration values. Keep the claim bounded by the returned graph evidence.",
        "required": ("accepts_batch_size", "MAX_BATCH_SIZE", "choose_queue", "DEFAULT_QUEUE"),
        "needs_retrieval": True,
        "handoff": True,
    },
)


def check_binary(binary: Path) -> dict:
    version = subprocess.run([str(binary), "version", "-format", "json"],
                             text=True, capture_output=True, check=True)
    info = json.loads(version.stdout)
    if info.get("program") != "repoctx" or not info.get("contracts", {}).get("discovery"):
        raise ValueError("binary lacks repoctx discovery capability metadata")
    probe = subprocess.run([str(binary), "discover", "-root", str(FIXTURE),
                            "-query", "dispatch", "-max-bytes", "4000"],
                           text=True, capture_output=True, check=True)
    if not json.loads(probe.stdout).get("results"):
        raise ValueError("binary discovery probe returned no results")
    with tempfile.TemporaryDirectory(prefix="repoctx-skill-eval-index-") as temp:
        index = Path(temp) / "repo.ir.json.gz"
        subprocess.run([str(binary), "compile", "-root", str(FIXTURE), "-o", str(index)],
                       text=True, capture_output=True, check=True)
        subprocess.run([str(binary), "validate", str(index)],
                       text=True, capture_output=True, check=True)
        context = subprocess.run([str(binary), "context", "-root", str(FIXTURE),
                                  "-query", "prepare_dispatch", "-max-bytes", "12000", str(index)],
                                 text=True, capture_output=True, check=True)
        json.loads(context.stdout)
    return {"sha256": pilot.sha256(binary), "version": info}


def run_case(args, case: dict, variant: str, position: int) -> dict:
    trial = args.run_dir / "trials" / f"{position:02d}-{case['id']}-{variant}"
    workspace = trial / "workspace"
    repository = workspace / "repository"
    trial.mkdir(parents=True)
    shutil.copytree(FIXTURE, repository)
    skill = workspace / ".agents/skills/repoctx"
    skill.parent.mkdir(parents=True)
    shutil.copytree(args.baseline_skill if variant == "baseline" else args.candidate_skill, skill)
    tools_dir = workspace / ".tools"
    tools_dir.mkdir()
    binary = tools_dir / "repoctx"
    shutil.copy2(args.repoctx, binary)
    binary.chmod(0o755)
    if pilot.sha256(binary) != args.binary_info["sha256"]:
        raise ValueError("trial binary changed after integration validation")
    scratch = workspace / ".scratch"
    scratch.mkdir()
    raw = trial / "raw"
    raw.mkdir()
    events_path, stderr_path, answer_path = raw / "events.jsonl", raw / "stderr.txt", raw / "final.txt"
    prompt = case["prompt"] + (
        "\n\nThe supplied repository is in `repository/`. Work only in that repository. "
        "Do not use the network. Tools on PATH are available. Use at most 16 tool calls. "
        "Give a concise final answer."
    )
    if case.get("handoff"):
        prompt += (
            "\n\nTrusted caller setup: the executable bytes now at `" + str(binary) +
            "` were validated for version, discover, compile, validate, and context "
            "with this repository before the trial. Their SHA-256 was checked after copying."
        )
    source_before = pilot.source_hashes(repository)
    auth_home = Path(tempfile.mkdtemp(prefix="repoctx-skill-eval-auth-"))
    try:
        shutil.copy2(args.auth_source, auth_home / "auth.json")
        (auth_home / "auth.json").chmod(0o600)
        environment = {"PATH": str(tools_dir) + os.pathsep + pilot.ORDINARY_PATH,
                       "CODEX_HOME": str(auth_home), "TMPDIR": str(scratch)}
        command = pilot.trial_command(args.codex, workspace, binary)
        exit_code, timed_out, seconds = pilot.run_process(
            command, prompt, workspace, environment,
            events_path, stderr_path, answer_path, args.timeout_seconds,
        )
    finally:
        shutil.rmtree(auth_home, ignore_errors=True)
    events, malformed = pilot.parse_events(events_path)
    tools = pilot.observed_tool_items(events)
    command_items = [event["item"] for event in tools
                     if event["item"].get("type") == "command_execution"]
    commands = [item.get("command", "") for item in command_items]
    successful_commands = [item.get("command", "") for item in command_items
                           if item.get("exit_code") == 0]
    answer = answer_path.read_text(encoding="utf-8", errors="replace") if answer_path.exists() else ""
    completed = next((event for event in reversed(events) if event.get("type") == "turn.completed"), None)
    record = {
        "case": case["id"], "variant": variant, "position": position,
        "exit_code": exit_code, "timed_out": timed_out, "wall_seconds": seconds,
        "usage": completed.get("usage") if completed else None,
        "answer": answer, "answer_pass": all(term.lower() in answer.lower() for term in case["required"]),
        "skill_read": any("SKILL.md" in command and "repoctx" in command
                          for command in successful_commands),
        "reference_read": any("references/" in command for command in successful_commands),
        "repoctx_retrieval": any("repoctx " + subcommand in command
                                 for command in successful_commands for subcommand in
                                 ("discover", "files", "search", "read", "context")),
        "version_check": any("repoctx version" in command for command in commands),
        "indexed_context": any("repoctx context" in command for command in successful_commands),
        "nonzero_commands": sum(item.get("exit_code") not in (None, 0) for item in command_items),
        "commands": commands, "tool_items": len(tools), "tool_call_cap": 16,
        "tool_call_cap_respected": len(tools) <= 16, "malformed_events": malformed,
        "source_unchanged": pilot.source_hashes(repository) == source_before,
    }
    record["routing_pass"] = record["skill_read"] == case["needs_retrieval"]
    pilot.write_json(trial / "record.json", record)
    print(json.dumps({key: record[key] for key in
                      ("case", "variant", "exit_code", "answer_pass", "skill_read",
                       "reference_read", "repoctx_retrieval", "version_check", "usage")}), flush=True)
    return record


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline-skill", type=Path, required=True)
    parser.add_argument("--candidate-skill", type=Path, default=ROOT / "skills/repoctx")
    parser.add_argument("--repoctx", type=Path, required=True)
    parser.add_argument("--run-dir", type=Path, required=True)
    parser.add_argument("--codex", type=Path, default=Path(shutil.which("codex") or "codex"))
    parser.add_argument("--auth-source", type=Path, default=Path.home() / ".codex/auth.json")
    parser.add_argument("--timeout-seconds", type=int, default=300)
    parser.add_argument("--case", action="append", choices=[case["id"] for case in CASES])
    parser.add_argument("--variant", action="append", choices=("baseline", "candidate"))
    args = parser.parse_args()
    args.run_dir = args.run_dir.resolve()
    pilot.outside_repository(args.run_dir)
    for path in (args.baseline_skill / "SKILL.md", args.candidate_skill / "SKILL.md",
                 args.repoctx, args.codex, args.auth_source):
        if not path.is_file():
            parser.error("missing required file: " + str(path))
    if args.run_dir.exists() or args.timeout_seconds < 1:
        parser.error("--run-dir must not exist and timeout must be positive")
    selected = [case for case in CASES if args.case is None or case["id"] in args.case]
    args.run_dir.mkdir(parents=True)
    args.binary_info = check_binary(args.repoctx)
    protocol = {
        "started_at": datetime.now(timezone.utc).isoformat(),
        "purpose": "exploratory SkillReducer routing and task retention comparison",
        "model": pilot.MODEL, "reasoning_effort": pilot.REASONING_EFFORT,
        "fixture_sha256": pilot.source_hashes(FIXTURE),
        "skill_sha256": {variant: pilot.sha256(path / "SKILL.md") for variant, path in
                         (("baseline", args.baseline_skill), ("candidate", args.candidate_skill))},
        "binary": args.binary_info,
        "cases": selected,
    }
    pilot.write_json(args.run_dir / "protocol.json", protocol)
    records = []
    for case_index, case in enumerate(selected):
        order = ("baseline", "candidate") if case_index % 2 == 0 else ("candidate", "baseline")
        if args.variant:
            order = tuple(variant for variant in order if variant in args.variant)
        for variant in order:
            records.append(run_case(args, case, variant, len(records) + 1))
    pilot.write_json(args.run_dir / "summary.json", records)


if __name__ == "__main__":
    main()
