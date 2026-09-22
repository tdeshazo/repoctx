# Script utilities

## Account Repoctx calls from saved shell events

`account_repoctx_calls.py` reads completed `command_execution` events from one
or more `events.jsonl` files, or recursively from directories. It counts each
shell command event once and reports every recognized Repoctx subcommand
invocation separately. It recognizes both `repoctx read ...` PATH lookups and
explicit executable paths such as `/tools/repoctx read ...`. The JSON output
includes the source, event ID, exit code, and command for each recognized event.

```sh
python3 scripts/account_repoctx_calls.py /path/to/eval/runs > /tmp/repoctx-calls.json
```

For the saved 2026-09-22 skill cohort, this replay reports 37 command events,
39 subcommand invocations (12 `discover`, 22 `read`, 5 `search`), 11 EOF range
errors, and one other nonzero event. The other failure is the codec stage's
downstream `stage_check.py` command after a successful Repoctx read. Only saved
event records are read; the script does not execute recorded commands or alter
logs. Its shell parser handles quoted shell command strings and common command
separators; it does not evaluate shell expansions or command substitutions.
