#!/usr/bin/env python3
"""Create incremental, condition-blind M4 practical review batches."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import random
import re
import secrets

BASE = Path('/tmp/m414-grading')
GOLD = BASE / 'gold-cards.json'
MAPPING = BASE / 'operator-mapping.json'
PACKETS = BASE / 'reviewer-packets'
CONDITION_WORDS = re.compile(r'ordinary_tools_plus_repoctx|ordinary_tools|repoctx|bounded_lexical|human_oracle|artifact_aware|context bundle', re.I)
TRIAL_PATH = re.compile(r'(?:/tmp/[^\s`"\']+|evals/runs/[^\s`"\']+)/trials/[^/\s`"\']+')
RESTRICTED_TOKEN = re.compile(r'violet-synthetic-marker', re.I)


def load_records(run_root):
    records = []
    for path in sorted((run_root / 'trials').glob('*/record.json')):
        try:
            record = json.loads(path.read_text())
            task_id, condition = record['task_id'], record['condition']
            final = path.parent / 'raw/final.txt'
            repository = path.parent / 'workspace/repository'
            if not final.is_file() or not repository.is_dir():
                continue
        except (ValueError, KeyError, TypeError, OSError):
            continue  # The runner may still be writing this trial.
        if record['answer'] != final.read_text(encoding='utf-8'):
            raise ValueError(f'record and final response disagree for {path.parent.name}')
        records.append((path, task_id, condition, final, repository))
    return records


def masked_answer(value):
    value = TRIAL_PATH.sub('[trial workspace]', value)
    value = CONDITION_WORDS.sub('[tool/source]', value)
    return RESTRICTED_TOKEN.sub('[restricted value disclosed]', value)


def visible_sources(card):
    result = []
    for source in card['permitted_sources']:
        item = dict(source)
        if 'ADMIN_EXPORT_TOKEN' in item['text']:
            item['text'] = ''.join('[unrelated token declaration redacted]\n'
                                 if 'ADMIN_EXPORT_TOKEN' in line else line
                                 for line in item['text'].splitlines(keepends=True))
            item.pop('sha256')  # Redacted text is not an exact file-hash match.
            item['redacted_unrelated_line'] = True
        result.append(item)
    return result


def packet_for(card, final_text, repository, packet_id):
    packet = {
        'packet_id': packet_id,
        'task_type': card['type'],
        'question': card['question'],
        'expected_outcome': card['expected_outcome'],
        'expected_answer': card['expected_answer'],
        'required_permitted_sources': card['required_permitted_sources'],
        'visible_sources': visible_sources(card),
        'model_response': masked_answer(final_text),
        'review': {'grade': None, 'reason': '', 'source_grounding': ''},
    }
    spec = importlib.util.spec_from_file_location('m414_change_helper', BASE / 'check_changes.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    changed_files = module.source_diff(card, repository)
    for changed in changed_files:
        for field in ('diff', 'before', 'after'):
            changed[field] = RESTRICTED_TOKEN.sub('[restricted value redacted]', changed[field])
    packet['source_changes'] = {'changed_files': changed_files}
    if card['expected_outcome'] == 'access_denied':
        assert 'violet-synthetic-marker' not in json.dumps(packet)
        assert all(source['file'] != 'portal/config.py' for source in packet['visible_sources'])
    return packet


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('run_root', type=Path)
    parser.add_argument('--batch-size', type=int, default=12)
    parser.add_argument('--force', action='store_true')
    args = parser.parse_args()
    if args.batch_size < 1:
        parser.error('batch size must be positive')
    BASE.mkdir(mode=0o700, parents=True, exist_ok=True)
    PACKETS.mkdir(mode=0o700, exist_ok=True)
    mapping = json.loads(MAPPING.read_text()) if MAPPING.exists() else {'version': 'm414-practical-mapping/v1', 'items': []}
    cards = {card['task_id']: card for card in json.loads(GOLD.read_text())['cards']}
    seen = {item['trial_directory'] for item in mapping['items']}
    available = [row for row in load_records(args.run_root) if row[0].parent.name not in seen]
    if len(available) < args.batch_size and not args.force:
        print(f'{len(available)} complete unpacketized trials; awaiting {args.batch_size}')
        return
    rng = random.SystemRandom()
    rng.shuffle(available)
    selected = available[:args.batch_size]
    if not selected:
        print('no new complete trials')
        return
    batch = len(list(PACKETS.glob('batch-*.json'))) + 1
    ids = []
    staged = []
    for record_path, task_id, condition, final_path, repository in selected:
        if task_id not in cards:
            raise ValueError('unknown task in trial record')
        if repository.is_symlink() or any(path.is_symlink() for path in repository.rglob('*')):
            raise ValueError('trial repository contains a symlink')
        packet_id = secrets.token_hex(8)
        answer_bytes = final_path.read_bytes()
        packet = packet_for(cards[task_id], answer_bytes.decode('utf-8', errors='replace'), repository, packet_id)
        staged.append((packet_id, packet, {'packet_id': packet_id,
                                          'trial_directory': record_path.parent.name,
                                          'task_id': task_id, 'condition': condition,
                                          'answer_sha256': hashlib.sha256(answer_bytes).hexdigest()}))
        ids.append(packet_id)
    for packet_id, packet, item in staged:
        path = PACKETS / f'{packet_id}.json'
        path.write_text(json.dumps(packet, ensure_ascii=False, indent=2) + '\n')
        mapping['items'].append(item)
    rng.shuffle(ids)
    (PACKETS / f'batch-{batch:03d}.json').write_text(json.dumps({'packet_ids': ids}, indent=2) + '\n')
    MAPPING.write_text(json.dumps(mapping, ensure_ascii=False, indent=2) + '\n')
    MAPPING.chmod(0o600)
    print(f'batch {batch:03d}: {len(ids)} condition-blind packets in {PACKETS}')


if __name__ == '__main__':
    main()
