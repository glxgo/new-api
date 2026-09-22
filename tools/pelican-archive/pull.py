#!/usr/bin/env python3
"""Bounded SSH archive pull; atomically replaces a private local snapshot.

Production: SSH alias must use a dedicated forced-command/read-only account.
Development --audit-export streams our exporter to python3 without remote writes.
"""
import argparse
import json
import os
import pathlib
import subprocess
import tempfile
import time
import sys
import fcntl

def parse_args():
    p = argparse.ArgumentParser()
    p.add_argument('--host', required=True)
    p.add_argument('--ssh-config', help='dedicated local SSH configuration for production transport')
    p.add_argument('--source-id', required=True)
    p.add_argument('--output', required=True)
    p.add_argument('--audit-export', action='store_true')
    p.add_argument('--database')
    p.add_argument('--grader')
    p.add_argument('--interval-seconds', type=int, default=0,
                   help='0: pull once; otherwise repeat every 60–86400 seconds')
    p.add_argument('--cycles', type=int, default=0,
                   help='limit repeated pulls; 0 runs until stopped')
    args = p.parse_args()
    if args.interval_seconds != 0 and not 60 <= args.interval_seconds <= 86400:
        p.error('interval-seconds must be 0 or 60–86400')
    if args.cycles < 0:
        p.error('cycles must be non-negative')
    return args


def pull_once(args):
    if args.host.startswith('-'):
        raise ValueError('invalid SSH alias')
    command = ['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', '-o', 'ConnectTimeout=10',
               '-o', 'ServerAliveInterval=5', '-o', 'ServerAliveCountMax=3', '-T']
    if getattr(args, 'ssh_config', None):
        command += ['-F', args.ssh_config]
    command.append(args.host)
    script = None
    if args.audit_export:
        import shlex
        if not args.database or not args.grader:
            raise ValueError('audit export requires exact database and grader paths')
        command.append(' '.join(shlex.quote(s) for s in ['python3', '-', '--database', args.database, '--grader', args.grader, '--source-id', args.source_id]))
        script = pathlib.Path(__file__).with_name('export.py').read_bytes()
    # Redirect to a bounded file rather than retaining arbitrarily large output
    # in process memory. Check child timeout and file size before publication.
    dest = pathlib.Path(args.output).resolve()
    dest.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, temp = tempfile.mkstemp(prefix='.pelican-', dir=dest.parent)
    try:
        with os.fdopen(fd, 'w+b') as out:
            import resource
            def limit():
                resource.setrlimit(resource.RLIMIT_FSIZE, (32*1024*1024, 32*1024*1024))
            subprocess.run(command, input=script, stdout=out, stderr=subprocess.PIPE, timeout=30, check=True, preexec_fn=limit)
            out.seek(0)
            snapshot = json.load(out)
            if snapshot.get('schema_version') != 1 or snapshot.get('source_id') != args.source_id:
                raise ValueError('archive identity/schema mismatch')
            if not isinstance(snapshot.get('targets'), list) or not isinstance(snapshot.get('runs'), list):
                raise ValueError('archive collections missing')
            out.flush()
            os.fsync(out.fileno())
        os.chmod(temp, 0o600)
        os.replace(temp, dest)
        print(f'archive saved: {len(snapshot["targets"])} targets, {len(snapshot["runs"])} records', flush=True)
    finally:
        if os.path.exists(temp):
            os.unlink(temp)


def run(args):
    dest = pathlib.Path(args.output).resolve()
    dest.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    # Separate lock survives atomic replacement and prevents overlapping pulls
    # from overwriting a newer snapshot with an older concurrent capture.
    fd = os.open(str(dest) + '.lock', os.O_CREAT | os.O_RDWR, 0o600)
    with os.fdopen(fd, 'w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        cycle = 0
        while True:
            started = time.monotonic()
            cycle += 1
            success = True
            try:
                pull_once(args)
            except (OSError, ValueError, subprocess.SubprocessError) as error:
                # Never print remote stderr/command arguments or archive contents.
                print(f'archive pull failed ({type(error).__name__}); previous snapshot retained', file=sys.stderr, flush=True)
                success = False
            if not args.interval_seconds or (args.cycles and cycle >= args.cycles):
                return 0 if success else 1
            remaining = max(0, args.interval_seconds - (time.monotonic() - started))
            time.sleep(remaining)


def main():
    try:
        return run(parse_args())
    except BlockingIOError:
        print('another archive pull is already running for this output', file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 0

if __name__ == '__main__':
    sys.exit(main())
