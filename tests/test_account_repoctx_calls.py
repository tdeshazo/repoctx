"""Replay saved shell events without conflating shell events and Repoctx calls."""

import importlib.util
import json
from pathlib import Path
import shlex
import tempfile
import unittest


PROJECT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "account_repoctx_calls", PROJECT / "scripts/account_repoctx_calls.py"
)
ACCOUNTING = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ACCOUNTING)


def command_event(event_id, command, exit_code=0, output=""):
    return {"type": "item.completed", "item": {
        "id": event_id, "type": "command_execution", "command": command,
        "exit_code": exit_code, "aggregated_output": output,
    }}


class RepoctxCallAccountingTests(unittest.TestCase):
    def test_path_and_explicit_executable_calls_in_one_shell_event(self):
        script = "repoctx search -query x && '/tools with spaces/repoctx' read -root ."
        command = "/usr/bin/zsh -lc " + shlex.quote(script)
        self.assertEqual(
            ACCOUNTING.recognize_subcommands(command), ["search", "read"]
        )

    def test_mentions_in_arguments_are_not_counted_as_invocations(self):
        command = "/bin/sh -lc " + shlex.quote(
            "printf '%s\\n' 'repoctx read -file x:1:2'; git status --short"
        )
        self.assertEqual(ACCOUNTING.recognize_subcommands(command), [])

    def test_replay_counts_command_events_and_subcommands_separately(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            events = root / "events.jsonl"
            rows = [
                {"type": "item.started", "item": {"id": "started"}},
                command_event("item_1", "/bin/zsh -lc " + shlex.quote(
                    "repoctx search -query x && repoctx discover -query y"
                )),
                command_event("item_2", "repoctx read -file x:4:99", 2,
                              "line range exceeds observed file (99); retry at EOF"),
                command_event("item_3", "repoctx discover -query z && stage_check", 1,
                              "public check failed"),
            ]
            events.write_text("".join(json.dumps(row) + "\n" for row in rows),
                              encoding="utf-8")

            report = ACCOUNTING.build_report([root])

        self.assertEqual(report["summary"], {
            "command_events": 3,
            "recognized_subcommands": 4,
            "subcommands": {"discover": 2, "read": 1, "search": 1},
            "nonzero_command_events": 2,
            "eof_read_failures": 1,
            "other_nonzero_command_events": 1,
        })
        self.assertEqual(report["events"][1]["failure_class"], "eof_read")
        self.assertEqual(report["events"][2]["failure_class"], "other")

    def test_new_requested_endpoint_diagnostics_count_as_eof_errors(self):
        with tempfile.TemporaryDirectory() as directory:
            events = Path(directory) / "events.jsonl"
            rows = [
                command_event("item_1", "repoctx read -file x:5:99", 2,
                              "requested end line 99 exceeds observed file (20 lines)"),
                command_event("item_2", "repoctx read -file x:99:0", 2,
                              "requested start line 99 exceeds observed file (20 lines)"),
            ]
            events.write_text("".join(json.dumps(row) + "\n" for row in rows),
                              encoding="utf-8")

            report = ACCOUNTING.build_report([events])

        self.assertEqual(report["summary"]["eof_read_failures"], 2)
        self.assertEqual(
            [event["failure_class"] for event in report["events"]],
            ["eof_read", "eof_read"],
        )


if __name__ == "__main__":
    unittest.main()
