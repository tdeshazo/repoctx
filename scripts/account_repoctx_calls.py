#!/usr/bin/env python3
"""Replay saved shell events and account for Repoctx command invocations."""

from __future__ import annotations

import argparse
from collections import Counter
import json
from pathlib import Path
import shlex
from typing import Any, Iterable


SUBCOMMANDS = frozenset({
    "discover", "overview", "files", "search", "read", "context",
    "compile", "artifacts", "manifest", "validate", "stats", "graph",
    "version", "help",
})
SHELLS = frozenset({"sh", "bash", "dash", "zsh", "ksh"})
COMMAND_WRAPPERS = frozenset({"command", "exec", "builtin"})
SHELL_OPERATORS = frozenset({";", "&&", "||", "|", "&", "(", ")"})
EOF_ERROR = "line range exceeds observed file"


def executable_name(token: str) -> str:
    """Return a shell executable's basename, including for quoted paths."""
    return token.replace("\\", "/").rsplit("/", 1)[-1]


def is_assignment(token: str) -> bool:
    name, separator, _ = token.partition("=")
    return bool(
        separator and name and name.replace("_", "a").isalnum()
        and not name[0].isdigit()
    )


def command_tokens(tokens: list[str]) -> list[str]:
    """Skip shell assignments and simple command wrappers before argv[0]."""
    words = list(tokens)
    while words and is_assignment(words[0]):
        words.pop(0)
    while words and executable_name(words[0]) in COMMAND_WRAPPERS:
        words.pop(0)
        while words and words[0].startswith("-"):
            words.pop(0)
        while words and is_assignment(words[0]):
            words.pop(0)
    if words and executable_name(words[0]) == "env":
        words.pop(0)
        while words and (words[0].startswith("-") or is_assignment(words[0])):
            words.pop(0)
        while words and is_assignment(words[0]):
            words.pop(0)
        return command_tokens(words)
    return words


def shell_script(command: str, *, depth: int = 0) -> str:
    """Unwrap recorded `sh -c`-style invocations to their executed script."""
    if depth > 8:
        raise ValueError("too many nested shell command strings")
    try:
        words = shlex.split(command, posix=True)
    except ValueError as exc:
        raise ValueError(f"cannot parse command string: {exc}") from exc
    words = command_tokens(words)
    if not words or executable_name(words[0]) not in SHELLS:
        return command

    shell_args = words[1:]
    for index, word in enumerate(shell_args):
        is_command_flag = word in {"-c", "--command"} or (
            word.startswith("-") and not word.startswith("--") and "c" in word[1:]
        )
        if is_command_flag:
            if index + 1 >= len(shell_args):
                raise ValueError("shell command flag has no script argument")
            return shell_script(shell_args[index + 1], depth=depth + 1)
    return command


def split_shell_commands(script: str) -> list[list[str]]:
    """Split a shell script at common command separators, respecting quotes."""
    try:
        lexer = shlex.shlex(script, posix=True, punctuation_chars=";&|()")
        lexer.whitespace_split = True
        words = list(lexer)
    except ValueError as exc:
        raise ValueError(f"cannot parse shell script: {exc}") from exc

    commands: list[list[str]] = []
    current: list[str] = []
    for word in words:
        if word in SHELL_OPERATORS:
            if current:
                commands.append(current)
                current = []
        else:
            current.append(word)
    if current:
        commands.append(current)
    return commands


def recognize_subcommands(command: str) -> list[str]:
    """Recognize direct Repoctx invocations in a recorded shell command.

    Both a PATH lookup (`repoctx read ...`) and an explicit executable path
    (`/tools/repoctx read ...`) are supported. Arguments mentioning Repoctx
    are ignored because only command-position executable tokens are checked.
    """
    script = shell_script(command)
    found = []
    for words in split_shell_commands(script):
        words = command_tokens(words)
        if len(words) < 2 or executable_name(words[0]) != "repoctx":
            continue
        subcommand = words[1]
        if subcommand in SUBCOMMANDS:
            found.append(subcommand)
    return found


def _event_output(item: dict[str, Any]) -> str:
    output = item.get("aggregated_output", "")
    return output if isinstance(output, str) else ""


def account_event_file(path: Path) -> list[dict[str, Any]]:
    """Return recognized Repoctx invocations from completed event records."""
    records = []
    seen_ids: set[str] = set()
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(f"{path}:{line_number}: invalid JSON: {exc}") from exc
        if not isinstance(event, dict):
            raise ValueError(f"{path}:{line_number}: event must be an object")
        item = event.get("item", {})
        if (event.get("type") != "item.completed" or not isinstance(item, dict)
                or item.get("type") != "command_execution"):
            continue
        command = item.get("command", "")
        if not isinstance(command, str):
            continue
        subcommands = recognize_subcommands(command)
        if not subcommands:
            continue

        event_id = item.get("id")
        if isinstance(event_id, str):
            if event_id in seen_ids:
                raise ValueError(f"{path}:{line_number}: duplicate completed event ID {event_id}")
            seen_ids.add(event_id)

        output = _event_output(item)
        exit_code = item.get("exit_code")
        eof_failure = "read" in subcommands and EOF_ERROR in output
        failed = type(exit_code) is int and exit_code != 0
        records.append({
            "source": str(path),
            "line": line_number,
            "event_id": event_id,
            "subcommands": subcommands,
            "exit_code": exit_code,
            "failure_class": "eof_read" if eof_failure else "other" if failed else None,
            "command": command,
        })
    return records


def event_files(inputs: Iterable[Path]) -> list[Path]:
    """Expand file and directory inputs, preserving order without duplicates."""
    paths: list[Path] = []
    seen: set[Path] = set()
    for value in inputs:
        candidates = [value] if value.is_file() else sorted(value.rglob("events.jsonl"))
        if not candidates and not value.exists():
            raise ValueError(f"input path does not exist: {value}")
        for path in candidates:
            resolved = path.resolve()
            if resolved in seen:
                continue
            seen.add(resolved)
            paths.append(resolved)
    if not paths:
        raise ValueError("no event files found")
    return paths


def build_report(inputs: Iterable[Path]) -> dict[str, Any]:
    files = event_files(inputs)
    records = [record for path in files for record in account_event_file(path)]
    subcommands = Counter(name for record in records for name in record["subcommands"])
    failed = [record for record in records
              if type(record["exit_code"]) is int and record["exit_code"] != 0]
    eof_failures = [record for record in records
                    if record["failure_class"] == "eof_read"]
    other_failures = [record for record in failed
                      if record["failure_class"] == "other"]
    return {
        "version": "repoctx.command-accounting/v1",
        "inputs": [str(path) for path in files],
        "summary": {
            "command_events": len(records),
            "recognized_subcommands": sum(subcommands.values()),
            "subcommands": dict(sorted(subcommands.items())),
            "nonzero_command_events": len(failed),
            "eof_read_failures": len(eof_failures),
            "other_nonzero_command_events": len(other_failures),
        },
        "events": records,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("events", nargs="+", type=Path,
                        help="event JSONL files or directories containing them")
    args = parser.parse_args()
    try:
        report = build_report(args.events)
    except (OSError, ValueError) as exc:
        parser.error(str(exc))
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
