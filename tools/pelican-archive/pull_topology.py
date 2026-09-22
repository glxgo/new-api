#!/usr/bin/env python3
"""One-shot read-only MySQL topology export for isolated local acceptance.

No credentials, endpoints, users or billing rows are selected. Never installs
anything remotely. The resulting private snapshot is not a production import.
"""
import argparse
import datetime
import json
import os
import pathlib
import shlex
import subprocess
import sys
import tempfile

OPTIONS = ('GroupRatio', 'UserUsableGroups', 'GroupOrder', 'GroupIconTypes',
           'group_ratio_setting.group_special_usable_group')
SQL = """
SET SESSION TRANSACTION READ ONLY;
START TRANSACTION WITH CONSISTENT SNAPSHOT;
SELECT JSON_OBJECT('kind','channel','data',JSON_OBJECT('id',id,'name',name,'status',status,'group',`group`,'models',models)) FROM channels ORDER BY id;
SELECT JSON_OBJECT('kind','ability','data',JSON_OBJECT('channel_id',channel_id,'group',`group`,'model',model,'enabled',IF(enabled,TRUE,FALSE))) FROM abilities ORDER BY channel_id,`group`,model;
SELECT JSON_OBJECT('kind','option','data',JSON_OBJECT('key',`key`,'value',value)) FROM options WHERE `key` IN ('GroupRatio','UserUsableGroups','GroupOrder','GroupIconTypes','group_ratio_setting.group_special_usable_group');
COMMIT;
"""


def parse_rows(lines):
    snapshot = {'schema_version': 1, 'captured_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                'channels': [], 'abilities': [], 'options': {}}
    fields = {'channel': {'id', 'name', 'status', 'group', 'models'},
              'ability': {'channel_id', 'group', 'model', 'enabled'},
              'option': {'key', 'value'}}
    for line in lines:
        row = json.loads(line)
        kind, data = row.get('kind'), row.get('data')
        if kind not in fields or not isinstance(data, dict) or set(data) != fields[kind]:
            raise ValueError('unexpected topology fields')
        if kind == 'option':
            if data['key'] not in OPTIONS or data['key'] in snapshot['options']:
                raise ValueError('unexpected or duplicate option')
            snapshot['options'][data['key']] = data['value']
        else:
            if kind == 'ability':
                if data['enabled'] not in (True, False, 0, 1):
                    raise ValueError('invalid ability status')
                data['enabled'] = bool(data['enabled'])
            snapshot['channels' if kind == 'channel' else 'abilities'].append(data)
    if not snapshot['channels'] or not {'GroupRatio', 'UserUsableGroups'} <= snapshot['options'].keys():
        raise ValueError('incomplete topology')
    if len(snapshot['channels']) > 10000 or len(snapshot['abilities']) > 100000:
        raise ValueError('topology size limit')
    return snapshot


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--host', required=True)
    p.add_argument('--port', type=int, default=22)
    p.add_argument('--identity-file')
    p.add_argument('--known-hosts')
    p.add_argument('--container', default='new-api-mysql')
    p.add_argument('--database', default='new-api')
    p.add_argument('--output', required=True)
    args = p.parse_args()
    if any(v.startswith('-') for v in (args.host, args.container, args.database)) or not 1 <= args.port <= 65535:
        p.error('invalid connection target')
    command = ['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', '-o', 'ConnectTimeout=10', '-p', str(args.port)]
    if args.identity_file:
        command += ['-i', args.identity_file]
    if args.known_hosts:
        command += ['-o', 'UserKnownHostsFile=' + args.known_hosts]
    mysql = 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql --default-character-set=utf8mb4 -uroot --batch --raw --skip-column-names ' + shlex.quote(args.database)
    command += [args.host, shlex.join(['docker', 'exec', '-i', args.container, 'sh', '-c', mysql])]
    dest = pathlib.Path(args.output).resolve()
    dest.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    temp = None
    try:
        with tempfile.TemporaryFile() as out:
            import resource
            def limit():
                resource.setrlimit(resource.RLIMIT_FSIZE, (32 << 20, 32 << 20))
            subprocess.run(command, input=SQL.encode(), stdout=out, stderr=subprocess.PIPE,
                           check=True, timeout=30, preexec_fn=limit)
            out.seek(0)
            snapshot = parse_rows(out)
        fd, temp = tempfile.mkstemp(prefix='.topology-', dir=dest.parent)
        with os.fdopen(fd, 'w', encoding='utf-8') as handle:
            json.dump(snapshot, handle, ensure_ascii=False)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temp, dest)
        print(f'topology saved: {len(snapshot["channels"])} channels, {len(snapshot["abilities"])} abilities')
        return 0
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(f'topology export failed ({type(error).__name__}); previous file retained', file=sys.stderr)
        return 1
    finally:
        if temp and os.path.exists(temp):
            os.unlink(temp)


if __name__ == '__main__':
    sys.exit(main())
