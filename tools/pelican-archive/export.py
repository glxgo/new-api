#!/usr/bin/env python3
"""Read-only, consistent whitelist export. Run via restricted SSH, never as a web endpoint.

Example (stdout is private archive data):
  python3 export.py --database /path/benchmarks.db --source-id bench-primary \
    --grader /path/pelicanGrader.js
The grader file is read as text; no external application code is executed.
"""
import argparse
import datetime
import json
import pathlib
import re
import sqlite3
import sys

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--database', required=True)
    parser.add_argument('--source-id', required=True)
    parser.add_argument('--grader', required=True)
    args = parser.parse_args()
    # URI mode=ro prevents writes, including accidental schema changes.
    db = sqlite3.connect(pathlib.Path(args.database).resolve().as_uri() + '?mode=ro', uri=True, timeout=5)
    db.row_factory = sqlite3.Row
    db.execute('PRAGMA query_only=ON')
    db.execute('BEGIN')
    config_row = db.execute("SELECT value FROM pelican_config WHERE key='global'").fetchone()
    config = json.loads(config_row['value']) if config_row else {}
    grader = pathlib.Path(args.grader).read_text(encoding='utf-8')
    # Version-specific fail-closed extraction; do not eval JavaScript.
    prompt_match = re.search(r"exports\.DEFAULT_PELICAN_PROMPT = '([^'\\]*)';", grader)
    expected_match = re.search(r'exports\.DEFAULT_PELICAN_EXPECTED_ANSWER = (\d+);', grader)
    if not prompt_match or not expected_match:
        raise ValueError('unsupported grader source format')
    builtin = not config.get('prompt', '').strip()
    prompt = prompt_match[1] if builtin else config['prompt']
    h = 0x811c9dc5
    encoded = prompt.encode('utf-16-le')
    for i in range(0, len(encoded), 2):
        h = ((h ^ int.from_bytes(encoded[i:i+2], 'little')) * 0x01000193) & 0xffffffff
    targets = [dict(r) for r in db.execute('SELECT provider_id, model_name, provider_name, interval_minutes, enabled FROM pelican_targets ORDER BY provider_id, model_name LIMIT 2001')]
    # Never export provider credentials, accounts, sessions, or environment.
    columns = 'id, provider_id, model_name, provider_name, grade, expected_answer, reported_answer, answer_in_svg, answer_in_text, svg, svg_bytes, svg_truncated, raw_text, latency_ms, ttft_ms, input_tokens, output_tokens, attempts, error, prompt_hash, created_at'
    runs = [dict(r) for r in db.execute('SELECT ' + columns + ' FROM pelican_runs ORDER BY id LIMIT 10001')]
    for run in runs:
        for key in ('svg', 'raw_text', 'error'):
            run[key] = run[key] or ''
    db.rollback()
    db.close()
    if len(targets) > 2000 or len(runs) > 10000:
        raise ValueError('archive export row limit exceeded')
    payload = json.dumps({
        'schema_version': 1, 'source_id': args.source_id,
        'captured_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'config': {'auto_run': config.get('autoRunEnabled', False),
                   'interval_minutes': config.get('defaultIntervalMinutes', 360),
                   'expected_answer': config.get('expectedAnswer', int(expected_match[1])),
                   'prompt': prompt, 'prompt_hash': f'{h:08x}',
                   'using_builtin': builtin, 'builtin_expected': int(expected_match[1])},
        'targets': targets, 'runs': runs,
    }, ensure_ascii=False, separators=(',', ':')).encode('utf-8')
    if len(payload) > 32 * 1024 * 1024:
        raise ValueError('archive export size limit exceeded')
    sys.stdout.buffer.write(payload)

if __name__ == '__main__':
    main()
