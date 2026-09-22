#!/usr/bin/env python3
"""Bind completed blinded reviews to the practical paired run, fail closed."""
import hashlib
import json
from pathlib import Path
import statistics

BASE = Path('/tmp/m414-grading')
RUN = Path('/tmp/m4-14-practical-20260922')
CONDITIONS = {'ordinary_tools', 'ordinary_tools_plus_repoctx'}


def require(ok, message):
    if not ok:
        raise ValueError(message)


def read(path):
    return json.loads(path.read_text())


def complete_sum(rows, field):
    values = [x['usage'].get(field) if isinstance(x['usage'], dict) else None for x in rows]
    return sum(values) if all(type(value) is int for value in values) else None


def complete_median(values):
    return statistics.median(values) if all(type(value) in {int, float} for value in values) else None


def main():
    mapping = read(BASE / 'operator-mapping.json')['items']
    require(len(mapping) == 66, f'expected 66 mapped trials, got {len(mapping)}')
    require(len({x['packet_id'] for x in mapping}) == 66, 'duplicate packet ID')
    require(len({x['trial_directory'] for x in mapping}) == 66, 'duplicate trial directory')
    grades = {}
    for path in sorted((BASE / 'grades').glob('batch-*.json')):
        for item in read(path):
            ident = item['packet_id']
            require(ident not in grades, f'duplicate grade for {ident}')
            require(item['grade'] in {'pass', 'partial', 'fail'} and
                    isinstance(item['reason'], str) and item['reason'].strip() and
                    isinstance(item['source_grounding'], str) and item['source_grounding'].strip(),
                    f'invalid grade for {ident}')
            grades[ident] = item
    require(set(grades) == {x['packet_id'] for x in mapping}, 'grade coverage is incomplete')
    audits = {}
    for path in sorted((BASE / 'grades').glob('audit-*.json')):
        for item in read(path):
            ident = item['packet_id']
            require(ident in grades and ident not in audits and
                    item['grade'] in {'pass', 'partial', 'fail'} and
                    item['reason'].strip() and item['source_grounding'].strip(),
                    f'invalid audit for {ident}')
            audits[ident] = item
    for path in sorted((BASE / 'reviewer-packets').glob('batch-*.json')):
        ids = read(path)['packet_ids']
        nonpasses = {ident for ident in ids if grades[ident]['grade'] != 'pass'}
        require(nonpasses <= set(audits), f'unaudited nonpass in {path.name}')
        passes = {ident for ident in ids if grades[ident]['grade'] == 'pass'}
        require(not passes or bool(passes & set(audits)),
                f'no sampled pass audit for {path.name}')
    adjudications = {x['packet_id']: x for x in read(BASE / 'grades/adjudications.json')}
    require(len(adjudications) == len(read(BASE / 'grades/adjudications.json')),
            'duplicate adjudication')
    for ident, item in adjudications.items():
        require(ident in grades and ident in audits and
                item['initial_grade'] == grades[ident]['grade'] and
                item['audit_grade'] == audits[ident]['grade'] and
                item['final_grade'] in {'pass', 'partial', 'fail'} and item['reason'].strip(),
                f'invalid adjudication for {ident}')
    disagreements = {ident for ident, audit in audits.items()
                     if audit['grade'] != grades[ident]['grade']}
    require(disagreements <= set(adjudications),
            'audit/primary disagreement lacks adjudication')
    checks = read(BASE / 'change-checks.json')['checks']
    require(len(checks) == 12 and len({x['trial_directory'] for x in checks}) == 12,
            f'expected 12 independent change checks, got {len(checks)}')
    check_by_trial = {x['trial_directory']: x for x in checks}
    cards = {x['task_id']: x for x in read(BASE / 'gold-cards.json')['cards']}
    scope_path = BASE / 'scope-findings.json'
    scope_findings = read(scope_path)['violations'] if scope_path.exists() else []
    require(len(scope_findings) == len({x['trial_directory'] for x in scope_findings}) and
            all(isinstance(x['reason'], str) and x['reason'].strip() for x in scope_findings),
            'invalid observed scope findings')
    scope_by_trial = {x['trial_directory']: x for x in scope_findings}
    require(set(scope_by_trial) <= {x['trial_directory'] for x in mapping},
            'scope finding names unknown trial')
    results = []
    for item in mapping:
        ident = item['packet_id']
        trial = RUN / 'trials' / item['trial_directory']
        record = read(trial / 'record.json')
        response = (trial / 'raw/final.txt').read_bytes()
        packet = read(BASE / 'reviewer-packets' / (ident + '.json'))
        require(record['task_id'] == item['task_id'] and record['condition'] == item['condition'] and
                record['answer'] == response.decode('utf-8') and
                hashlib.sha256(response).hexdigest() == item['answer_sha256'],
                f'trial binding mismatch for {ident}')
        require(item['condition'] in CONDITIONS and item['task_id'] in cards and
                packet['question'] == cards[item['task_id']]['question'] and
                packet['expected_outcome'] == cards[item['task_id']]['expected_outcome'],
                f'packet/gold mismatch for {ident}')
        check = check_by_trial.get(item['trial_directory'])
        if cards[item['task_id']]['type'] == 'change':
            require(check is not None and check['task_id'] == item['task_id'],
                    f'missing change check for {ident}')
        else:
            require(check is None, f'unexpected change check for {ident}')
        grade = adjudications.get(ident, {}).get('final_grade', grades[ident]['grade'])
        process_ok = record['exit_code'] == 0 and record['timed_out'] is False
        change_ok = check is None or check['check']['passed'] is True
        changed = packet['source_changes']['changed_files']
        scope_violation = item['trial_directory'] in scope_by_trial
        success = int(grade == 'pass' and process_ok and change_ok and not scope_violation)
        results.append({
            'packet_id': ident, 'trial_directory': item['trial_directory'],
            'task_id': item['task_id'], 'condition': item['condition'],
            'task_type': cards[item['task_id']]['type'], 'expected_outcome': cards[item['task_id']]['expected_outcome'],
            'initial_grade': grades[ident]['grade'], 'audit_grade': audits.get(ident, {}).get('grade'),
            'final_grade': grade, 'grade_reason': adjudications.get(ident, {}).get('reason', grades[ident]['reason']),
            'source_grounding': (audits[ident]['source_grounding'] if ident in adjudications
                                 else grades[ident]['source_grounding']),
            'verified_success': success,
            'process_ok': process_ok, 'exit_code': record['exit_code'], 'timed_out': record['timed_out'],
            'tool_call_cap_respected': record['tool_call_cap_respected'],
            'scope_violation_observed': scope_violation,
            'scope_finding': scope_by_trial.get(item['trial_directory']),
            'independent_change_check': check['check'] if check else None,
            'source_mutation_on_nonchange': bool(changed) and cards[item['task_id']]['type'] != 'change',
            'changed_files': [x['file'] for x in changed],
            'wall_seconds': record.get('wall_seconds'), 'usage': record.get('usage'),
            'command_call_count': record.get('command_call_count'),
            'observed_tool_item_count': record.get('observed_tool_item_count'),
            'run_errors': record['errors'],
        })
    by_task = {}
    for result in results:
        by_task.setdefault(result['task_id'], {})[result['condition']] = result
    require(len(by_task) == 33 and all(set(pair) == CONDITIONS for pair in by_task.values()),
            'paired coverage is incomplete')
    by_condition = {}
    for condition in sorted(CONDITIONS):
        rows = [x for x in results if x['condition'] == condition]
        by_condition[condition] = {
            'trials': len(rows), 'verified_successes': sum(x['verified_success'] for x in rows),
            'success_rate': sum(x['verified_success'] for x in rows) / len(rows),
            'median_wall_seconds': complete_median([x['wall_seconds'] for x in rows]),
            'total_input_tokens': complete_sum(rows, 'input_tokens'),
            'total_output_tokens': complete_sum(rows, 'output_tokens'),
            'usage_missing_trials': sum(not isinstance(x['usage'], dict) or
                                        type(x['usage'].get('input_tokens')) is not int or
                                        type(x['usage'].get('output_tokens')) is not int for x in rows),
            'tool_cap_overages': sum(x['tool_call_cap_respected'] is False for x in rows),
            'total_command_calls': (sum(x['command_call_count'] for x in rows)
                                    if all(type(x['command_call_count']) is int for x in rows) else None),
            'total_observed_tool_items': (sum(x['observed_tool_item_count'] for x in rows)
                                          if all(type(x['observed_tool_item_count']) is int for x in rows)
                                          else None),
            'scope_violations_observed': sum(x['scope_violation_observed'] for x in rows),
            'source_mutations_on_nonchanges': sum(x['source_mutation_on_nonchange'] for x in rows),
        }
    paired = {
        'treatment_only_success': sum(pair['ordinary_tools_plus_repoctx']['verified_success'] == 1 and
                                      pair['ordinary_tools']['verified_success'] == 0 for pair in by_task.values()),
        'baseline_only_success': sum(pair['ordinary_tools_plus_repoctx']['verified_success'] == 0 and
                                     pair['ordinary_tools']['verified_success'] == 1 for pair in by_task.values()),
        'both_success': sum(pair['ordinary_tools_plus_repoctx']['verified_success'] == 1 and
                            pair['ordinary_tools']['verified_success'] == 1 for pair in by_task.values()),
        'neither_success': sum(pair['ordinary_tools_plus_repoctx']['verified_success'] == 0 and
                               pair['ordinary_tools']['verified_success'] == 0 for pair in by_task.values()),
    }
    (BASE / 'operator-results.json').write_text(json.dumps({'version': 'm414-practical-joined/v1', 'results': results}, indent=2) + '\n')
    (BASE / 'summary.json').write_text(json.dumps({'version': 'm414-practical-summary/v1',
                                                   'by_condition': by_condition, 'paired': paired,
                                                   'audited_packets': len(audits),
                                                   'adjudications': len(adjudications),
                                                   'scope_findings_recorded': scope_path.exists()}, indent=2) + '\n')
    print('joined 66 reviewed trials; wrote operator-results.json and summary.json')


if __name__ == '__main__':
    main()
