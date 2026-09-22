#!/usr/bin/env python3
"""Run M4 change checks in a separate timed, read-only filesystem sandbox."""
import argparse
import difflib
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

PROJECT = Path('/home/travis/Workspace/repository_context_program/repoctx')
GOLD = Path('/tmp/m414-grading/gold-cards.json')
REPOSITORIES = Path('/tmp/m414-grading/operator-repository-map.json')


def repository_for(trial):
    candidates = [trial / 'workspace' / 'repository', trial / 'repository']
    found = [path for path in candidates if path.is_dir() and not path.is_symlink()]
    if len(found) != 1:
        raise ValueError(f'expected one exported repository in {trial}')
    return found[0]


def source_diff(card, repository):
    repository_name = json.loads(REPOSITORIES.read_text())[card['task_id']]
    original = PROJECT / 'evals/m4/repositories' / repository_name
    before_files = {item['file'] for item in card['permitted_sources']}
    after_files = {p.relative_to(repository).as_posix() for p in repository.rglob('*')
                   if p.is_file() and '__pycache__' not in p.parts and p.suffix != '.pyc'}
    output = []
    for relative in sorted(before_files | after_files):
        before_path, after_path = original / relative, repository / relative
        before = before_path.read_text().splitlines(keepends=True) if before_path.is_file() else []
        after = after_path.read_text().splitlines(keepends=True) if after_path.is_file() else []
        if before == after:
            continue
        output.append({'file': relative, 'diff': ''.join(difflib.unified_diff(
            before, after, fromfile='before/' + relative, tofile='after/' + relative, n=0)),
            'before': ''.join(before), 'after': ''.join(after)})
    return output


def run_check(card, repository):
    command = ['bwrap', '--ro-bind', '/usr', '/usr', '--ro-bind', '/lib', '/lib',
               '--ro-bind', '/lib64', '/lib64', '--ro-bind', '/bin', '/bin',
               '--ro-bind', '/etc', '/etc', '--dev', '/dev', '--proc', '/proc',
               '--tmpfs', '/tmp', '--ro-bind', str(PROJECT), '/project',
               '--ro-bind', str(repository), '/trial', '--clearenv',
               '--setenv', 'PYTHONDONTWRITEBYTECODE', '1', '--setenv', 'PATH', '/usr/bin',
               '--unshare-pid', '--die-with-parent', '--chdir', '/project', '--',
               '/usr/bin/python3', '-I', '-B', '/project/scripts/check_m4_tasks.py',
               '--verify-change', card['task_id'], '--repository', '/trial']
    start = time.monotonic()
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=15)
    except subprocess.TimeoutExpired:
        return {'passed': False, 'reason': 'verifier timeout', 'verifier_seconds': round(time.monotonic()-start, 3)}
    elapsed = round(time.monotonic()-start, 3)
    try:
        report = json.loads(result.stdout)
        check = report['check']
        if type(check['passed']) is not bool or not isinstance(check['message'], str):
            raise ValueError('malformed check result')
        passed = result.returncode == 0 and report['ok'] is True and check['passed'] is True
        return {'passed': passed, 'reason': check['message'], 'verifier_seconds': elapsed,
                'exit_code': result.returncode}
    except (ValueError, KeyError, TypeError):
        return {'passed': False, 'reason': 'verifier error: ' + result.stderr[-1000:],
                'verifier_seconds': elapsed, 'exit_code': result.returncode}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('run_root', type=Path)
    args = parser.parse_args()
    cards = {card['task_id']: card for card in json.loads(GOLD.read_text())['cards']}
    checks = []
    for record_path in sorted(args.run_root.glob('trials/*/record.json')):
        try:
            record = json.loads(record_path.read_text())
            task_id = record['task_id']
            card = cards[task_id]
        except (OSError, ValueError, KeyError, TypeError):
            continue  # A trial still being written is not ready for verification.
        if card['type'] != 'change':
            continue
        try:
            repository = repository_for(record_path.parent)
            if any(path.is_symlink() for path in repository.rglob('*')):
                raise ValueError('trial repository contains a symlink')
            diffs = source_diff(card, repository)
            with tempfile.TemporaryDirectory(prefix='m414-check-') as temp:
                verification_copy = Path(temp) / 'repository'
                shutil.copytree(repository, verification_copy,
                                ignore=shutil.ignore_patterns('__pycache__', '*.pyc'))
                check = run_check(card, verification_copy)
            checks.append({'trial_directory': record_path.parent.name, 'task_id': task_id,
                           'check': check, 'changed_files': diffs})
        except (OSError, ValueError, KeyError) as exc:
            checks.append({'trial_directory': record_path.parent.name, 'task_id': task_id,
                           'check': {'passed': False, 'reason': str(exc)}, 'changed_files': []})
    target = Path('/tmp/m414-grading/change-checks.json')
    temporary = target.with_suffix('.tmp')
    temporary.write_text(json.dumps({'checks': checks}, ensure_ascii=False, indent=2) + '\n')
    temporary.replace(target)
    print(f'{len(checks)} change checks written to {target}')


if __name__ == '__main__':
    main()
