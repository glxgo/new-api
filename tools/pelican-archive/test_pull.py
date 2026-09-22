import argparse
import contextlib
import io
import json
import pathlib
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import pull


class PullTests(unittest.TestCase):
    def test_dedicated_config_and_commandless_production_transport(self):
        with tempfile.TemporaryDirectory() as directory:
            args = argparse.Namespace(host='pelican-source', ssh_config='/private/config', source_id='fixture',
                                      output=str(pathlib.Path(directory) / 'archive.json'), audit_export=False)
            def command(argv, **kwargs):
                self.assertEqual(argv[-1], 'pelican-source')
                self.assertEqual(argv[argv.index('-F') + 1], '/private/config')
                self.assertIn('-T', argv)
                self.assertIsNone(kwargs['input'])
                kwargs['stdout'].write(b'{"schema_version":1,"source_id":"fixture","runs":[],"targets":[]}')
            with patch('pull.subprocess.run', side_effect=command), contextlib.redirect_stdout(io.StringIO()):
                pull.pull_once(args)

    def test_failure_preserves_snapshot_and_next_cycle_recovers(self):
        with tempfile.TemporaryDirectory() as directory:
            output = pathlib.Path(directory) / 'archive.json'
            output.write_bytes(b'previous snapshot')
            args = argparse.Namespace(host='fixture', source_id='fixture', output=str(output),
                                      audit_export=False, interval_seconds=60, cycles=2)
            calls = []

            def command(*unused, **kwargs):
                calls.append(True)
                if len(calls) == 1:
                    kwargs['stdout'].write(b'incomplete')
                    raise subprocess.TimeoutExpired('redacted', 30)
                self.assertEqual(output.read_bytes(), b'previous snapshot')
                kwargs['stdout'].write(json.dumps({'schema_version': 1, 'source_id': 'fixture',
                                                  'targets': [], 'runs': []}).encode())

            with patch('pull.subprocess.run', side_effect=command), patch('pull.time.sleep'), contextlib.redirect_stderr(io.StringIO()), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(pull.run(args), 0)
            self.assertEqual(len(calls), 2)
            self.assertEqual(json.loads(output.read_text())['source_id'], 'fixture')
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            self.assertEqual(list(pathlib.Path(directory).glob('.pelican-*')), [])

    def test_wrong_source_cannot_replace_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            output = pathlib.Path(directory) / 'archive.json'
            output.write_bytes(b'previous snapshot')
            args = argparse.Namespace(host='fixture', source_id='fixture', output=str(output),
                                      audit_export=False, interval_seconds=0, cycles=0)
            def command(*unused, **kwargs):
                kwargs['stdout'].write(b'{"schema_version":1,"source_id":"other","runs":[],"targets":[]}')
            with patch('pull.subprocess.run', side_effect=command), contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(pull.run(args), 1)
            self.assertEqual(output.read_bytes(), b'previous snapshot')

    def test_parallel_puller_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            args = argparse.Namespace(output=str(pathlib.Path(directory) / 'archive.json'), interval_seconds=0, cycles=0)
            with open(args.output + '.lock', 'w') as lock:
                pull.fcntl.flock(lock, pull.fcntl.LOCK_EX | pull.fcntl.LOCK_NB)
                with self.assertRaises(BlockingIOError):
                    pull.run(args)


if __name__ == '__main__':
    unittest.main()
