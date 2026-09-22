#!/usr/bin/env python3
"""Classify observed repoctx use from completed trial command events."""
import json
from pathlib import Path
import re

RUN = Path('/tmp/m4-14-practical-20260922')
OUT = Path('/tmp/m414-grading/adoption-audit.json')
SUBCOMMAND = re.compile(r"(?<![\w/])(?:[^\s'\";&|]*/)?repoctx\s+(version|discover|files|search|read|overview|compile|validate|context)\b", re.I)
RETRIEVAL = {'discover', 'files', 'search', 'read', 'overview', 'context'}
SUSPECT = re.compile(r'\b(?:curl|wget|ssh|scp|nc|ncat|git\s+clone)\b|https?://|\.\./\.\.|/home/travis/|evals/m4/|scoring\.json', re.I)


def main():
    rows = []
    suspects = []
    for path in sorted((RUN / 'trials').glob('*/record.json')):
        record = json.loads(path.read_text())
        subcommands = []
        skill_read = False
        retrieval_output = False
        for event in record['command_events']:
            item = event.get('item', {})
            if event.get('type') != 'item.completed' or not isinstance(item, dict):
                continue
            command = item.get('command', '')
            output = item.get('aggregated_output', '')
            if not isinstance(command, str) or not isinstance(output, str):
                continue
            matches = [name.lower() for name in SUBCOMMAND.findall(command)]
            for name in matches:
                subcommands.append({'subcommand': name, 'command_exit_code': item.get('exit_code'),
                                    'output_bytes': len(output.encode('utf-8'))})
                if name in RETRIEVAL and item.get('exit_code') == 0 and output.strip():
                    retrieval_output = True
            if 'skill/repoctx/SKILL.md' in command and ('# Repoctx' in output or 'name: repoctx' in output):
                skill_read = True
            if SUSPECT.search(command):
                suspects.append({'trial_directory': path.parent.name, 'command': command,
                                 'exit_code': item.get('exit_code')})
        names = [x['subcommand'] for x in subcommands]
        rows.append({'trial_directory': path.parent.name, 'task_id': record['task_id'],
                     'condition': record['condition'], 'skill_read_observed': skill_read,
                     'repoctx_version_invoked': 'version' in names,
                     'repoctx_retrieval_invoked': any(name in RETRIEVAL for name in names),
                     'repoctx_retrieval_output_observed': retrieval_output,
                     'repoctx_index_commands_invoked': sum(name in {'compile', 'validate'} for name in names),
                     'version_only': 'version' in names and not any(name in RETRIEVAL for name in names),
                     'subcommands': subcommands})
    if len(rows) != 66 or len({row['trial_directory'] for row in rows}) != 66:
        raise ValueError('adoption audit requires all 66 distinct trials')
    by_task = {}
    for row in rows:
        by_task.setdefault(row['task_id'], set()).add(row['condition'])
    if len(by_task) != 33 or any(conditions != {'ordinary_tools', 'ordinary_tools_plus_repoctx'}
                                  for conditions in by_task.values()):
        raise ValueError('adoption audit requires 33 complete condition pairs')
    OUT.write_text(json.dumps({'version': 'm414-practical-adoption-audit/v1', 'rows': rows,
                               'scope_suspects': suspects}, ensure_ascii=False, indent=2) + '\n')
    OUT.chmod(0o600)
    print('audited', len(rows), 'trials; retrieval output observed',
          sum(x['repoctx_retrieval_output_observed'] for x in rows),
          'scope suspects', len(suspects))


if __name__ == '__main__':
    main()
