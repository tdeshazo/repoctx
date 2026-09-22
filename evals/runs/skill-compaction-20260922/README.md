# Repoctx skill compaction trial, 2026-09-22

This is an exploratory, single-run comparison of the original 11,499-byte
`SKILL.md` at commit `22dcd9b`, the initial compact skill, and one targeted
revision. It applies the [SkillReducer](https://arxiv.org/html/2603.29919)
routing and task-retention checks to the
repoctx skill; it is not a general token-savings claim.

`scripts/eval_skill_compaction.py` used fresh copies of the fixed
`evals/pilot/repositories/relay-board` fixture. Each workspace exposed one
skill variant at `.agents/skills/repoctx`, the same pinned executable, and the
same task prompt. Codex used `gpt-5.6-luna` at medium reasoning effort. The
runner verified the executable's version and exercised discovery, compilation,
validation, and indexed context once before the trials. Only the indexed task
received a trusted capability handoff. Tool network access was disabled and
trial source files were unchanged.

| Task | Skill | Required answer terms | Tool calls | Failed commands | Input + output tokens |
| --- | --- | --- | ---: | ---: | ---: |
| Supplied evidence | Original | Pass | 0 | 0 | 15,442 |
| Supplied evidence | Initial compact | Pass | 0 | 0 | 15,648 |
| Locate queue function | Original | Pass | 4 | 0 | 78,421 |
| Locate queue function | Initial compact | Pass | 2 | 0 | 51,627 |
| Trace dispatch | Original | Pass | 5 | 0 | 99,806 |
| Trace dispatch | Initial compact | Pass | 4 | 0 | 89,635 |
| Indexed trace | Original | Pass | 8 | 1 | 193,056 |
| Indexed trace | Initial compact | Pass | 21 | 8 | 276,707 |
| Indexed trace | Revised compact | Pass | 8 | 1 | 197,551 |

Both variants skipped the skill and repoctx on supplied evidence. Both loaded
their skill and used repoctx on the location and trace tasks. The initial compact
indexed trial loaded its indexed reference and honored the capability handoff,
but it attempted positional `repoctx read` commands, then consulted help and
repeated several source reads, exceeding the advisory 16-call cap. The revision
restored an exact `read -file`
example, batched-range guidance, and a bounded initial graph depth. The revised
indexed trial loaded the reference, made no invalid repoctx call, and skipped
the repeated version probe. Its single failed command was a read of a
nonexistent user skill path; it then read the intended trial skill. The
original indexed trial had the same kind of failed path lookup. The revised
answer cited both returned caller-to-callee callsites and described the
configuration accurately.

Each row is one stochastic agent run. Provider caching was uncontrolled, and
the lexical answer check is narrower than full task correctness. The raw
events, final answers, stderr, records, protocols, and exact skill snapshots
are archived here; workspaces, binaries, and authentication files are not.

To run another fresh comparison, provide an original skill directory, a built
repoctx executable, and a new output directory outside this repository:

```sh
python scripts/eval_skill_compaction.py \
  --baseline-skill /tmp/original-repoctx-skill \
  --repoctx /tmp/repoctx \
  --run-dir /tmp/new-skill-compaction-run
```
